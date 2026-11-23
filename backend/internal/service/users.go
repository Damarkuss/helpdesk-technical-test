package service

import (
	"context"
	"strings"

	"github.com/paska/helpdesk/internal/domain"
)

// UserService implements user administration.
type UserService struct {
	users UserRepository
}

func NewUserService(users UserRepository) *UserService { return &UserService{users: users} }

type ListUsersInput struct {
	Query    string
	Role     string
	Page     int
	PageSize int
}

// List returns users for the admin screen.
func (s *UserService) List(ctx context.Context, actor *domain.User, in ListUsersInput) (*Page[domain.User], error) {
	if actor.Role != domain.RoleAdmin {
		return nil, domain.Forbidden("only admins can list users")
	}
	f := UserFilter{Query: strings.TrimSpace(in.Query)}
	f.Page, f.PageSize = pagination(in.Page, in.PageSize)
	if in.Role != "" {
		r := domain.Role(in.Role)
		if !r.Valid() {
			return nil, domain.FieldError("role", "must be one of: requester, agent, admin")
		}
		f.Roles = []domain.Role{r}
	}
	rows, total, err := s.users.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return newPage(rows, f.Page, f.PageSize, total), nil
}

// Assignable lists agents and admins; used to populate the "assign to" menu.
func (s *UserService) Assignable(ctx context.Context, actor *domain.User) ([]domain.UserRef, error) {
	if !actor.Role.IsStaff() {
		return nil, domain.Forbidden("only agents and admins can list assignees")
	}
	rows, _, err := s.users.List(ctx, UserFilter{Roles: []domain.Role{domain.RoleAgent, domain.RoleAdmin}, Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	refs := make([]domain.UserRef, len(rows))
	for i := range rows {
		refs[i] = rows[i].Ref()
	}
	return refs, nil
}

// ChangeRole changes another user's role. Admin only.
func (s *UserService) ChangeRole(ctx context.Context, actor *domain.User, id int64, role string) (*domain.User, error) {
	if actor.Role != domain.RoleAdmin {
		return nil, domain.Forbidden("only admins can change roles")
	}
	r := domain.Role(role)
	if !r.Valid() {
		return nil, domain.FieldError("role", "must be one of: requester, agent, admin")
	}
	if id == actor.ID {
		// Prevents an admin from locking everyone out by demoting the last admin.
		return nil, domain.Conflict("cannot_change_own_role", "you cannot change your own role")
	}
	if _, err := s.users.GetByID(ctx, id); err != nil {
		return nil, err
	}
	if err := s.users.UpdateRole(ctx, id, r); err != nil {
		return nil, err
	}
	return s.users.GetByID(ctx, id)
}
