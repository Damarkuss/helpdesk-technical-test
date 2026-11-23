package memory

import (
	"context"
	"sort"
	"strings"

	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/service"
)

type ticketRepo struct{ s *Store }

// hydrate resolves user references. Callers must hold at least a read lock.
func (r *ticketRepo) hydrate(row *ticketRow) domain.Ticket {
	t := row.Ticket
	if u, ok := r.s.users[row.requesterID]; ok {
		t.Requester = u.Ref()
	}
	t.Assignee = nil
	if row.assigneeID != nil {
		if u, ok := r.s.users[*row.assigneeID]; ok {
			ref := u.Ref()
			t.Assignee = &ref
		}
	}
	return t
}

func (r *ticketRepo) Create(_ context.Context, t *domain.Ticket) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	now := r.s.now()
	t.ID = r.s.nextTicket
	r.s.nextTicket++
	t.CreatedAt, t.UpdatedAt = now, now
	row := &ticketRow{Ticket: *t, requesterID: t.Requester.ID}
	if t.Assignee != nil {
		id := t.Assignee.ID
		row.assigneeID = &id
	}
	r.s.tickets[t.ID] = row
	return nil
}

func (r *ticketRepo) Get(_ context.Context, id int64) (*domain.Ticket, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	row, ok := r.s.tickets[id]
	if !ok {
		return nil, domain.NotFound("ticket")
	}
	t := r.hydrate(row)
	return &t, nil
}

func (r *ticketRepo) List(_ context.Context, f service.TicketFilter) ([]domain.Ticket, int, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()

	q := strings.ToLower(f.Query)
	var out []domain.Ticket
	for _, row := range r.s.tickets {
		switch {
		case f.RequesterID != nil && row.requesterID != *f.RequesterID:
			continue
		case f.Status != nil && row.Status != *f.Status:
			continue
		case f.Priority != nil && row.Priority != *f.Priority:
			continue
		case f.Unassigned && row.assigneeID != nil:
			continue
		case f.AssigneeID != nil && (row.assigneeID == nil || *row.assigneeID != *f.AssigneeID):
			continue
		case q != "" && !strings.Contains(strings.ToLower(row.Title), q):
			continue
		}
		out = append(out, r.hydrate(row))
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var cmp int
		switch f.SortBy {
		case "title":
			cmp = strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		case "status":
			cmp = statusRank(a.Status) - statusRank(b.Status)
		case "priority":
			cmp = a.Priority.Rank() - b.Priority.Rank()
		case "updated_at":
			cmp = a.UpdatedAt.Compare(b.UpdatedAt)
		default:
			cmp = a.CreatedAt.Compare(b.CreatedAt)
		}
		if cmp == 0 {
			return a.ID > b.ID // stable tiebreak, newest first (same as the SQL)
		}
		if f.SortDesc {
			return cmp > 0
		}
		return cmp < 0
	})
	return paginate(out, f.Page, f.PageSize)
}

func statusRank(s domain.Status) int {
	for i, v := range domain.Statuses {
		if v == s {
			return i + 1
		}
	}
	return 0
}

func (r *ticketRepo) Update(_ context.Context, id int64, u service.TicketUpdate) (*domain.Ticket, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	row, ok := r.s.tickets[id]
	if !ok {
		return nil, domain.NotFound("ticket")
	}
	if u.ExpectedStatus != "" && row.Status != u.ExpectedStatus {
		return nil, domain.Conflict("stale_ticket", "the ticket was changed by someone else; reload and try again")
	}
	if u.Title != nil {
		row.Title = *u.Title
	}
	if u.Description != nil {
		row.Description = *u.Description
	}
	if u.Category != nil {
		row.Category = *u.Category
	}
	if u.Priority != nil {
		row.Priority = *u.Priority
	}
	if u.Status != nil {
		row.Status = *u.Status
	}
	if u.SetAssignee {
		row.assigneeID = nil
		if u.AssigneeID != nil {
			v := *u.AssigneeID
			row.assigneeID = &v
		}
	}
	row.UpdatedAt = r.s.now()
	t := r.hydrate(row)
	return &t, nil
}

func (r *ticketRepo) Delete(_ context.Context, id int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.tickets[id]; !ok {
		return domain.NotFound("ticket")
	}
	delete(r.s.tickets, id)
	kept := r.s.comments[:0]
	for _, c := range r.s.comments {
		if c.TicketID != id {
			kept = append(kept, c)
		}
	}
	r.s.comments = kept
	return nil
}

func (r *ticketRepo) AddComment(_ context.Context, c *domain.Comment) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	row, ok := r.s.tickets[c.TicketID]
	if !ok {
		return domain.NotFound("ticket")
	}
	c.ID = r.s.nextComment
	r.s.nextComment++
	c.CreatedAt = r.s.now()
	row.UpdatedAt = c.CreatedAt
	cp := *c
	r.s.comments = append(r.s.comments, &cp)
	return nil
}

func (r *ticketRepo) ListComments(_ context.Context, ticketID int64, includeInternal bool) ([]domain.Comment, error) {
	r.s.mu.RLock()
	defer r.s.mu.RUnlock()
	out := []domain.Comment{}
	for _, c := range r.s.comments {
		if c.TicketID != ticketID || (c.IsInternal && !includeInternal) {
			continue
		}
		cp := *c
		if u, ok := r.s.users[c.Author.ID]; ok {
			cp.Author = u.Ref()
		}
		out = append(out, cp)
	}
	return out, nil
}
