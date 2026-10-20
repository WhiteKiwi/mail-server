package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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
	if len(os.Args) == 5 && os.Args[1] == "prepare-runtime" {
		if err := config.PrepareRuntime(os.Args[2], os.Args[3], os.Args[4]); err != nil {
			logger.Error("prepare runtime input failed")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "prepare-migration" {
		if err := config.PrepareMigration(os.Args[2], os.Args[3]); err != nil {
			logger.Error("prepare migration input failed")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "check-runtime" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := checkRuntime(ctx); err != nil {
			logger.Error("mail runtime admission failed")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "migrate" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		databaseURL, err := config.MigrationDatabaseURL()
		if err != nil {
			logger.Error("explicit mail migration input rejected")
			os.Exit(1)
		}
		if err := migrate(ctx, databaseURL); err != nil {
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
	store, err := delivery.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("open mail database failed")
		os.Exit(1)
	}
	defer store.Close()
	mailer := delivery.NewSMTPMailer(delivery.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, FromAddress: cfg.FromAddress, SESConfigurationSet: cfg.SESConfigurationSet})
	app, err := mailserver.New(store, mailer, cfg.Clients, logger)
	if err != nil {
		logger.Error("build mail server failed")
		os.Exit(1)
	}
	httpServer := &http.Server{Addr: cfg.ListenAddress, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		logger.Error("mail listener unavailable")
		os.Exit(1)
	}
	logger.Info("mail server listening", "address", cfg.ListenAddress, "version", version)
	if err := serve(ctx, httpServer, listener, 30*time.Second); err != nil {
		logger.Error("mail server stopped")
		os.Exit(1)
	}
}

// Wait for Shutdown before main closes the pool and exits.
func serve(ctx context.Context, server *http.Server, listener net.Listener, grace time.Duration) error {
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	select {
	case err := <-finished:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			<-finished
			return err
		}
		err := <-finished
		if err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

func migrate(ctx context.Context, databaseURL string) error {
	if os.Getenv("MAIL_DATABASE_TRANSPORT") == "docker-desktop" {
		return fmt.Errorf("Docker Desktop transport is runtime only")
	}
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

func checkRuntime(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := delivery.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.VerifySchema(ctx)
}
