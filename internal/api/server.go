package api

import (
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/lucanhost/s3index/internal/config"
	"github.com/lucanhost/s3index/internal/s3client"
	"github.com/lucanhost/s3index/internal/store"
)

type Server struct {
	config   *config.Config
	s3client *s3client.Client
	store    *store.Store
	staticFS fs.FS
}

func NewServer(cfg *config.Config, s3c *s3client.Client, st *store.Store, staticFS fs.FS) *Server {
	return &Server{config: cfg, s3client: s3c, store: st, staticFS: staticFS}
}

func (s *Server) SetupRouter() *http.ServeMux {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/list", s.handleList)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("POST /api/sync", s.handleSync)
	mux.HandleFunc("GET /api/object/{key...}", s.handleObjectRedirect)

	// Frontend or API-only fallback
	if s.staticFS != nil {
		log.Println("Serving embedded Svelte frontend from frontend/dist/")
		mux.HandleFunc("/", s.serveSPA)
	} else {
		log.Println("WARNING: Frontend files not embedded. Serving HTTP API only.")
		mux.HandleFunc("/", s.apiOnlyFallback)
	}

	return mux
}

// serveSPA serves embedded frontend with SPA fallback
func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	// Try exact file match
	path := strings.TrimPrefix(r.URL.Path, "/")
	if _, err := s.staticFS.Open(path); err == nil {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.FileServer(http.FS(s.staticFS)).ServeHTTP(w, r)
		return
	}

	// SPA fallback - serve index.html
	s.serveIndex(w, r)
}

// serveIndex serves the SPA index.html
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	f, err := s.staticFS.Open("index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, "index.html", st.ModTime(), f.(io.ReadSeeker))
}

// apiOnlyFallback returns a JSON message when frontend is not embedded
func (s *Server) apiOnlyFallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		writeJSON(w, http.StatusOK, map[string]string{"message": "S3 Index API is active. Frontend assets not embedded."})
		return
	}
	http.NotFound(w, r)
}