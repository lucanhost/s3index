package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"ts":     time.Now().UnixMilli(),
	})
}

func handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if globalConfig.S3Bucket == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "S3_BUCKET not configured"})
		return
	}

	prefix := r.URL.Query().Get("prefix")
	cacheKey := "list:" + prefix
	if val, found := apiCache.Get(cacheKey); found {
		if listing, ok := val.(DirectoryListing); ok {
			w.Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=60")
			w.Header().Set("X-Cache", "HIT")
			json.NewEncoder(w).Encode(listing)
			return
		}
	}

	listing, err := ListDirectory(r.Context(), prefix)
	if err != nil {
		log.Printf("ListDirectory error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	apiCache.Set(cacheKey, listing)
	w.Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=60")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(listing)
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if globalConfig.S3Bucket == "" {
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

	cacheKey := "info:" + key
	if val, found := apiCache.Get(cacheKey); found {
		if info, ok := val.(FileInfo); ok {
			w.Header().Set("Cache-Control", "public, max-age=300")
			w.Header().Set("X-Cache", "HIT")
			json.NewEncoder(w).Encode(info)
			return
		}
	}

	info, err := GetObjectInfo(r.Context(), key)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Not found"})
			return
		}
		log.Printf("GetObjectInfo error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	apiCache.Set(cacheKey, info)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(info)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if globalConfig.S3Bucket == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "S3_BUCKET not configured"})
		return
	}

	query := r.URL.Query().Get("q")
	cacheKey := "search:" + query
	if val, found := apiCache.Get(cacheKey); found {
		if results, ok := val.(SearchResults); ok {
			w.Header().Set("X-Cache", "HIT")
			json.NewEncoder(w).Encode(results)
			return
		}
	}

	results, err := SearchBucket(r.Context(), query)
	if err != nil {
		log.Printf("SearchBucket error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	apiCache.Set(cacheKey, results)
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(results)
}

func handleObjectRedirect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if globalConfig.S3Bucket == "" {
		http.Error(w, "S3_BUCKET not configured", http.StatusInternalServerError)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "Missing key", http.StatusBadRequest)
		return
	}

	presignedUrl, err := GetPresignedUrl(r.Context(), key, time.Hour)
	if err != nil {
		log.Printf("GetPresignedUrl error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, presignedUrl, http.StatusFound)
}

func serveEmbeddedSPA(staticFS fs.FS, indexHTML []byte) http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		f, err := staticFS.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to Svelte index.html
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexHTML)
	})
}
