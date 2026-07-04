package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lucanhost/s3index/internal/db"
)

const requestTimeout = 5 * time.Second

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// errorJSON writes an error JSON response
func errorJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleHealth returns service status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"ts":     time.Now().UnixMilli(),
	})
}

// handleList returns directory contents
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		errorJSON(w, http.StatusInternalServerError, "S3_BUCKET not configured")
		return
	}

	prefix := r.URL.Query().Get("prefix")
	q := s.store.GetQueries()
	if q == nil {
		errorJSON(w, http.StatusInternalServerError, "Store not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	rows, err := q.ListObjectsByParent(ctx, prefix)
	if err != nil {
		log.Printf("ListObjectsByParent error: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Database error")
		return
	}

	files, folders := toEntries(rows)
	writeJSON(w, http.StatusOK, DirectoryListing{Folders: folders, Files: files})
}

// handleInfo returns file metadata
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		errorJSON(w, http.StatusInternalServerError, "S3_BUCKET not configured")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		errorJSON(w, http.StatusBadRequest, "Missing key parameter")
		return
	}

	q := s.store.GetQueries()
	if q == nil {
		errorJSON(w, http.StatusInternalServerError, "Store not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	obj, err := q.GetObject(ctx, key)
	if err != nil {
		errorJSON(w, http.StatusNotFound, "Not found")
		return
	}

	writeJSON(w, http.StatusOK, FileInfo{
		Size:         obj.Size,
		ContentType:  obj.ContentType,
		LastModified: obj.LastModified,
		ETag:         obj.Etag,
	})
}

// handleSearch returns search results
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		errorJSON(w, http.StatusInternalServerError, "S3_BUCKET not configured")
		return
	}

	query := r.URL.Query().Get("q")
	q := s.store.GetQueries()
	if q == nil {
		errorJSON(w, http.StatusInternalServerError, "Store not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	lowerQ := strings.ToLower(query)
	escapedQ := "\"" + strings.ReplaceAll(lowerQ, "\"", "\"\"") + "\""

	rows, err := q.SearchObjects(ctx, escapedQ)
	if err != nil {
		log.Printf("SearchObjects error: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Database error")
		return
	}

	files, folders := toEntriesLimited(rows, 500, 100)
	writeJSON(w, http.StatusOK, SearchResults{Files: files, Folders: folders})
}

// handleSync triggers immediate sync
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	_ = s.config.S3Bucket // validate config
	s.store.TriggerSync()
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "Sync triggered"})
}

// handleObjectRedirect generates S3 presigned URL
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

// toEntries converts db rows to FileEntry/FolderEntry slices
func toEntries(rows []db.ListObjectsByParentRow) ([]FileEntry, []FolderEntry) {
	files := make([]FileEntry, 0, len(rows))
	folders := make([]FolderEntry, 0, len(rows))

	for _, obj := range rows {
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
	return files, folders
}

// toEntriesLimited limits the number of returned entries
func toEntriesLimited(rows []db.SearchObjectsRow, maxFiles, maxFolders int) ([]FileEntry, []FolderEntry) {
	files := make([]FileEntry, 0, len(rows))
	folders := make([]FolderEntry, 0, len(rows))

	for _, obj := range rows {
		if obj.IsDir {
			if len(folders) < maxFolders {
				folders = append(folders, FolderEntry{Name: obj.Name, Path: obj.Key})
			}
		} else {
			if len(files) < maxFiles {
				files = append(files, FileEntry{
					Name:         obj.Name,
					Path:         obj.Key,
					Size:         obj.Size,
					LastModified: obj.LastModified,
				})
			}
		}
	}
	return files, folders
}