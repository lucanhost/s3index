package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/lucanhost/s3index/internal/config"
	"github.com/lucanhost/s3index/internal/s3client"
	"github.com/lucanhost/s3index/internal/store"
)

type Server struct {
	config    *config.Config
	s3client  *s3client.Client
	store     *store.Store
	staticFS  fs.FS
	indexHTML []byte
}

func NewServer(cfg *config.Config, s3c *s3client.Client, st *store.Store, staticFS fs.FS, indexHTML []byte) *Server {
	return &Server{
		config:    cfg,
		s3client:  s3c,
		store:     st,
		staticFS:  staticFS,
		indexHTML: indexHTML,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/list", s.handleList)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/object/{key...}", s.handleObjectRedirect)

	if len(s.indexHTML) > 0 && s.staticFS != nil {
		log.Println("Serving embedded Svelte frontend from frontend/dist/")
		mux.HandleFunc("/", s.serveEmbeddedSPA)
	} else {
		log.Println("WARNING: Frontend frontend/dist/ files not found or not built. Serving HTTP API only.")
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
}

func (s *Server) serveEmbeddedSPA(w http.ResponseWriter, r *http.Request) {
	if s.staticFS == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "S3 Index API is active. Frontend assets not embedded.",
		})
		return
	}

	fileServer := http.FileServer(http.FS(s.staticFS))

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	f, err := s.staticFS.Open(path)
	if err == nil {
		f.Close()
		
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}

		fileServer.ServeHTTP(w, r)
		return
	}

	// Fallback to Svelte index.html
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(s.indexHTML)
}
