package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kelindar/column"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"ts":     time.Now().UnixMilli(),
	})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.config.S3Bucket == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "S3_BUCKET not configured"})
		return
	}

	prefix := r.URL.Query().Get("prefix")
	col := s.store.GetCollection()
	if col == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Store not initialized"})
		return
	}

	files := make([]FileEntry, 0)
	folders := make([]FolderEntry, 0)

	col.Query(func(txn *column.Txn) error {
		name := txn.String("name")
		key := txn.String("key")
		isDir := txn.Bool("is_dir")
		size := txn.Int64("size")
		lastMod := txn.String("last_modified")

		return txn.WithString("parent", func(v string) bool {
			return v == prefix
		}).Range(func(i uint32) {
			n, _ := name.Get()
			k, _ := key.Get()
			dir := isDir.Get()

			if dir {
				folders = append(folders, FolderEntry{Name: n, Path: k})
			} else {
				sz, _ := size.Get()
				lm, _ := lastMod.Get()
				files = append(files, FileEntry{
					Name:         n,
					Path:         k,
					Size:         sz,
					LastModified: lm,
				})
			}
		})
	})

	listing := &DirectoryListing{
		Folders: folders,
		Files:   files,
	}

	json.NewEncoder(w).Encode(listing)
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.config.S3Bucket == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "S3_BUCKET not configured"})
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing key parameter"})
		return
	}

	col := s.store.GetCollection()
	if col == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Store not initialized"})
		return
	}

	var info FileInfo
	var found bool

	col.Query(func(txn *column.Txn) error {
		size := txn.Int64("size")
		cType := txn.String("content_type")
		lastMod := txn.String("last_modified")
		etag := txn.String("etag")

		return txn.WithString("key", func(v string) bool {
			return v == key
		}).Range(func(i uint32) {
			found = true
			info.Size, _ = size.Get()
			info.ContentType, _ = cType.Get()
			info.LastModified, _ = lastMod.Get()
			info.ETag, _ = etag.Get()
		})
	})

	if !found {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Not found"})
		return
	}

	json.NewEncoder(w).Encode(info)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.config.S3Bucket == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "S3_BUCKET not configured"})
		return
	}

	query := r.URL.Query().Get("q")
	col := s.store.GetCollection()
	if col == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Store not initialized"})
		return
	}

	lowerQ := strings.ToLower(query)
	files := make([]FileEntry, 0)
	folders := make([]FolderEntry, 0)

	col.Query(func(txn *column.Txn) error {
		name := txn.String("name")
		key := txn.String("key")
		isDir := txn.Bool("is_dir")
		size := txn.Int64("size")
		lastMod := txn.String("last_modified")

		return txn.WithString("name", func(v string) bool {
			return strings.Contains(strings.ToLower(v), lowerQ)
		}).Range(func(i uint32) {
			n, _ := name.Get()
			k, _ := key.Get()
			dir := isDir.Get()

			if dir {
				if len(folders) < 100 {
					folders = append(folders, FolderEntry{Name: n, Path: k})
				}
			} else {
				if len(files) < 500 {
					sz, _ := size.Get()
					lm, _ := lastMod.Get()
					files = append(files, FileEntry{
						Name:         n,
						Path:         k,
						Size:         sz,
						LastModified: lm,
					})
				}
			}
		})
	})

	json.NewEncoder(w).Encode(SearchResults{Files: files, Folders: folders})
}

func (s *Server) handleObjectRedirect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.config.S3Bucket == "" {
		http.Error(w, "S3_BUCKET not configured", http.StatusInternalServerError)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "Missing key", http.StatusBadRequest)
		return
	}

	presignedUrl, err := s.s3client.GetPresignedUrl(r.Context(), key, time.Hour)
	if err != nil {
		log.Printf("GetPresignedUrl error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, presignedUrl, http.StatusFound)
}
