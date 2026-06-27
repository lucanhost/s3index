package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	return &Server{
		config:   cfg,
		s3client: s3c,
		store:    st,
		staticFS: staticFS,
	}
}

func (s *Server) SetupRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(s.securityHeaders)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/list", s.handleList)
		r.Get("/info", s.handleInfo)
		r.Get("/search", s.handleSearch)
		r.Get("/object/*", s.handleObjectRedirect)
	})

	if s.staticFS != nil {
		log.Println("Serving embedded Svelte frontend from frontend/dist/")
		s.serveEmbeddedSPA(r)
	} else {
		log.Println("WARNING: Frontend frontend/dist/ files not found or not built. Serving HTTP API only.")
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
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

	return r
}

func (s *Server) serveEmbeddedSPA(r chi.Router) {
	fileServer := http.FileServer(http.FS(s.staticFS))

	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		f, err := s.staticFS.Open(path)
		if err != nil {
			// File not found, serve index.html for SPA fallback
			req.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			fileServer.ServeHTTP(w, req)
			return
		}
		f.Close()

		// Set appropriate Cache-Control headers
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}

		fileServer.ServeHTTP(w, req)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

