package store

import (
	"context"
	"database/sql"
	"log"
	"mime"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lucanhost/s3index/internal/db"
	"github.com/lucanhost/s3index/internal/s3client"
	_ "modernc.org/sqlite"
)

type DBState struct {
	DB      *sql.DB
	Queries *db.Queries
}

type Store struct {
	state    atomic.Pointer[DBState]
	s3client *s3client.Client
}

const schemaSQL = `
CREATE TABLE objects (
    key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    parent TEXT NOT NULL,
    is_dir BOOLEAN NOT NULL,
    size INTEGER NOT NULL,
    last_modified TEXT NOT NULL,
    content_type TEXT NOT NULL,
    etag TEXT NOT NULL
);

CREATE INDEX objects_parent_idx ON objects(parent);
CREATE INDEX objects_name_idx ON objects(name);
CREATE INDEX objects_is_dir_idx ON objects(is_dir);
`

func NewStore(ctx context.Context, client *s3client.Client, syncInterval time.Duration) *Store {
	s := &Store{
		s3client: client,
	}

	log.Println("Performing initial S3 object prefetch...")
	newState, err := s.fetchAndCreateDB(ctx)
	if err != nil {
		log.Printf("Warning: Initial S3 prefetch failed: %v. Starting with empty store.", err)
		newState, _ = createEmptyDB()
	} else {
		log.Println("Initial S3 prefetch complete.")
	}

	s.state.Store(newState)

	// Start background sync worker if interval is positive
	if syncInterval > 0 {
		go s.startSyncWorker(ctx, syncInterval)
	}

	return s
}

func (s *Store) GetQueries() *db.Queries {
	state := s.state.Load()
	if state != nil {
		return state.Queries
	}
	return nil
}

func createEmptyDB() (*DBState, error) {
	conn, err := sql.Open("sqlite", "file::memory:?mode=memory")
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec(schemaSQL); err != nil {
		return nil, err
	}

	return &DBState{
		DB:      conn,
		Queries: db.New(conn),
	}, nil
}

func (s *Store) fetchAndCreateDB(ctx context.Context) (*DBState, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	newState, err := createEmptyDB()
	if err != nil {
		return nil, err
	}

	tx, err := newState.DB.BeginTx(fetchCtx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	qtx := newState.Queries.WithTx(tx)

	objectCh := s.s3client.ListObjects(fetchCtx, true)
	var folders = make(map[string]bool)

	for obj := range objectCh {
		if obj.Err != nil {
			return nil, obj.Err
		}

		// Skip explicit folder placeholders if they just end with slash
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}

		contentType := obj.ContentType
		if contentType == "" {
			contentType = guessContentType(obj.Key)
		}

		parts := strings.Split(obj.Key, "/")
		fileName := parts[len(parts)-1]

		var parentPrefix string
		if len(parts) > 1 {
			parentPrefix = strings.Join(parts[:len(parts)-1], "/") + "/"
		}

		err = qtx.InsertObject(fetchCtx, db.InsertObjectParams{
			Key:          obj.Key,
			Name:         fileName,
			Parent:       parentPrefix,
			IsDir:        false,
			Size:         obj.Size,
			LastModified: obj.LastModified.Format(time.RFC3339),
			ContentType:  contentType,
			Etag:         obj.ETag,
		})
		if err != nil {
			return nil, err
		}

		// Add parent directories recursively
		for i := len(parts) - 2; i >= 0; i-- {
			var currentPrefix string
			if i > 0 {
				currentPrefix = strings.Join(parts[:i], "/") + "/"
			}
			folderName := parts[i]
			folderPath := strings.Join(parts[:i+1], "/") + "/"

			if !folders[folderPath] {
				folders[folderPath] = true
				err = qtx.InsertObject(fetchCtx, db.InsertObjectParams{
					Key:          folderPath,
					Name:         folderName,
					Parent:       currentPrefix,
					IsDir:        true,
					Size:         0,
					LastModified: "",
					ContentType:  "",
					Etag:         "",
				})
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return newState, nil
}

func (s *Store) startSyncWorker(ctx context.Context, interval time.Duration) {
	log.Printf("Starting background S3 sync worker with interval: %s", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping S3 sync worker.")
			return
		case <-ticker.C:
			log.Println("Starting background S3 sync...")
			newState, err := s.fetchAndCreateDB(ctx)
			if err != nil {
				log.Printf("Background S3 sync error: %v", err)
				continue
			}

			// Atomic swap, then close the old DB
			oldState := s.state.Swap(newState)
			if oldState != nil && oldState.DB != nil {
				oldState.DB.Close()
			}
			log.Println("Background S3 sync complete.")
		}
	}
}

func guessContentType(key string) string {
	ext := filepath.Ext(key)
	t := mime.TypeByExtension(ext)
	if t == "" {
		return "application/octet-stream"
	}
	return t
}
