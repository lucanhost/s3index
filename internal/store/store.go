package store

import (
	"context"
	"database/sql"
	"log"
	"mime"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lucanhost/s3index/internal/db"
	"github.com/lucanhost/s3index/internal/s3client"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
)



type DBState struct {
	DB      *sql.DB
	Queries *db.Queries
}

type Store struct {
	state    atomic.Pointer[DBState]
	s3client *s3client.Client
	wg       sync.WaitGroup
	trigger  chan struct{} // Channel to trigger immediate sync
}

func NewStore(ctx context.Context, client *s3client.Client, syncInterval time.Duration) *Store {
	s := &Store{
		s3client: client,
		trigger:  make(chan struct{}, 1),
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

	if syncInterval > 0 {
		s.wg.Add(1)
		go s.startSyncWorker(ctx, syncInterval)
	}

	return s
}

func (s *Store) Shutdown() {
	s.wg.Wait()
	state := s.state.Load()
	if state != nil && state.DB != nil {
		state.DB.Close()
	}
}

func (s *Store) GetQueries() *db.Queries {
	state := s.state.Load()
	if state != nil {
		return state.Queries
	}
	return nil
}

// TriggerSync requests an immediate sync. Non-blocking - safe to call from handlers.
func (s *Store) TriggerSync() {
	select {
	case s.trigger <- struct{}{}:
	default:
		// Trigger already pending, skip
	}
}

func createEmptyDB() (*DBState, error) {
	conn, err := driver.Open("file::memory:?mode=memory", fts5.Register)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)

	// Performance optimizations for in-memory SQLite
	conn.Exec("PRAGMA page_size = 8192;")
	conn.Exec("PRAGMA cache_size = 10000;")      // Larger cache for in-memory
	conn.Exec("PRAGMA journal_mode = MEMORY;")     // No disk writes
	conn.Exec("PRAGMA synchronous = OFF;")          // Faster commits

	if _, err := conn.Exec(db.SchemaSQL); err != nil {
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

		if strings.HasSuffix(obj.Key, "/") {
			continue
		}

		contentType := obj.ContentType
		if contentType == "" {
			contentType = guessContentType(obj.Key)
		}

		lastSlash := strings.LastIndexByte(obj.Key, '/')
		var fileName, parentPrefix string
		if lastSlash == -1 {
			fileName = obj.Key
			parentPrefix = ""
		} else {
			fileName = obj.Key[lastSlash+1:]
			parentPrefix = obj.Key[:lastSlash+1]
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

		// Zero-allocation folder path traversal
		current := 0
		for {
			idx := strings.IndexByte(obj.Key[current:], '/')
			if idx == -1 {
				break
			}
			folderEnd := current + idx + 1
			folderPath := obj.Key[:folderEnd]

			if !folders[folderPath] {
				folders[folderPath] = true

				parentPath := ""
				folderName := folderPath[:len(folderPath)-1] // remove trailing slash
				lastParentSlash := strings.LastIndexByte(folderName, '/')
				if lastParentSlash != -1 {
					parentPath = folderName[:lastParentSlash+1]
					folderName = folderName[lastParentSlash+1:]
				}

				err = qtx.InsertObject(fetchCtx, db.InsertObjectParams{
					Key:          folderPath,
					Name:         folderName,
					Parent:       parentPath,
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
			current = folderEnd
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Actively compact SQLite internal heap and release unused pages back to the libc allocator
	newState.DB.Exec("PRAGMA shrink_memory")

	return newState, nil
}

func (s *Store) startSyncWorker(ctx context.Context, interval time.Duration) {
	defer s.wg.Done()

	log.Printf("Starting background S3 sync worker with interval: %s", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping S3 sync worker.")
			return
		case <-ticker.C:
			s.performSync(ctx)
		case <-s.trigger:
			log.Println("Immediate sync triggered...")
			s.performSync(ctx)
		}
	}
}

// performSync executes the sync operation and swaps the database state
func (s *Store) performSync(ctx context.Context) {
	newState, err := s.fetchAndCreateDB(ctx)
	if err != nil {
		log.Printf("S3 sync error: %v", err)
		return
	}

	oldState := s.state.Swap(newState)
	if oldState != nil && oldState.DB != nil {
		oldState.DB.Close()
	}
	log.Println("S3 sync complete.")
}

func guessContentType(key string) string {
	ext := filepath.Ext(key)
	t := mime.TypeByExtension(ext)
	if t == "" {
		return "application/octet-stream"
	}
	return t
}
