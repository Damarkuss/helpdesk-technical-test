package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/paska/helpdesk/internal/domain"
)

// AuthService handles registration, login and token authentication.
type AuthService struct {
	users  UserRepository
	hasher PasswordHasher
	tokens TokenManager

	// dummyHash is verified when the email is unknown so that a missing account
	// and a wrong password take about the same time (no user enumeration by timing).
	dummyHash string
}

func NewAuthService(users UserRepository, hasher PasswordHasher, tokens TokenManager) (*AuthService, error) {
	dummy, err := hasher.Hash("not-a-real-password")
	if err != nil {
		return nil, err
	}
	return &AuthService{users: users, hasher: hasher, tokens: tokens, dummyHash: dummy}, nil
}

type RegisterInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResult is returned by Register and Login.
type AuthResult struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      *domain.User `json:"user"`
}

// Register creates a requester account. Self-registration can never create staff.
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	name := strings.TrimSpace(in.Name)
	email := normalizeEmail(in.Email)

	fields := map[string]string{}
	checkLen(fields, "name", name, 2, 100)
	if !validEmail(email) {
		fields["email"] = "must be a valid email address"
	}
	checkLen(fields, "password", in.Password, 8, 128)
	if len(fields) > 0 {
		return nil, domain.Validation(fields)
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	u := &domain.User{Name: name, Email: email, PasswordHash: hash, Role: domain.RoleRequester}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return s.issue(u)
}

// Login verifies credentials. Every failure returns the same generic error.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (*AuthResult, error) {
	bad := domain.Unauthenticated("invalid email or password")

	email := normalizeEmail(in.Email)
	fields := map[string]string{}
	if email == "" {
		fields["email"] = "is required"
	}
	if in.Password == "" {
		fields["password"] = "is required"
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields)
	}

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		var de *domain.Error
		if errors.As(err, &de) && de.Kind == domain.KindNotFound {
			s.hasher.Verify(in.Password, s.dummyHash)
			return nil, bad
		}
		return nil, err
	}
	if !s.hasher.Verify(in.Password, u.PasswordHash) {
		return nil, bad
	}
	return s.issue(u)
}

// Authenticate resolves a bearer token to the current user. The user is loaded
// from the database on every request so role changes and deletions take effect
// immediately instead of waiting for the token to expire.
func (s *AuthService) Authenticate(ctx context.Context, token string) (*domain.User, error) {
	claims, err := s.tokens.Parse(token)
	if err != nil {
		return nil, domain.Unauthenticated("invalid or expired token")
	}
	u, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		var de *domain.Error
		if errors.As(err, &de) && de.Kind == domain.KindNotFound {
			return nil, domain.Unauthenticated("account no longer exists")
		}
		return nil, err
	}
	return u, nil
}

func (s *AuthService) issue(u *domain.User) (*AuthResult, error) {
	tok, exp, err := s.tokens.Issue(u.ID, string(u.Role))
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: tok, ExpiresAt: exp, User: u}, nil
}
