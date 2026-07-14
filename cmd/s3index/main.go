package main

import (
	"context"
	"io/fs"
	"log/slog"
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
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.LoadConfig()

	s3Client, err := s3client.NewClient(cfg)
	if err != nil {
		slog.Error("Failed to initialize S3 client", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	colStore := store.NewStore(ctx, s3Client, cfg.SyncInterval)

	staticFS, err := fs.Sub(s3index.EmbedFS, "frontend/dist")
	if err != nil {
		staticFS = nil
	}

	server := api.NewServer(cfg, s3Client, colStore, staticFS)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Server listening", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server")

	cancel()
	colStore.Shutdown()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exited")
}
