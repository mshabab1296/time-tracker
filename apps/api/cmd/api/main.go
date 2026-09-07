package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/platform/config"
	"github.com/fortune-tech/time-tracker/apps/api/internal/platform/database"
	platformemail "github.com/fortune-tech/time-tracker/apps/api/internal/platform/email"
	"github.com/fortune-tech/time-tracker/apps/api/internal/platform/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	db, err := database.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	var sender platformemail.Sender
	if cfg.EmailProvider == "ses" {
		sender, err = platformemail.NewSES(context.Background(), cfg.AWSRegion, cfg.EmailFrom)
		if err != nil {
			logger.Error("SES sender initialization failed", "error", err)
			os.Exit(1)
		}
	} else {
		sender = platformemail.SMTP{Address: cfg.SMTPAddress, From: cfg.EmailFrom}
	}
	server := &http.Server{Addr: cfg.APIAddress, Handler: httpserver.New(db, logger, sender, cfg.WebBaseURL, cfg.CookieSecure), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("api started", "address", cfg.APIAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
