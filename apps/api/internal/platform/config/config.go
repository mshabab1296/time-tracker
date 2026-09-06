package config

import (
	"fmt"
	"os"
)

type Config struct {
	APIAddress  string
	DatabaseURL string
}

func Load() (Config, error) {
	cfg := Config{APIAddress: valueOrDefault("API_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL")}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
