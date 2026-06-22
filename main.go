package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
)

//go:embed all:frontend/dist
var embedFS embed.FS

// S3 config

type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type FileEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
}

type DirectoryListing struct {
	Folders []FolderEntry `json:"folders"`
	Files   []FileEntry   `json:"files"`
}

type FileInfo struct {
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType"`
	LastModified string `json:"lastModified"`
	ETag         string `json:"eTag"`
}

type SearchResults struct {
	Files   []FileEntry   `json:"files"`
	Folders []FolderEntry `json:"folders"`
}

func main() {
	// Initialize and load configurations
	loadConfig()

	if err := initS3Client(); err != nil {
		log.Fatalf("Failed to initialize S3 client: %v", err)
	}

	// Initialize API LRU Cache
	initCache()

	// Setup API router
	http.HandleFunc("GET /api/health", handleHealth)
	http.HandleFunc("GET /api/list", handleList)
	http.HandleFunc("GET /api/info", handleInfo)
	http.HandleFunc("GET /api/search", handleSearch)
	http.HandleFunc("GET /api/object/{key...}", handleObjectRedirect)

	// Embedded static frontend configuration
	staticFS, err := fs.Sub(embedFS, "frontend/dist")
	var indexHTML []byte
	if err == nil {
		indexHTML, _ = embedFS.ReadFile("frontend/dist/index.html")
	}

	if len(indexHTML) > 0 && staticFS != nil {
		log.Println("Serving embedded Svelte frontend from frontend/dist/")
		http.Handle("/", serveEmbeddedSPA(staticFS, indexHTML))
	} else {
		log.Println("WARNING: Frontend frontend/dist/ files not found or not built. Serving HTTP API only.")
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"message": "S3 Index API is active. Frontend assets not embedded.",
				})
				return
			}
			http.NotFound(w, r)
		})
	}

	log.Printf("Server listening on port %s", globalConfig.Port)
	if err := http.ListenAndServe(":"+globalConfig.Port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
