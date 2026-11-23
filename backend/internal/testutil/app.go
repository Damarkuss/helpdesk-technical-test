// Package testutil wires the real services to the in-memory repositories so
// tests exercise the actual business rules without a database.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/paska/helpdesk/internal/auth"
	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/repository/memory"
	"github.com/paska/helpdesk/internal/service"
)

const Password = "Passw0rd!test"

// App is a fully wired application backed by memory.
type App struct {
	T       *testing.T
	Store   *memory.Store
	Auth    *service.AuthService
	Tickets *service.TicketService
	Users   *service.UserService
	Clock   *time.Time
}

// NewApp builds an App. The clock starts at a fixed instant and only moves when a test advances it.
func NewApp(t *testing.T) *App {
	t.Helper()
	clock := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	now := func() time.Time { return clock }

	store := memory.NewStore()
	store.SetClock(now)

	hasher := auth.NewHasher(1000) // fast hashing, tests only
	tokens := auth.NewTokenManager("test-secret-test-secret-test-secret", time.Hour, now)

	authSvc, err := service.NewAuthService(store.Users(), hasher, tokens)
	if err != nil {
		t.Fatal(err)
	}
	return &App{
		T:       t,
		Store:   store,
		Auth:    authSvc,
		Tickets: service.NewTicketService(store.Tickets(), store.Users(), 4*time.Hour, now),
		Users:   service.NewUserService(store.Users()),
		Clock:   &clock,
	}
}

// NewUser inserts a user with the given role and returns it.
func (a *App) NewUser(name string, role domain.Role) *domain.User {
	a.T.Helper()
	hash, err := auth.NewHasher(1000).Hash(Password)
	if err != nil {
		a.T.Fatal(err)
	}
	u := &domain.User{Name: name, Email: name + "@example.com", PasswordHash: hash, Role: role}
	if err := a.Store.Users().Create(context.Background(), u); err != nil {
		a.T.Fatal(err)
	}
	return u
}

// NewTicket creates a ticket owned by owner through the service (so rules apply).
func (a *App) NewTicket(owner *domain.User, title string) *service.TicketView {
	a.T.Helper()
	tv, err := a.Tickets.Create(context.Background(), owner, service.CreateTicketInput{
		Title: title, Description: "Something is broken and needs attention.", Category: "software", Priority: "medium",
	})
	if err != nil {
		a.T.Fatal(err)
	}
	return tv
}
