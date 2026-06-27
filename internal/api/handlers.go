package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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
	db := s.store.GetDB()
	if db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Store not initialized"})
		return
	}

	files := make([]FileEntry, 0)
	folders := make([]FolderEntry, 0)

	rows, err := db.Query("SELECT key, name, is_dir, size, last_modified FROM objects WHERE parent = ?", prefix)
	if err != nil {
		log.Printf("Query error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Database error"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, name, lastMod string
		var isDir bool
		var size int64

		if err := rows.Scan(&key, &name, &isDir, &size, &lastMod); err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}

		if isDir {
			folders = append(folders, FolderEntry{Name: name, Path: key})
		} else {
			files = append(files, FileEntry{
				Name:         name,
				Path:         key,
				Size:         size,
				LastModified: lastMod,
			})
		}
	}

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

	db := s.store.GetDB()
	if db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Store not initialized"})
		return
	}

	var info FileInfo
	err := db.QueryRow("SELECT size, content_type, last_modified, etag FROM objects WHERE key = ?", key).Scan(&info.Size, &info.ContentType, &info.LastModified, &info.ETag)
	if err != nil {
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
	db := s.store.GetDB()
	if db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Store not initialized"})
		return
	}

	lowerQ := strings.ToLower(query)
	files := make([]FileEntry, 0)
	folders := make([]FolderEntry, 0)

	// In SQLite, LIKE is case-insensitive by default for ASCII.
	rows, err := db.Query("SELECT key, name, is_dir, size, last_modified FROM objects WHERE name LIKE ? LIMIT 600", "%"+lowerQ+"%")
	if err != nil {
		log.Printf("Query error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Database error"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, name, lastMod string
		var isDir bool
		var size int64

		if err := rows.Scan(&key, &name, &isDir, &size, &lastMod); err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}

		if isDir {
			if len(folders) < 100 {
				folders = append(folders, FolderEntry{Name: name, Path: key})
			}
		} else {
			if len(files) < 500 {
				files = append(files, FileEntry{
					Name:         name,
					Path:         key,
					Size:         size,
					LastModified: lastMod,
				})
			}
		}
	}

	json.NewEncoder(w).Encode(SearchResults{Files: files, Folders: folders})
}

func (s *Server) handleObjectRedirect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.config.S3Bucket == "" {
		http.Error(w, "S3_BUCKET not configured", http.StatusInternalServerError)
		return
	}

	key := chi.URLParam(r, "*")
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
