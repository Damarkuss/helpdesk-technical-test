package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/service"
)

// TicketRepo implements service.TicketRepository.
type TicketRepo struct{ db *sql.DB }

func NewTicketRepo(db *sql.DB) *TicketRepo { return &TicketRepo{db: db} }

const ticketSelect = `
SELECT t.id, t.title, t.description, t.category, t.priority, t.status, t.created_at, t.updated_at,
       r.id, r.name, r.role,
       a.id, a.name, a.role
FROM tickets t
JOIN users r      ON r.id = t.requester_id
LEFT JOIN users a ON a.id = t.assignee_id`

func scanTicket(row rowScanner) (*domain.Ticket, error) {
	var (
		t                domain.Ticket
		priority, status string
		requesterRole    string
		aID              sql.NullInt64
		aName, aRole     sql.NullString
	)
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Category, &priority, &status, &t.CreatedAt, &t.UpdatedAt,
		&t.Requester.ID, &t.Requester.Name, &requesterRole, &aID, &aName, &aRole)
	if err != nil {
		return nil, err
	}
	t.Priority, t.Status = domain.Priority(priority), domain.Status(status)
	t.Requester.Role = domain.Role(requesterRole)
	if aID.Valid {
		t.Assignee = &domain.UserRef{ID: aID.Int64, Name: aName.String, Role: domain.Role(aRole.String)}
	}
	return &t, nil
}

func (r *TicketRepo) Create(ctx context.Context, t *domain.Ticket) error {
	var assignee sql.NullInt64
	if t.Assignee != nil {
		assignee = sql.NullInt64{Int64: t.Assignee.ID, Valid: true}
	}
	return r.db.QueryRowContext(ctx, `
		INSERT INTO tickets (title, description, category, priority, status, requester_id, assignee_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		t.Title, t.Description, t.Category, string(t.Priority), string(t.Status), t.Requester.ID, assignee,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *TicketRepo) Get(ctx context.Context, id int64) (*domain.Ticket, error) {
	t, err := scanTicket(r.db.QueryRowContext(ctx, ticketSelect+` WHERE t.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NotFound("ticket")
	}
	return t, err
}

// orderExpr maps the whitelisted sort keys to SQL. User input is never
// interpolated into the query: only values from this map are.
var orderExpr = map[string]string{
	"created_at": "t.created_at",
	"updated_at": "t.updated_at",
	"title":      "lower(t.title)",
	"status":     "CASE t.status WHEN 'open' THEN 1 WHEN 'in_progress' THEN 2 WHEN 'resolved' THEN 3 ELSE 4 END",
	"priority":   "CASE t.priority WHEN 'low' THEN 1 WHEN 'medium' THEN 2 WHEN 'high' THEN 3 ELSE 4 END",
}

func (r *TicketRepo) List(ctx context.Context, f service.TicketFilter) ([]domain.Ticket, int, error) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.RequesterID != nil {
		add(`t.requester_id = ?`, *f.RequesterID)
	}
	if f.Status != nil {
		add(`t.status = ?`, string(*f.Status))
	}
	if f.Priority != nil {
		add(`t.priority = ?`, string(*f.Priority))
	}
	if f.AssigneeID != nil {
		add(`t.assignee_id = ?`, *f.AssigneeID)
	}
	if f.Unassigned {
		conds = append(conds, `t.assignee_id IS NULL`)
	}
	if f.Query != "" {
		add(`t.title ILIKE ?`, likePattern(f.Query))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM tickets t`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	expr, ok := orderExpr[f.SortBy]
	if !ok {
		expr = orderExpr["created_at"]
	}
	dir := "ASC"
	if f.SortDesc {
		dir = "DESC"
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	query := fmt.Sprintf(`%s%s ORDER BY %s %s, t.id DESC LIMIT $%d OFFSET $%d`,
		ticketSelect, where, expr, dir, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []domain.Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *t)
	}
	return out, total, rows.Err()
}

func (r *TicketRepo) Update(ctx context.Context, id int64, u service.TicketUpdate) (*domain.Ticket, error) {
	sets := []string{"updated_at = now()"}
	var args []any
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if u.Title != nil {
		set("title", *u.Title)
	}
	if u.Description != nil {
		set("description", *u.Description)
	}
	if u.Category != nil {
		set("category", *u.Category)
	}
	if u.Priority != nil {
		set("priority", string(*u.Priority))
	}
	if u.Status != nil {
		set("status", string(*u.Status))
	}
	if u.SetAssignee {
		var v sql.NullInt64
		if u.AssigneeID != nil {
			v = sql.NullInt64{Int64: *u.AssigneeID, Valid: true}
		}
		set("assignee_id", v)
	}

	args = append(args, id)
	where := fmt.Sprintf("id = $%d", len(args))
	if u.ExpectedStatus != "" {
		// Optimistic lock: the update only applies if nobody changed the status meanwhile.
		args = append(args, string(u.ExpectedStatus))
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	res, err := r.db.ExecContext(ctx, `UPDATE tickets SET `+strings.Join(sets, ", ")+` WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.Get(ctx, id); err != nil {
			return nil, err // really missing
		}
		return nil, domain.Conflict("stale_ticket", "the ticket was changed by someone else; reload and try again")
	}
	return r.Get(ctx, id)
}

func (r *TicketRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tickets WHERE id = $1`, id) // comments cascade
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.NotFound("ticket")
	}
	return nil
}

func (r *TicketRepo) AddComment(ctx context.Context, c *domain.Comment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after Commit

	// Touching the ticket also proves it exists and bumps updated_at.
	res, err := tx.ExecContext(ctx, `UPDATE tickets SET updated_at = now() WHERE id = $1`, c.TicketID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.NotFound("ticket")
	}
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO comments (ticket_id, author_id, body, is_internal) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		c.TicketID, c.Author.ID, c.Body, c.IsInternal,
	).Scan(&c.ID, &c.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *TicketRepo) ListComments(ctx context.Context, ticketID int64, includeInternal bool) ([]domain.Comment, error) {
	query := `
		SELECT c.id, c.ticket_id, c.body, c.is_internal, c.created_at, u.id, u.name, u.role
		FROM comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.ticket_id = $1`
	if !includeInternal {
		query += ` AND NOT c.is_internal`
	}
	rows, err := r.db.QueryContext(ctx, query+` ORDER BY c.created_at, c.id`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Comment{}
	for rows.Next() {
		var c domain.Comment
		var role string
		if err := rows.Scan(&c.ID, &c.TicketID, &c.Body, &c.IsInternal, &c.CreatedAt, &c.Author.ID, &c.Author.Name, &role); err != nil {
			return nil, err
		}
		c.Author.Role = domain.Role(role)
		out = append(out, c)
	}
	return out, rows.Err()
}
