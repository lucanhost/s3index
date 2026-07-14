package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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

	slog.Info("Performing initial S3 object prefetch")
	newState, err := s.fetchAndCreateDB(ctx)
	if err != nil {
		slog.Warn("Initial S3 prefetch failed, starting with empty store", "error", err)
		newState, _ = createEmptyDB()
	} else {
		slog.Info("Initial S3 prefetch complete")
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

	slog.Info("Starting background S3 sync worker", "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping S3 sync worker")
			return
		case <-ticker.C:
			s.performSync(ctx)
		case <-s.trigger:
			slog.Info("Immediate sync triggered")
			s.performSync(ctx)
		}
	}
}

// performSync executes the sync operation
func (s *Store) performSync(ctx context.Context) {
	state := s.state.Load()

	if state == nil || state.DB == nil {
		newState, err := s.fetchAndCreateDB(ctx)
		if err != nil {
			slog.Error("Full sync failed", "error", err)
			return
		}
		oldState := s.state.Swap(newState)
		if oldState != nil && oldState.DB != nil {
			oldState.DB.Close()
		}
		slog.Info("Full sync complete")
		return
	}

	if err := s.incrementalSync(ctx); err != nil {
		slog.Error("Incremental sync failed", "error", err)
		return
	}
	slog.Info("Incremental sync complete")
}

// incrementalSync updates the existing database in-place using a temp table
// to track all keys seen during the S3 scan, then removes stale entries.
func (s *Store) incrementalSync(ctx context.Context) error {
	syncCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	state := s.state.Load()

	tx, err := state.DB.BeginTx(syncCtx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := state.Queries.WithTx(tx)

	_, err = tx.ExecContext(syncCtx, "DROP TABLE IF EXISTS _sync_seen")
	if err != nil {
		return fmt.Errorf("drop stale temp: %w", err)
	}
	_, err = tx.ExecContext(syncCtx, "CREATE TEMP TABLE _sync_seen(key TEXT PRIMARY KEY)")
	if err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}

	etagStmt, err := tx.PrepareContext(syncCtx, "SELECT etag FROM objects WHERE key = ?")
	if err != nil {
		return fmt.Errorf("prepare etag stmt: %w", err)
	}
	defer etagStmt.Close()

	trackStmt, err := tx.PrepareContext(syncCtx, "INSERT OR IGNORE INTO _sync_seen(key) VALUES (?)")
	if err != nil {
		return fmt.Errorf("prepare track stmt: %w", err)
	}
	defer trackStmt.Close()

	objectCh := s.s3client.ListObjects(syncCtx, true)
	folders := make(map[string]struct{})

	for obj := range objectCh {
		if obj.Err != nil {
			return fmt.Errorf("s3 list: %w", obj.Err)
		}

		// Extract implicit folder paths from this key
		current := 0
		for {
			idx := strings.IndexByte(obj.Key[current:], '/')
			if idx == -1 {
				break
			}
			folderEnd := current + idx + 1
			folders[obj.Key[:folderEnd]] = struct{}{}
			current = folderEnd
		}

		if strings.HasSuffix(obj.Key, "/") {
			continue
		}

		contentType := obj.ContentType
		if contentType == "" {
			contentType = guessContentType(obj.Key)
		}

		// Check if object changed — skip upsert when etag matches
		var currentEtag string
		err := etagStmt.QueryRowContext(syncCtx, obj.Key).Scan(&currentEtag)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("check etag %s: %w", obj.Key, err)
		}

		if err == sql.ErrNoRows || currentEtag != obj.ETag {
			name, parent := splitKey(obj.Key)

			err = qtx.UpsertObject(syncCtx, db.UpsertObjectParams{
				Key:          obj.Key,
				Name:         name,
				Parent:       parent,
				IsDir:        false,
				Size:         obj.Size,
				LastModified: obj.LastModified.Format(time.RFC3339),
				ContentType:  contentType,
				Etag:         obj.ETag,
			})
			if err != nil {
				return fmt.Errorf("upsert %s: %w", obj.Key, err)
			}
		}

		_, err = trackStmt.ExecContext(syncCtx, obj.Key)
		if err != nil {
			return fmt.Errorf("track key %s: %w", obj.Key, err)
		}
	}

	// Prepare folder existence check
	folderStmt, err := tx.PrepareContext(syncCtx, "SELECT 1 FROM objects WHERE key = ? AND is_dir = 1")
	if err != nil {
		return fmt.Errorf("prepare folder stmt: %w", err)
	}
	defer folderStmt.Close()

	// Sync folder entries — skip existing folders to avoid FTS trigger churn
	for folderPath := range folders {
		var exists int
		err := folderStmt.QueryRowContext(syncCtx, folderPath).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("check folder %s: %w", folderPath, err)
		}

		if err == sql.ErrNoRows {
			trimmed := folderPath[:len(folderPath)-1]
			name, parent := splitKey(trimmed)

			err = qtx.UpsertObject(syncCtx, db.UpsertObjectParams{
				Key:          folderPath,
				Name:         name,
				Parent:       parent,
				IsDir:        true,
				Size:         0,
				LastModified: "",
				ContentType:  "",
				Etag:         "",
			})
			if err != nil {
				return fmt.Errorf("upsert folder %s: %w", folderPath, err)
			}
		}

		_, err = trackStmt.ExecContext(syncCtx, folderPath)
		if err != nil {
			return fmt.Errorf("track folder %s: %w", folderPath, err)
		}
	}

	// Delete objects no longer in S3
	result, err := tx.ExecContext(syncCtx, "DELETE FROM objects WHERE key NOT IN (SELECT key FROM _sync_seen)")
	if err != nil {
		return fmt.Errorf("delete stale: %w", err)
	}
	deleted, _ := result.RowsAffected()

	_, err = tx.ExecContext(syncCtx, "DROP TABLE IF EXISTS _sync_seen")
	if err != nil {
		return fmt.Errorf("drop temp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if deleted > 0 {
		slog.Info("Removed stale objects", "count", deleted)
	}

	return nil
}

// splitKey extracts filename and parent prefix from an S3 key
func splitKey(key string) (name, parent string) {
	lastSlash := strings.LastIndexByte(key, '/')
	if lastSlash == -1 {
		return key, ""
	}
	return key[lastSlash+1:], key[:lastSlash+1]
}

func guessContentType(key string) string {
	ext := filepath.Ext(key)
	t := mime.TypeByExtension(ext)
	if t == "" {
		return "application/octet-stream"
	}
	return t
}
