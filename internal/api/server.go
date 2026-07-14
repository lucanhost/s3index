package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lucanhost/s3index/internal/config"
	"github.com/lucanhost/s3index/internal/s3client"
	"github.com/lucanhost/s3index/internal/store"
)

type ctxKey string

const reqIDKey ctxKey = "request_id"

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(reqIDKey).(string)
	return id
}

type Server struct {
	config   *config.Config
	s3client *s3client.Client
	store    *store.Store
	staticFS fs.FS
}

func NewServer(cfg *config.Config, s3c *s3client.Client, st *store.Store, staticFS fs.FS) *Server {
	return &Server{config: cfg, s3client: s3c, store: st, staticFS: staticFS}
}

// requestIDMiddleware injects a request ID into context and response headers
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestID()
		ctx := context.WithValue(r.Context(), reqIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) SetupRouter() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/list", s.handleList)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/object/{key...}", s.handleObjectRedirect)

	if s.staticFS != nil {
		slog.Info("Serving embedded Svelte frontend from frontend/dist/")
		mux.HandleFunc("/", s.serveSPA)
	} else {
		slog.Warn("Frontend files not embedded — API only")
		mux.HandleFunc("/", s.apiOnlyFallback)
	}

	// Wrap entire mux with request ID middleware
	return mux
}

func (s *Server) Handler() http.Handler {
	return requestIDMiddleware(s.SetupRouter())
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if _, err := s.staticFS.Open(path); err == nil {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.FileServer(http.FS(s.staticFS)).ServeHTTP(w, r)
		return
	}

	s.serveIndex(w, r)
}

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

func (s *Server) apiOnlyFallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		writeJSON(w, http.StatusOK, map[string]string{"message": "S3 Index API is active. Frontend assets not embedded."})
		return
	}
	http.NotFound(w, r)
}


