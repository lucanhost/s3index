package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const requestTimeout = 5 * time.Second

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"ts":     time.Now().UnixMilli(),
	})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "S3_BUCKET not configured"})
		return
	}

	prefix := r.URL.Query().Get("prefix")
	q := s.store.GetQueries()
	if q == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Store not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	objects, err := q.ListObjectsByParent(ctx, prefix)
	if err != nil {
		log.Printf("ListObjectsByParent error: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}

	files := make([]FileEntry, 0, len(objects))
	folders := make([]FolderEntry, 0, len(objects))

	for _, obj := range objects {
		if obj.IsDir {
			folders = append(folders, FolderEntry{Name: obj.Name, Path: obj.Key})
		} else {
			files = append(files, FileEntry{
				Name:         obj.Name,
				Path:         obj.Key,
				Size:         obj.Size,
				LastModified: obj.LastModified,
			})
		}
	}

	listing := &DirectoryListing{
		Folders: folders,
		Files:   files,
	}

	jsonResponse(w, http.StatusOK, listing)
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "S3_BUCKET not configured"})
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing key parameter"})
		return
	}

	q := s.store.GetQueries()
	if q == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Store not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	obj, err := q.GetObject(ctx, key)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return
	}

	info := FileInfo{
		Size:         obj.Size,
		ContentType:  obj.ContentType,
		LastModified: obj.LastModified,
		ETag:         obj.Etag,
	}

	jsonResponse(w, http.StatusOK, info)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "S3_BUCKET not configured"})
		return
	}

	query := r.URL.Query().Get("q")
	q := s.store.GetQueries()
	if q == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Store not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	lowerQ := strings.ToLower(query)
	escapedQ := "\"" + strings.ReplaceAll(lowerQ, "\"", "\"\"") + "\""
	
	objects, err := q.SearchObjects(ctx, escapedQ)
	if err != nil {
		log.Printf("SearchObjects error: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}

	files := make([]FileEntry, 0, len(objects))
	folders := make([]FolderEntry, 0, len(objects))

	for _, obj := range objects {
		if obj.IsDir {
			if len(folders) < 100 {
				folders = append(folders, FolderEntry{Name: obj.Name, Path: obj.Key})
			}
		} else {
			if len(files) < 500 {
				files = append(files, FileEntry{
					Name:         obj.Name,
					Path:         obj.Key,
					Size:         obj.Size,
					LastModified: obj.LastModified,
				})
			}
		}
	}

	jsonResponse(w, http.StatusOK, SearchResults{Files: files, Folders: folders})
}

func (s *Server) handleObjectRedirect(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		http.Error(w, "S3_BUCKET not configured", http.StatusInternalServerError)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "Missing key", http.StatusBadRequest)
		return
	}

	decodedKey, err := url.PathUnescape(key)
	if err != nil {
		decodedKey = key
	}

	presignedUrl, err := s.s3client.GetPresignedUrl(r.Context(), decodedKey, time.Hour)
	if err != nil {
		log.Printf("GetPresignedUrl error: %v", err)
		http.Error(w, "Failed to generate presigned URL", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, presignedUrl, http.StatusFound)
}
