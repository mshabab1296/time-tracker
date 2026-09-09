package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFindsParentDotEnvWithoutOverridingProcessEnvironment(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, ".env"), []byte("DATABASE_URL=from-dotenv\nAPI_ADDR=:9090\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(directory, "apps", "api", "bin")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(child); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDirectory) })

	originalDatabaseURL, hadDatabaseURL := os.LookupEnv("DATABASE_URL")
	if err := os.Unsetenv("DATABASE_URL"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadDatabaseURL {
			_ = os.Setenv("DATABASE_URL", originalDatabaseURL)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	})
	config, err := Load()
	if err != nil || config.DatabaseURL != "from-dotenv" || config.APIAddress != ":9090" {
		t.Fatalf("loaded config=%+v err=%v", config, err)
	}

	t.Setenv("DATABASE_URL", "from-process")
	config, err = Load()
	if err != nil || config.DatabaseURL != "from-process" {
		t.Fatalf("process environment should win: config=%+v err=%v", config, err)
	}
}
