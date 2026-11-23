// Command devserver runs the same API as cmd/server but keeps all data in
// memory, so the UI can be tried without PostgreSQL or Docker:
//
//	go run ./cmd/devserver        # http://localhost:8080, data is lost on exit
//
// It seeds the demo accounts with the password "Password123!". Do not use it in production.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/paska/helpdesk/internal/auth"
	"github.com/paska/helpdesk/internal/repository/memory"
	"github.com/paska/helpdesk/internal/seed"
	"github.com/paska/helpdesk/internal/service"
	"github.com/paska/helpdesk/internal/transport/httpapi"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	store := memory.NewStore()
	hasher := auth.NewHasher(10_000) // light work factor: this is a throwaway demo server
	tokens := auth.NewTokenManager("dev-only-secret-dev-only-secret-0000", 24*time.Hour, nil)

	if err := seed.Run(context.Background(), seed.Deps{
		Users: store.Users(), Tickets: store.Tickets(), Hasher: hasher, Password: "Password123!", Log: log,
	}); err != nil {
		log.Error("seed failed", "error", err)
		os.Exit(1)
	}

	authSvc, err := service.NewAuthService(store.Users(), hasher, tokens)
	if err != nil {
		log.Error("setup failed", "error", err)
		os.Exit(1)
	}
	handler := httpapi.New(httpapi.Deps{
		Auth:    authSvc,
		Tickets: service.NewTicketService(store.Tickets(), store.Users(), 4*time.Hour, nil),
		Users:   service.NewUserService(store.Users()),
		Logger:  log,
	})

	log.Info("in-memory demo server listening", "addr", addr, "login", "admin@helpdesk.local / Password123!")
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
