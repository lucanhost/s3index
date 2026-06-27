package config

import (
	"fmt"
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
	_ = godotenv.Load()

	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("Failed to parse configuration: %v", err)
	}

	if cfg.S3Bucket == "" {
		log.Fatal("S3_BUCKET environment variable is required")
	}
	if cfg.S3AccessKeyID == "" {
		log.Fatal("S3_ACCESS_KEY_ID environment variable is required")
	}
	if cfg.S3SecretAccessKey == "" {
		log.Fatal("S3_SECRET_ACCESS_KEY environment variable is required")
	}

	fmt.Printf("Config loaded: bucket=%s region=%s endpoint=%s port=%s sync=%s\n",
		cfg.S3Bucket, cfg.S3Region, cfg.S3Endpoint, cfg.Port, cfg.SyncInterval)

	return &cfg
}
