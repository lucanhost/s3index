package store

import (
	"context"
	"log"
	"mime"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kelindar/column"
	"github.com/lucanhost/s3index/internal/s3client"
)

type Store struct {
	collection atomic.Pointer[column.Collection]
	s3client   *s3client.Client
}

func NewStore(ctx context.Context, client *s3client.Client, syncInterval time.Duration) *Store {
	s := &Store{
		s3client: client,
	}

	log.Println("Performing initial S3 object prefetch...")
	col, err := s.loadStoreFromS3(ctx)
	if err != nil {
		log.Printf("Warning: Initial S3 prefetch failed: %v. Starting with empty store.", err)
		col = newEmptyStore()
	} else {
		log.Println("Initial S3 prefetch complete.")
	}
	s.collection.Store(col)

	// Start background sync worker
	go s.startSyncWorker(ctx, syncInterval)

	return s
}

func (s *Store) GetCollection() *column.Collection {
	return s.collection.Load()
}

func newEmptyStore() *column.Collection {
	col := column.NewCollection()
	col.CreateColumn("key", column.ForString())
	col.CreateColumn("name", column.ForString())
	col.CreateColumn("parent", column.ForString())
	col.CreateColumn("is_dir", column.ForBool())
	col.CreateColumn("size", column.ForInt64())
	col.CreateColumn("last_modified", column.ForString()) // RFC3339
	col.CreateColumn("content_type", column.ForString())
	col.CreateColumn("etag", column.ForString())

	// Create index for fast directory filtering
	col.CreateIndex("is_dir_idx", "is_dir", func(r column.Reader) bool {
		return r.Bool()
	})
	return col
}

func (s *Store) loadStoreFromS3(ctx context.Context) (*column.Collection, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	col := newEmptyStore()

	objectCh := s.s3client.ListObjects(fetchCtx, true)

	var folders = make(map[string]bool)
	var fetchErr error

	col.Query(func(txn *column.Txn) error {
		for obj := range objectCh {
			if obj.Err != nil {
				fetchErr = obj.Err
				return fetchErr
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

			// Insert file
			txn.Insert(func(r column.Row) error {
				r.SetString("key", obj.Key)
				r.SetString("name", fileName)
				r.SetString("parent", parentPrefix)
				r.SetBool("is_dir", false)
				r.SetInt64("size", obj.Size)
				r.SetString("last_modified", obj.LastModified.Format(time.RFC3339))
				r.SetString("content_type", contentType)
				r.SetString("etag", obj.ETag)
				return nil
			})

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
					txn.Insert(func(r column.Row) error {
						r.SetString("key", folderPath)
						r.SetString("name", folderName)
						r.SetString("parent", currentPrefix)
						r.SetBool("is_dir", true)
						r.SetInt64("size", 0)
						r.SetString("last_modified", "")
						r.SetString("content_type", "")
						r.SetString("etag", "")
						return nil
					})
				}
			}
		}
		return nil
	})

	if fetchErr != nil {
		return nil, fetchErr
	}

	return col, nil
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
			col, err := s.loadStoreFromS3(ctx)
			if err != nil {
				log.Printf("Background S3 sync error: %v", err)
				continue
			}
			s.collection.Store(col)
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
