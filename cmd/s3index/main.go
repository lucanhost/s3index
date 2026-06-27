package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucanhost/s3index"
	"github.com/lucanhost/s3index/internal/api"
	"github.com/lucanhost/s3index/internal/config"
	"github.com/lucanhost/s3index/internal/s3client"
	"github.com/lucanhost/s3index/internal/store"
)

func main() {
	cfg := config.LoadConfig()

	s3Client, err := s3client.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize S3 client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	colStore := store.NewStore(ctx, s3Client, cfg.SyncInterval)

	staticFS, err := fs.Sub(s3index.EmbedFS, "frontend/dist")
	if err != nil {
		staticFS = nil
	}

	server := api.NewServer(cfg, s3Client, colStore, staticFS)
	mux := server.SetupRouter()

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("Server listening on port %s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	cancel()

	colStore.Shutdown()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
