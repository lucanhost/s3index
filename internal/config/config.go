package config

import (
	"log"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	S3Bucket          string        `env:"S3_BUCKET"`
	S3Region          string        `env:"S3_REGION"`
	S3Endpoint        string        `env:"S3_ENDPOINT"`
	S3AccessKeyID     string        `env:"S3_ACCESS_KEY_ID"`
	S3SecretAccessKey string        `env:"S3_SECRET_ACCESS_KEY"`
	S3ForcePathStyle  bool          `env:"S3_FORCE_PATH_STYLE" envDefault:"false"`
	Port              string        `env:"PORT" envDefault:"8080"`
	SyncInterval      time.Duration `env:"SYNC_INTERVAL" envDefault:"5m"`
}

func LoadConfig() *Config {
	// Load .env file (ignore error if not found, as configuration can come from system env vars)
	_ = godotenv.Load()

	var cfg Config

	// Parse environment variables directly into Config struct
	if err := env.Parse(&cfg); err != nil {
		log.Printf("Warning: Failed to parse configuration: %v", err)
	}

	if cfg.S3Bucket == "" {
		log.Println("WARNING: S3_BUCKET environment variable is not set")
	}

	return &cfg
}
