// Command server runs the helpdesk API.
//
//	server           run migrations, optionally seed, then serve HTTP
//	server migrate   run migrations and exit
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

	"github.com/paska/helpdesk/internal/auth"
	"github.com/paska/helpdesk/internal/config"
	"github.com/paska/helpdesk/internal/repository/postgres"
	"github.com/paska/helpdesk/internal/seed"
	"github.com/paska/helpdesk/internal/service"
	"github.com/paska/helpdesk/internal/transport/httpapi"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, cfg.DatabaseURL, 60*time.Second)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := postgres.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	log.Info("migrations up to date")
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return nil
	}

	users, tickets := postgres.NewUserRepo(db), postgres.NewTicketRepo(db)
	hasher := auth.NewHasher(cfg.PasswordIterations)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL, nil)

	if cfg.SeedEnabled {
		if err := seed.Run(ctx, seed.Deps{Users: users, Tickets: tickets, Hasher: hasher, Password: cfg.SeedPassword, Log: log}); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
	}

	authSvc, err := service.NewAuthService(users, hasher, tokens)
	if err != nil {
		return err
	}
	handler := httpapi.New(httpapi.Deps{
		Auth:        authSvc,
		Tickets:     service.NewTicketService(tickets, users, cfg.UrgentSLA, nil),
		Users:       service.NewUserService(users),
		Logger:      log,
		CORSOrigins: cfg.CORSOrigins,
		Ping:        db.PingContext,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
