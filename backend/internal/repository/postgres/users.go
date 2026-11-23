package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"

	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/service"
)

const userColumns = `id, name, email, password_hash, role, created_at`

// UserRepo implements service.UserRepository.
type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(row rowScanner) (*domain.User, error) {
	var u domain.User
	var role string
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &role, &u.CreatedAt); err != nil {
		return nil, err
	}
	u.Role = domain.Role(role)
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (name, email, password_hash, role) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		u.Name, u.Email, u.PasswordHash, string(u.Role),
	).Scan(&u.ID, &u.CreatedAt)
	if isUniqueViolation(err) {
		return domain.Conflict("email_taken", "an account with this email already exists")
	}
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NotFound("user")
	}
	return u, err
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, email))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NotFound("user")
	}
	return u, err
}

func (r *UserRepo) List(ctx context.Context, f service.UserFilter) ([]domain.User, int, error) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.Query != "" {
		add(`(name ILIKE ? OR email ILIKE ?)`, likePattern(f.Query))
	}
	if len(f.Roles) > 0 {
		roles := make([]string, len(f.Roles))
		for i, role := range f.Roles {
			roles[i] = string(role)
		}
		add(`role = ANY(?)`, pq.Array(roles))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM users`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT `+userColumns+` FROM users%s ORDER BY id LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []domain.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *u)
	}
	return out, total, rows.Err()
}

func (r *UserRepo) UpdateRole(ctx context.Context, id int64, role domain.Role) error {
	res, err := r.db.ExecContext(ctx, `UPDATE users SET role = $2 WHERE id = $1`, id, string(role))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.NotFound("user")
	}
	return nil
}

// likePattern turns user input into a "contains" pattern for ILIKE,
// escaping the wildcard characters so a search for "100%" is taken literally.
func likePattern(s string) string {
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return "%" + esc + "%"
}
