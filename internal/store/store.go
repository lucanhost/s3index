package store

import (
	"context"
	"database/sql"
	"log"
	"mime"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lucanhost/s3index/internal/s3client"
	_ "modernc.org/sqlite"
)

type Store struct {
	mu       sync.RWMutex
	db       *sql.DB
	s3client *s3client.Client
}

func NewStore(ctx context.Context, client *s3client.Client, syncInterval time.Duration) *Store {
	// Initialize an in-memory db with shared cache
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		log.Fatalf("Failed to open SQLite database: %v", err)
	}

	// Adjust connection pool for memory db
	db.SetMaxOpenConns(1)

	s := &Store{
		db:       db,
		s3client: client,
	}

	if err := s.initSchema(db, "objects"); err != nil {
		log.Fatalf("Failed to initialize SQLite schema: %v", err)
	}

	log.Println("Performing initial S3 object prefetch...")
	if err := s.loadStoreFromS3(ctx); err != nil {
		log.Printf("Warning: Initial S3 prefetch failed: %v. Starting with empty store.", err)
	} else {
		log.Println("Initial S3 prefetch complete.")
	}

	// Start background sync worker
	go s.startSyncWorker(ctx, syncInterval)

	return s
}

func (s *Store) initSchema(db *sql.DB, tableName string) error {
	query := `
	CREATE TABLE IF NOT EXISTS ` + tableName + ` (
		key TEXT PRIMARY KEY,
		name TEXT,
		parent TEXT,
		is_dir BOOLEAN,
		size INTEGER,
		last_modified TEXT,
		content_type TEXT,
		etag TEXT
	);
	CREATE INDEX IF NOT EXISTS ` + tableName + `_parent_idx ON ` + tableName + `(parent);
	CREATE INDEX IF NOT EXISTS ` + tableName + `_name_idx ON ` + tableName + `(name);
	CREATE INDEX IF NOT EXISTS ` + tableName + `_is_dir_idx ON ` + tableName + `(is_dir);
	`
	_, err := db.Exec(query)
	return err
}

func (s *Store) GetDB() *sql.DB {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db
}

func (s *Store) loadStoreFromS3(ctx context.Context) error {
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Load objects into a temporary table to avoid blocking reads
	tempTable := "objects_tmp"
	if err := s.initSchema(s.db, tempTable); err != nil {
		return err
	}

	// Clear temp table just in case
	if _, err := s.db.Exec("DELETE FROM " + tempTable); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(fetchCtx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO " + tempTable + " (key, name, parent, is_dir, size, last_modified, content_type, etag) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	objectCh := s.s3client.ListObjects(fetchCtx, true)
	var folders = make(map[string]bool)

	for obj := range objectCh {
		if obj.Err != nil {
			return obj.Err
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

		_, err = stmt.Exec(obj.Key, fileName, parentPrefix, false, obj.Size, obj.LastModified.Format(time.RFC3339), contentType, obj.ETag)
		if err != nil {
			return err
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
				_, err = stmt.Exec(folderPath, folderName, currentPrefix, true, 0, "", "", "")
				if err != nil {
					return err
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Atomic table swap using a transaction
	swapTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer swapTx.Rollback()

	if _, err := swapTx.Exec("DROP TABLE IF EXISTS objects"); err != nil {
		return err
	}
	if _, err := swapTx.Exec("ALTER TABLE " + tempTable + " RENAME TO objects"); err != nil {
		return err
	}

	// Recreate indexes on the new table
	if _, err := swapTx.Exec("CREATE INDEX IF NOT EXISTS objects_parent_idx ON objects(parent)"); err != nil {
		return err
	}
	if _, err := swapTx.Exec("CREATE INDEX IF NOT EXISTS objects_name_idx ON objects(name)"); err != nil {
		return err
	}
	if _, err := swapTx.Exec("CREATE INDEX IF NOT EXISTS objects_is_dir_idx ON objects(is_dir)"); err != nil {
		return err
	}

	return swapTx.Commit()
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
			if err := s.loadStoreFromS3(ctx); err != nil {
				log.Printf("Background S3 sync error: %v", err)
				continue
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
