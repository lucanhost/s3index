package api

import (
	"io/fs"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
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

func (s *Server) SetupRouter() *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(recover.New())
	app.Use(logger.New())

	apiGroup := app.Group("/api")
	apiGroup.Get("/health", s.handleHealth)
	apiGroup.Get("/list", s.handleList)
	apiGroup.Get("/info", s.handleInfo)
	apiGroup.Get("/search", s.handleSearch)
	apiGroup.Get("/object/*", s.handleObjectRedirect)

	if s.staticFS != nil {
		log.Println("Serving embedded Svelte frontend from frontend/dist/")
		s.serveEmbeddedSPA(app)
	} else {
		log.Println("WARNING: Frontend frontend/dist/ files not found or not built. Serving HTTP API only.")
		app.Use(func(c *fiber.Ctx) error {
			if c.Path() == "/" {
				return c.JSON(fiber.Map{
					"message": "S3 Index API is active. Frontend assets not embedded.",
				})
			}
			return c.SendStatus(fiber.StatusNotFound)
		})
	}

	return app
}

func (s *Server) serveEmbeddedSPA(app *fiber.App) {
	// Serve static files using Fiber's filesystem middleware
	app.Use("/", filesystem.New(filesystem.Config{
		Root:   http.FS(s.staticFS),
		Browse: false,
		MaxAge: 86400,
	}))

	// Fallback route for SPA - if no static file matched, serve index.html from embedded FS
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodGet {
			return c.SendStatus(fiber.StatusNotFound)
		}

		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Type("html")

		f, err := s.staticFS.Open("index.html")
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		buf := make([]byte, stat.Size())
		if _, err := f.Read(buf); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Send(buf)
	})
}
