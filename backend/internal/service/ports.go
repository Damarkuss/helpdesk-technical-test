// Package service implements the helpdesk business rules. It depends only on
// the interfaces declared here, so it can be tested with in-memory fakes.
package service

import (
	"context"
	"time"

	"github.com/paska/helpdesk/internal/auth"
	"github.com/paska/helpdesk/internal/domain"
)

// Page is a paginated result.
type Page[T any] struct {
	Items      []T `json:"data"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func newPage[T any](items []T, page, size, total int) *Page[T] {
	if items == nil {
		items = []T{}
	}
	pages := (total + size - 1) / size
	return &Page[T]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: pages}
}

// UserFilter selects users for the admin list.
type UserFilter struct {
	Query    string
	Roles    []domain.Role // empty = all roles
	Page     int
	PageSize int
}

// UserRepository persists users.
type UserRepository interface {
	// Create stores u and fills ID and CreatedAt. Returns a Conflict "email_taken" error on duplicates.
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context, f UserFilter) ([]domain.User, int, error)
	UpdateRole(ctx context.Context, id int64, role domain.Role) error
}

// TicketFilter selects tickets. Zero values mean "no constraint".
type TicketFilter struct {
	RequesterID *int64
	Status      *domain.Status
	Priority    *domain.Priority
	AssigneeID  *int64
	Unassigned  bool
	Query       string // case-insensitive substring of the title
	SortBy      string // created_at | updated_at | title | status | priority
	SortDesc    bool
	Page        int
	PageSize    int
}

// TicketUpdate is a partial update. Nil pointers are left unchanged.
type TicketUpdate struct {
	Title       *string
	Description *string
	Category    *string
	Priority    *domain.Priority
	Status      *domain.Status
	SetAssignee bool
	AssigneeID  *int64 // nil with SetAssignee=true means "unassign"

	// ExpectedStatus makes the update conditional on the status read earlier,
	// so two concurrent transitions cannot both succeed (optimistic locking).
	ExpectedStatus domain.Status
}

// TicketRepository persists tickets and their comments.
type TicketRepository interface {
	Create(ctx context.Context, t *domain.Ticket) error
	Get(ctx context.Context, id int64) (*domain.Ticket, error)
	List(ctx context.Context, f TicketFilter) ([]domain.Ticket, int, error)
	// Update applies u. It returns Conflict "stale_ticket" if the status changed since ExpectedStatus.
	Update(ctx context.Context, id int64, u TicketUpdate) (*domain.Ticket, error)
	Delete(ctx context.Context, id int64) error

	AddComment(ctx context.Context, c *domain.Comment) error
	ListComments(ctx context.Context, ticketID int64, includeInternal bool) ([]domain.Comment, error)
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) bool
}

// TokenManager issues and parses access tokens.
type TokenManager interface {
	Issue(userID int64, role string) (string, time.Time, error)
	Parse(token string) (*auth.Claims, error)
}
