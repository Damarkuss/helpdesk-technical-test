package seed_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/paska/helpdesk/internal/auth"
	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/repository/memory"
	"github.com/paska/helpdesk/internal/seed"
	"github.com/paska/helpdesk/internal/service"
)

func TestSeedIsIdempotentAndCoversEveryRole(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore()
	deps := seed.Deps{
		Users: store.Users(), Tickets: store.Tickets(),
		Hasher: auth.NewHasher(1000), Password: "demo-password", Log: slog.New(slog.DiscardHandler),
	}

	for i := 0; i < 3; i++ { // running repeatedly (container restarts) must not duplicate anything
		if err := seed.Run(ctx, deps); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}

	users, total, _ := store.Users().List(ctx, service.UserFilter{Page: 1, PageSize: 100})
	if total != len(seed.Accounts) {
		t.Fatalf("users = %d, want %d", total, len(seed.Accounts))
	}
	roles := map[domain.Role]bool{}
	for _, u := range users {
		roles[u.Role] = true
		if !deps.Hasher.Verify("demo-password", u.PasswordHash) {
			t.Errorf("%s cannot log in with the seed password", u.Email)
		}
	}
	for _, r := range domain.Roles {
		if !roles[r] {
			t.Errorf("no seeded account with role %s", r)
		}
	}

	_, tickets, _ := store.Tickets().List(ctx, service.TicketFilter{Page: 1, PageSize: 100})
	if tickets != 5 {
		t.Errorf("tickets = %d, want 5 (no duplicates after re-seeding)", tickets)
	}
}
