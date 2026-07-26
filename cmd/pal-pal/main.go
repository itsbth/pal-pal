package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itsbth/pal-pal/internal/config"
	"github.com/itsbth/pal-pal/internal/monitor"
	"github.com/itsbth/pal-pal/internal/palworld"
	"github.com/itsbth/pal-pal/internal/store"
	palweb "github.com/itsbth/pal-pal/internal/web"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "pal-pal",
		Short:         "A small web companion for a Palworld dedicated server",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newServeCommand())
	return root
}

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the monitor and web interface",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = godotenv.Load()
			return run(cmd.Context())
		},
	}
}

func run(parent context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	api, err := palworld.NewClient(cfg.APIRoot, cfg.APIPassword)
	if err != nil {
		return err
	}

	database, err := store.Open(cfg.DataPath)
	if err != nil {
		return err
	}
	defer database.Close()

	serverMonitor := monitor.New(api, database, cfg.PollInterval, cfg.HistoryLimit, log)
	webServer, err := palweb.New(api, serverMonitor, database, palweb.Config{
		PublicRead:     cfg.PublicRead,
		PublicPassword: cfg.PublicPassword,
		AdminPassword:  cfg.AdminPassword,
		SecureCookies:  cfg.SecureCookies,
	}, log)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           webServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	monitorErrors := make(chan error, 1)
	go func() {
		monitorErrors <- serverMonitor.Run(ctx)
	}()

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("pal-pal listening", "address", cfg.ListenAddress, "public_read", cfg.PublicRead)
		serverErrors <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
	case err := <-monitorErrors:
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("run monitor: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	return nil
}
