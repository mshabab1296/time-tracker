package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	APIAddress    string
	DatabaseURL   string
	SMTPAddress   string
	EmailFrom     string
	WebBaseURL    string
	CookieSecure  bool
	EmailProvider string
	AWSRegion     string
}

func Load() (Config, error) {
	if err := loadLocalDotEnv(); err != nil {
		return Config{}, err
	}
	cfg := Config{APIAddress: valueOrDefault("API_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), SMTPAddress: valueOrDefault("SMTP_ADDRESS", "localhost:1025"), EmailFrom: valueOrDefault("EMAIL_FROM", "no-reply@timetracker.local"), WebBaseURL: valueOrDefault("WEB_BASE_URL", "http://localhost:5173"), CookieSecure: boolOrDefault("COOKIE_SECURE", false), EmailProvider: valueOrDefault("EMAIL_PROVIDER", "smtp"), AWSRegion: valueOrDefault("AWS_REGION", "ap-south-1")}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.EmailProvider != "smtp" && cfg.EmailProvider != "ses" {
		return Config{}, fmt.Errorf("EMAIL_PROVIDER must be smtp or ses")
	}
	return cfg, nil
}

// loadLocalDotEnv supports running the API from apps/api or apps/api/bin during
// local development. It is a no-op when no .env file exists, and godotenv.Load
// deliberately leaves real process environment variables unchanged for deployments.
func loadLocalDotEnv() error {
	directory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	for {
		path := filepath.Join(directory, ".env")
		info, statErr := os.Stat(path)
		if statErr == nil && !info.IsDir() {
			if err := godotenv.Load(path); err != nil {
				return fmt.Errorf("load local .env: %w", err)
			}
			return nil
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return fmt.Errorf("inspect local .env: %w", statErr)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return nil
		}
		directory = parent
	}
}

func boolOrDefault(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
