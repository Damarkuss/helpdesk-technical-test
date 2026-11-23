package postgres_test

// Integration tests against a real PostgreSQL. They are skipped unless
// TEST_DATABASE_URL is set, e.g.:
//
//	docker run --rm -d -p 5433:5432 -e POSTGRES_PASSWORD=pw -e POSTGRES_DB=helpdesk_test postgres:16-alpine
//	TEST_DATABASE_URL='postgres://postgres:pw@localhost:5433/helpdesk_test?sslmode=disable' go test ./internal/repository/postgres/...
//
// WARNING: the tests drop and recreate the public schema of that database.

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/repository/postgres"
	"github.com/paska/helpdesk/internal/service"
)

var ctx = context.Background()

func openClean(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration tests")
	}
	db, err := postgres.Open(ctx, url, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateCanRunTwice(t *testing.T) {
	db := openClean(t)
	if err := postgres.Migrate(ctx, db); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != 1 {
		t.Errorf("schema_migrations rows = %d, err = %v; want 1", n, err)
	}
}

func TestUsersUniqueEmailIgnoresCase(t *testing.T) {
	db := openClean(t)
	users := postgres.NewUserRepo(db)

	if err := users.Create(ctx, &domain.User{Name: "A", Email: "a@example.com", PasswordHash: "h", Role: domain.RoleRequester}); err != nil {
		t.Fatal(err)
	}
	err := users.Create(ctx, &domain.User{Name: "B", Email: "A@Example.com", PasswordHash: "h", Role: domain.RoleRequester})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != "email_taken" {
		t.Fatalf("err = %v, want email_taken", err)
	}
	if _, err := users.GetByEmail(ctx, "A@EXAMPLE.COM"); err != nil {
		t.Errorf("case-insensitive lookup: %v", err)
	}
}

func TestTicketListFilterSortPaginateAndSearch(t *testing.T) {
	db := openClean(t)
	users, tickets := postgres.NewUserRepo(db), postgres.NewTicketRepo(db)

	mk := func(name string, role domain.Role) *domain.User {
		u := &domain.User{Name: name, Email: name + "@example.com", PasswordHash: "h", Role: role}
		if err := users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	alice, bob, agent := mk("alice", domain.RoleRequester), mk("bob", domain.RoleRequester), mk("agent", domain.RoleAgent)

	add := func(owner *domain.User, title string, p domain.Priority) *domain.Ticket {
		tk := &domain.Ticket{Title: title, Description: "d", Category: "general", Priority: p, Status: domain.StatusOpen, Requester: owner.Ref()}
		if err := tickets.Create(ctx, tk); err != nil {
			t.Fatal(err)
		}
		return tk
	}
	add(alice, "Printer 100% broken", domain.PriorityLow)
	add(alice, "VPN down", domain.PriorityUrgent)
	add(bob, "Printer_jam", domain.PriorityHigh)
	add(bob, "Laptop", domain.PriorityMedium)

	list := func(f service.TicketFilter) ([]domain.Ticket, int) {
		if f.Page == 0 {
			f.Page, f.PageSize = 1, 50
		}
		rows, total, err := tickets.List(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return rows, total
	}

	aliceID := alice.ID
	if _, total := list(service.TicketFilter{RequesterID: &aliceID}); total != 2 {
		t.Errorf("requester scope: total = %d, want 2", total)
	}
	// LIKE wildcards in the search term are taken literally.
	if rows, _ := list(service.TicketFilter{Query: "100%"}); len(rows) != 1 || rows[0].Title != "Printer 100% broken" {
		t.Errorf("literal %%: %+v", rows)
	}
	if rows, _ := list(service.TicketFilter{Query: "r_j"}); len(rows) != 1 {
		t.Errorf("literal _: got %d rows, want 1", len(rows))
	}
	if rows, _ := list(service.TicketFilter{Query: "PRINTER"}); len(rows) != 2 {
		t.Errorf("case-insensitive search: %d rows, want 2", len(rows))
	}

	// Priority sorts by urgency, not alphabetically.
	rows, _ := list(service.TicketFilter{SortBy: "priority", SortDesc: true})
	if rows[0].Title != "VPN down" || rows[3].Title != "Printer 100% broken" {
		t.Errorf("priority desc: first=%s last=%s", rows[0].Title, rows[3].Title)
	}

	// Pagination.
	page2, total := list(service.TicketFilter{SortBy: "title", Page: 2, PageSize: 3})
	if total != 4 || len(page2) != 1 {
		t.Errorf("page 2: total=%d rows=%d", total, len(page2))
	}

	// Assignment filters.
	st := domain.StatusInProgress
	if _, err := tickets.Update(ctx, rows[0].ID, service.TicketUpdate{Status: &st, SetAssignee: true, AssigneeID: &agent.ID, ExpectedStatus: domain.StatusOpen}); err != nil {
		t.Fatal(err)
	}
	if _, total := list(service.TicketFilter{AssigneeID: &agent.ID}); total != 1 {
		t.Errorf("assignee filter: %d", total)
	}
	if _, total := list(service.TicketFilter{Unassigned: true}); total != 3 {
		t.Errorf("unassigned filter: %d", total)
	}
}

func TestTicketUpdateDetectsConcurrentStatusChange(t *testing.T) {
	db := openClean(t)
	users, tickets := postgres.NewUserRepo(db), postgres.NewTicketRepo(db)
	u := &domain.User{Name: "a", Email: "a@example.com", PasswordHash: "h", Role: domain.RoleAgent}
	if err := users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	tk := &domain.Ticket{Title: "T", Description: "d", Category: "general", Priority: domain.PriorityLow, Status: domain.StatusOpen, Requester: u.Ref()}
	if err := tickets.Create(ctx, tk); err != nil {
		t.Fatal(err)
	}

	inProgress, closed := domain.StatusInProgress, domain.StatusClosed
	if _, err := tickets.Update(ctx, tk.ID, service.TicketUpdate{Status: &inProgress, ExpectedStatus: domain.StatusOpen}); err != nil {
		t.Fatal(err)
	}
	// A second writer still believes the ticket is "open".
	_, err := tickets.Update(ctx, tk.ID, service.TicketUpdate{Status: &closed, ExpectedStatus: domain.StatusOpen})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != "stale_ticket" {
		t.Errorf("err = %v, want stale_ticket", err)
	}
}

func TestCommentsInternalFilteringAndCascadeDelete(t *testing.T) {
	db := openClean(t)
	users, tickets := postgres.NewUserRepo(db), postgres.NewTicketRepo(db)
	u := &domain.User{Name: "a", Email: "a@example.com", PasswordHash: "h", Role: domain.RoleAgent}
	if err := users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	tk := &domain.Ticket{Title: "T", Description: "d", Category: "general", Priority: domain.PriorityLow, Status: domain.StatusOpen, Requester: u.Ref()}
	if err := tickets.Create(ctx, tk); err != nil {
		t.Fatal(err)
	}
	for _, internal := range []bool{false, true} {
		if err := tickets.AddComment(ctx, &domain.Comment{TicketID: tk.ID, Author: u.Ref(), Body: "x", IsInternal: internal}); err != nil {
			t.Fatal(err)
		}
	}
	if cs, _ := tickets.ListComments(ctx, tk.ID, false); len(cs) != 1 {
		t.Errorf("public view: %d comments, want 1", len(cs))
	}
	if cs, _ := tickets.ListComments(ctx, tk.ID, true); len(cs) != 2 {
		t.Errorf("staff view: %d comments, want 2", len(cs))
	}
	if err := tickets.AddComment(ctx, &domain.Comment{TicketID: 9999, Author: u.Ref(), Body: "x"}); err == nil {
		t.Error("comment on a missing ticket must fail")
	}

	if err := tickets.Delete(ctx, tk.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM comments`).Scan(&n)
	if n != 0 {
		t.Errorf("%d comments left after deleting the ticket", n)
	}
}
