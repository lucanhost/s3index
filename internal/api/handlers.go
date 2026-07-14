package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lucanhost/s3index/internal/db"
)

const requestTimeout = 5 * time.Second

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func errorJSON(w http.ResponseWriter, r *http.Request, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{
		"error":      msg,
		"request_id": GetRequestID(r.Context()),
	})
}

func handleError(ctx context.Context, msg string, err error) {
	slog.Error(msg, "error", err, "request_id", GetRequestID(ctx))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	q := s.store.GetQueries()
	storeOK := q != nil

	status := "ok"
	code := http.StatusOK
	if !storeOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]interface{}{
		"status":     status,
		"ts":         time.Now().UnixMilli(),
		"store":      storeOK,
		"request_id": GetRequestID(r.Context()),
	})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		errorJSON(w, r, http.StatusInternalServerError, "S3_BUCKET not configured")
		return
	}

	prefix := r.URL.Query().Get("prefix")
	q := s.store.GetQueries()
	if q == nil {
		errorJSON(w, r, http.StatusInternalServerError, "Store not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	offset := 0
	limit := 500
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := parseInt(o); err == nil && v >= 0 {
			offset = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := parseInt(l); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}

	// Fetch folders separately — they're typically few
	folderRows, err := q.ListFoldersByParent(ctx, prefix)
	if err != nil {
		handleError(ctx, "ListFoldersByParent failed", err)
		errorJSON(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	folders := make([]FolderEntry, 0, len(folderRows))
	for _, f := range folderRows {
		folders = append(folders, FolderEntry{Name: f.Name, Path: f.Key})
	}

	// Fetch files with pagination (request limit+1 to check has_more)
	paginatedRows, err := q.ListObjectsByParentPaginated(ctx, prefix, limit, offset)
	if err != nil {
		handleError(ctx, "ListObjectsByParentPaginated failed", err)
		errorJSON(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	hasMore := len(paginatedRows) > limit
	if hasMore {
		paginatedRows = paginatedRows[:limit]
	}

	files := make([]FileEntry, 0, len(paginatedRows))
	for _, obj := range paginatedRows {
		files = append(files, FileEntry{
			Name:         obj.Name,
			Path:         obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
		})
	}

	writeJSON(w, http.StatusOK, DirectoryListing{
		Folders: folders,
		Files:   files,
		HasMore: hasMore,
		Offset:  offset,
	})
}

// parseInt is a simple helper to avoid importing strconv for one function
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		errorJSON(w, r, http.StatusInternalServerError, "S3_BUCKET not configured")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		errorJSON(w, r, http.StatusBadRequest, "Missing key parameter")
		return
	}

	q := s.store.GetQueries()
	if q == nil {
		errorJSON(w, r, http.StatusInternalServerError, "Store not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	obj, err := q.GetObject(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorJSON(w, r, http.StatusNotFound, "Not found")
		} else {
			handleError(ctx, "GetObject failed", err)
			errorJSON(w, r, http.StatusInternalServerError, "Database error")
		}
		return
	}

	writeJSON(w, http.StatusOK, FileInfo{
		Size:         obj.Size,
		ContentType:  obj.ContentType,
		LastModified: obj.LastModified,
		ETag:         obj.Etag,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		errorJSON(w, r, http.StatusInternalServerError, "S3_BUCKET not configured")
		return
	}

	query := r.URL.Query().Get("q")
	q := s.store.GetQueries()
	if q == nil {
		errorJSON(w, r, http.StatusInternalServerError, "Store not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	lowerQ := strings.ToLower(query)
	escapedQ := "\"" + strings.ReplaceAll(lowerQ, "\"", "\"\"") + "\""

	rows, err := q.SearchObjects(ctx, escapedQ)
	if err != nil {
		handleError(ctx, "SearchObjects failed", err)
		errorJSON(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	files, folders := toEntriesLimited(rows, 500, 100)
	writeJSON(w, http.StatusOK, SearchResults{Files: files, Folders: folders})
}

func (s *Server) handleObjectRedirect(w http.ResponseWriter, r *http.Request) {
	if s.config.S3Bucket == "" {
		errorJSON(w, r, http.StatusInternalServerError, "S3_BUCKET not configured")
		return
	}

	key := r.PathValue("key")
	if key == "" {
		errorJSON(w, r, http.StatusBadRequest, "Missing key")
		return
	}

	decodedKey, err := url.PathUnescape(key)
	if err != nil {
		decodedKey = key
	}

	presignedUrl, err := s.s3client.GetPresignedUrl(r.Context(), decodedKey, time.Hour)
	if err != nil {
		handleError(r.Context(), "GetPresignedUrl failed", err)
		errorJSON(w, r, http.StatusInternalServerError, "Failed to generate presigned URL")
		return
	}

	http.Redirect(w, r, presignedUrl, http.StatusFound)
}

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
