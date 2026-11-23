// Package memory is an in-memory implementation of the repository interfaces.
// It exists so the service and HTTP layers can be tested quickly without a
// database; the production implementation lives in the postgres package.
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/service"
)

type ticketRow struct {
	domain.Ticket
	requesterID int64
	assigneeID  *int64
}

// Store holds all data. Use Users() and Tickets() to get the repository views.
type Store struct {
	mu       sync.RWMutex
	users    map[int64]*domain.User
	tickets  map[int64]*ticketRow
	comments []*domain.Comment

	nextUser, nextTicket, nextComment int64
	now                               func() time.Time
}

func NewStore() *Store {
	return &Store{
		users:       map[int64]*domain.User{},
		tickets:     map[int64]*ticketRow{},
		nextUser:    1,
		nextTicket:  1,
		nextComment: 1,
		now:         time.Now,
	}
}

// SetClock overrides the clock (tests).
func (s *Store) SetClock(now func() time.Time) { s.now = now }

func (s *Store) Users() service.UserRepository     { return &userRepo{s} }
func (s *Store) Tickets() service.TicketRepository { return &ticketRepo{s} }

// ---- users ------------------------------------------------------------------

type userRepo struct{ s *Store }

func (r *userRepo) Create(_ context.Context, u *domain.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, existing := range r.s.users {
		if strings.EqualFold(existing.Email, u.Email) {
			return domain.Conflict("email_taken", "an account with this email already exists")
		}
	}
	u.ID = r.s.nextUser
	r.s.nextUser++
	u.CreatedAt = r.s.now()
	cp := *u
	r.s.users[u.ID] = &cp
	return nil
}

func (r *userRepo) GetByID(_ context.Context, id int64) (*domain.User, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	u, ok := r.s.users[id]
	if !ok {
		return nil, domain.NotFound("user")
	}
	cp := *u
	return &cp, nil
}

func (r *userRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	for _, u := range r.s.users {
		if strings.EqualFold(u.Email, email) {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.NotFound("user")
}

func (r *userRepo) List(_ context.Context, f service.UserFilter) ([]domain.User, int, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	q := strings.ToLower(f.Query)
	var out []domain.User
	for _, u := range r.s.users {
		if q != "" && !strings.Contains(strings.ToLower(u.Name), q) && !strings.Contains(strings.ToLower(u.Email), q) {
			continue
		}
		if len(f.Roles) > 0 && !containsRole(f.Roles, u.Role) {
			continue
		}
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return paginate(out, f.Page, f.PageSize)
}

func (r *userRepo) UpdateRole(_ context.Context, id int64, role domain.Role) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return domain.NotFound("user")
	}
	u.Role = role
	return nil
}

func containsRole(roles []domain.Role, r domain.Role) bool {
	for _, v := range roles {
		if v == r {
			return true
		}
	}
	return false
}

// paginate returns the requested page and the total before slicing.
func paginate[T any](all []T, page, size int) ([]T, int, error) {
	total := len(all)
	start := (page - 1) * size
	if start >= total {
		return []T{}, total, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}
