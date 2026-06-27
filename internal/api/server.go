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
	return &Server{
		config:   cfg,
		s3client: s3c,
		store:    st,
		staticFS: staticFS,
	}
}

func (s *Server) SetupRouter() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/list", s.handleList)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/object/{key...}", s.handleObjectRedirect)

	if s.staticFS != nil {
		log.Println("Serving embedded Svelte frontend from frontend/dist/")
		s.serveEmbeddedSPA(mux)
	} else {
		log.Println("WARNING: Frontend frontend/dist/ files not found or not built. Serving HTTP API only.")
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"message": "S3 Index API is active. Frontend assets not embedded."}`))
				return
			}
			http.NotFound(w, r)
		})
	}

	return mux
}

func (s *Server) serveEmbeddedSPA(mux *http.ServeMux) {
	fileServer := http.FileServer(http.FS(s.staticFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Attempt to open the exact file
		f, err := s.staticFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil {
			f.Close()
			// File exists, serve it with FileServer
			w.Header().Set("Cache-Control", "public, max-age=86400")
			fileServer.ServeHTTP(w, r)
			return
		}

		// File does not exist, serve index.html (SPA fallback)
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		idx, err := s.staticFS.Open("index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer idx.Close()

		stat, err := idx.Stat()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		
		http.ServeContent(w, r, "index.html", stat.ModTime(), idx.(io.ReadSeeker))
	})
}
