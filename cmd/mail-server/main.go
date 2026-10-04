package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/whitekiwi/mail-server/internal/config"
	"github.com/whitekiwi/mail-server/internal/delivery"
	"github.com/whitekiwi/mail-server/internal/migrations"
	mailserver "github.com/whitekiwi/mail-server/internal/server"
)

var version = "development"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if len(os.Args) == 2 && os.Args[1] == "migrate" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := migrate(ctx, os.Getenv("MAIL_MIGRATION_DATABASE_URL")); err != nil {
			logger.Error("explicit mail database migration failed")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 1 {
		logger.Error("unsupported arguments")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid runtime configuration", "error", err)
		os.Exit(1)
	}
	store, err := delivery.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("open mail database failed")
		os.Exit(1)
	}
	defer store.Close()
	if err := store.VerifySchema(context.Background()); err != nil {
		logger.Error("mail database schema is not ready; run explicit migrate before startup")
		os.Exit(1)
	}
	mailer := delivery.NewSMTPMailer(delivery.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, FromAddress: cfg.FromAddress, SESConfigurationSet: cfg.SESConfigurationSet})
	app, err := mailserver.New(store, mailer, cfg.Clients, logger)
	if err != nil {
		logger.Error("build mail server failed")
		os.Exit(1)
	}
	httpServer := &http.Server{Addr: cfg.ListenAddress, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdown); err != nil {
			logger.Error("mail server graceful shutdown failed")
		}
	}()
	logger.Info("mail server listening", "address", cfg.ListenAddress, "version", version)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("mail server stopped")
		os.Exit(1)
	}
}

func migrate(ctx context.Context, databaseURL string) error {
	if databaseURL == "" {
		return fmt.Errorf("MAIL_MIGRATION_DATABASE_URL is required")
	}
	store, err := delivery.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.Migrate(ctx, migrations.Initial)
}
