package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/paska/helpdesk/internal/domain"
)

// TicketService implements ticket and comment use cases, including all
// role-based access rules and the status workflow.
type TicketService struct {
	tickets TicketRepository
	users   UserRepository
	now     func() time.Time

	// urgentSLA is how long an urgent ticket may stay "open" before it is flagged overdue.
	// Zero disables the flag.
	urgentSLA time.Duration
}

func NewTicketService(tickets TicketRepository, users UserRepository, urgentSLA time.Duration, now func() time.Time) *TicketService {
	if now == nil {
		now = time.Now
	}
	return &TicketService{tickets: tickets, users: users, urgentSLA: urgentSLA, now: now}
}

// TicketView is a ticket as seen by one specific user: it carries the status
// changes that user may perform right now, so clients never duplicate the workflow.
type TicketView struct {
	domain.Ticket
	AllowedTransitions []domain.Status `json:"allowed_transitions"`
	IsOverdue          bool            `json:"is_overdue"`
}

// TicketDetail is a TicketView plus the comments the user is allowed to see.
type TicketDetail struct {
	TicketView
	Comments []domain.Comment `json:"comments"`
}

func (s *TicketService) view(t *domain.Ticket, actor *domain.User) TicketView {
	overdue := s.urgentSLA > 0 &&
		t.Priority == domain.PriorityUrgent &&
		t.Status == domain.StatusOpen &&
		s.now().Sub(t.CreatedAt) > s.urgentSLA
	return TicketView{
		Ticket:             *t,
		AllowedTransitions: domain.AllowedTransitions(t.Status, actor.Role, t.IsOwnedBy(actor.ID)),
		IsOverdue:          overdue,
	}
}

// canRead: staff can read every ticket, everyone else only their own.
func canRead(actor *domain.User, t *domain.Ticket) bool {
	return actor.Role.IsStaff() || t.IsOwnedBy(actor.ID)
}

// load fetches a ticket and enforces read access.
func (s *TicketService) load(ctx context.Context, actor *domain.User, id int64) (*domain.Ticket, error) {
	t, err := s.tickets.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canRead(actor, t) {
		return nil, domain.Forbidden("you do not have access to this ticket")
	}
	return t, nil
}

// ---- create -----------------------------------------------------------------

type CreateTicketInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Priority    string `json:"priority"`
}

func (s *TicketService) Create(ctx context.Context, actor *domain.User, in CreateTicketInput) (*TicketView, error) {
	title := strings.TrimSpace(in.Title)
	desc := strings.TrimSpace(in.Description)
	category := strings.TrimSpace(in.Category)
	priority := domain.Priority(strings.TrimSpace(in.Priority))
	if category == "" {
		category = "general"
	}
	if priority == "" {
		priority = domain.PriorityMedium
	}

	fields := map[string]string{}
	checkLen(fields, "title", title, 3, 200)
	checkLen(fields, "description", desc, 1, 5000)
	if !domain.ValidCategory(category) {
		fields["category"] = "must be one of: " + strings.Join(domain.Categories, ", ")
	}
	if !priority.Valid() {
		fields["priority"] = "must be one of: low, medium, high, urgent"
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields)
	}

	t := &domain.Ticket{
		Title: title, Description: desc, Category: category, Priority: priority,
		Status: domain.StatusOpen, Requester: actor.Ref(),
	}
	if err := s.tickets.Create(ctx, t); err != nil {
		return nil, err
	}
	v := s.view(t, actor)
	return &v, nil
}

// ---- read -------------------------------------------------------------------

type ListTicketsInput struct {
	Status   string
	Priority string
	Assignee string // "", "me", "unassigned" or a user id
	Query    string
	Sort     string
	Order    string // asc | desc
	Page     int
	PageSize int
}

var sortable = map[string]bool{"created_at": true, "updated_at": true, "title": true, "status": true, "priority": true}

func (s *TicketService) List(ctx context.Context, actor *domain.User, in ListTicketsInput) (*Page[TicketView], error) {
	page, size := pagination(in.Page, in.PageSize)
	f := TicketFilter{Query: strings.TrimSpace(in.Query), SortBy: "created_at", SortDesc: true, Page: page, PageSize: size}
	fields := map[string]string{}

	// Requesters only ever see their own tickets, whatever filters they send.
	if !actor.Role.IsStaff() {
		id := actor.ID
		f.RequesterID = &id
	}

	if in.Status != "" {
		st := domain.Status(in.Status)
		if !st.Valid() {
			fields["status"] = "must be one of: open, in_progress, resolved, closed"
		}
		f.Status = &st
	}
	if in.Priority != "" {
		p := domain.Priority(in.Priority)
		if !p.Valid() {
			fields["priority"] = "must be one of: low, medium, high, urgent"
		}
		f.Priority = &p
	}
	switch in.Assignee {
	case "":
	case "unassigned":
		f.Unassigned = true
	case "me":
		id := actor.ID
		f.AssigneeID = &id
	default:
		id, err := strconv.ParseInt(in.Assignee, 10, 64)
		if err != nil || id <= 0 {
			fields["assignee"] = `must be "me", "unassigned" or a user id`
		} else {
			f.AssigneeID = &id
		}
	}
	if in.Sort != "" {
		if !sortable[in.Sort] {
			fields["sort"] = "must be one of: created_at, updated_at, title, status, priority"
		}
		f.SortBy = in.Sort
	}
	switch strings.ToLower(in.Order) {
	case "", "desc":
	case "asc":
		f.SortDesc = false
	default:
		fields["order"] = "must be asc or desc"
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields)
	}

	rows, total, err := s.tickets.List(ctx, f)
	if err != nil {
		return nil, err
	}
	views := make([]TicketView, len(rows))
	for i := range rows {
		views[i] = s.view(&rows[i], actor)
	}
	return newPage(views, page, size, total), nil
}

func (s *TicketService) Get(ctx context.Context, actor *domain.User, id int64) (*TicketDetail, error) {
	t, err := s.load(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	// Internal notes are only fetched for staff, so they cannot leak by accident.
	comments, err := s.tickets.ListComments(ctx, id, actor.Role.IsStaff())
	if err != nil {
		return nil, err
	}
	return &TicketDetail{TicketView: s.view(t, actor), Comments: comments}, nil
}

// ---- update -----------------------------------------------------------------

// OptionalID distinguishes "field absent" from "field set to null" in a JSON PATCH body.
type OptionalID struct {
	Set   bool
	Value *int64
}

func (o *OptionalID) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return errors.New("must be a user id or null")
	}
	o.Value = &v
	return nil
}

type UpdateTicketInput struct {
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Category    *string    `json:"category"`
	Priority    *string    `json:"priority"`
	Status      *string    `json:"status"`
	AssigneeID  OptionalID `json:"assignee_id"`
}

func (in UpdateTicketInput) empty() bool {
	return in.Title == nil && in.Description == nil && in.Category == nil &&
		in.Priority == nil && in.Status == nil && !in.AssigneeID.Set
}

func (s *TicketService) Update(ctx context.Context, actor *domain.User, id int64, in UpdateTicketInput) (*TicketView, error) {
	if in.empty() {
		return nil, domain.Validation(map[string]string{"_": "provide at least one field to update"})
	}
	t, err := s.load(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	isOwner, staff := t.IsOwnedBy(actor.ID), actor.Role.IsStaff()
	upd := TicketUpdate{ExpectedStatus: t.Status}

	// 1) Format validation of every supplied value.
	fields := map[string]string{}
	if in.Title != nil {
		v := strings.TrimSpace(*in.Title)
		checkLen(fields, "title", v, 3, 200)
		in.Title = &v
	}
	if in.Description != nil {
		v := strings.TrimSpace(*in.Description)
		checkLen(fields, "description", v, 1, 5000)
		in.Description = &v
	}
	if in.Category != nil && !domain.ValidCategory(*in.Category) {
		fields["category"] = "must be one of: " + strings.Join(domain.Categories, ", ")
	}
	var newPriority *domain.Priority
	if in.Priority != nil {
		p := domain.Priority(*in.Priority)
		if !p.Valid() {
			fields["priority"] = "must be one of: low, medium, high, urgent"
		}
		newPriority = &p
	}
	var newStatus *domain.Status
	if in.Status != nil {
		st := domain.Status(*in.Status)
		if !st.Valid() {
			fields["status"] = "must be one of: open, in_progress, resolved, closed"
		}
		newStatus = &st
	}
	if len(fields) > 0 {
		return nil, domain.Validation(fields)
	}

	// 2) Status workflow (rejects anything outside the transition table).
	if newStatus != nil && *newStatus != t.Status {
		if err := domain.CheckTransition(t.Status, *newStatus, actor.Role, isOwner); err != nil {
			return nil, err
		}
		upd.Status = newStatus
	}

	// 3) A closed ticket is read-only.
	if t.Status == domain.StatusClosed && (in.Title != nil || in.Description != nil || in.Category != nil ||
		in.Priority != nil || in.AssigneeID.Set) {
		return nil, domain.Conflict("ticket_closed", "a closed ticket can no longer be changed")
	}

	// 4) Content: staff always, the owner only while the ticket is still open.
	if in.Title != nil || in.Description != nil || in.Category != nil {
		if !staff && !(isOwner && t.Status == domain.StatusOpen) {
			return nil, domain.Forbidden("you can only edit your own ticket while it is open")
		}
		if in.Title != nil && *in.Title != t.Title {
			upd.Title = in.Title
		}
		if in.Description != nil && *in.Description != t.Description {
			upd.Description = in.Description
		}
		if in.Category != nil && *in.Category != t.Category {
			upd.Category = in.Category
		}
	}

	// 5) Priority: staff only.
	if newPriority != nil {
		if !staff {
			return nil, domain.Forbidden("only agents and admins can change the priority")
		}
		if *newPriority != t.Priority {
			upd.Priority = newPriority
		}
	}

	// 6) Assignee: agents can take a ticket for themselves; admins can assign any staff member.
	if in.AssigneeID.Set {
		if !staff {
			return nil, domain.Forbidden("only agents and admins can assign tickets")
		}
		target := in.AssigneeID.Value
		if target != nil {
			if actor.Role == domain.RoleAgent && *target != actor.ID {
				return nil, domain.Forbidden("agents can only assign tickets to themselves; ask an admin to assign others")
			}
			u, err := s.users.GetByID(ctx, *target)
			if err != nil {
				var de *domain.Error
				if errors.As(err, &de) && de.Kind == domain.KindNotFound {
					return nil, domain.FieldError("assignee_id", "user not found")
				}
				return nil, err
			}
			if !u.Role.IsStaff() {
				return nil, domain.FieldError("assignee_id", "assignee must be an agent or admin")
			}
		}
		if !sameID(t.Assignee, target) {
			upd.SetAssignee, upd.AssigneeID = true, target
		}
	} else if upd.Status != nil && *upd.Status == domain.StatusInProgress && t.Assignee == nil {
		// Starting work on an unassigned ticket takes ownership of it.
		id := actor.ID
		upd.SetAssignee, upd.AssigneeID = true, &id
	}

	// Nothing actually differs from the stored ticket: succeed without touching it.
	if upd.Title == nil && upd.Description == nil && upd.Category == nil &&
		upd.Priority == nil && upd.Status == nil && !upd.SetAssignee {
		v := s.view(t, actor)
		return &v, nil
	}

	updated, err := s.tickets.Update(ctx, id, upd)
	if err != nil {
		return nil, err
	}
	v := s.view(updated, actor)
	return &v, nil
}

func sameID(a *domain.UserRef, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.ID == *b
}

// Delete permanently removes a ticket. Admin only.
func (s *TicketService) Delete(ctx context.Context, actor *domain.User, id int64) error {
	if actor.Role != domain.RoleAdmin {
		return domain.Forbidden("only admins can delete tickets")
	}
	return s.tickets.Delete(ctx, id)
}

// ---- comments ---------------------------------------------------------------

type AddCommentInput struct {
	Body       string `json:"body"`
	IsInternal bool   `json:"is_internal"`
}

func (s *TicketService) AddComment(ctx context.Context, actor *domain.User, ticketID int64, in AddCommentInput) (*domain.Comment, error) {
	t, err := s.load(ctx, actor, ticketID)
	if err != nil {
		return nil, err
	}
	if in.IsInternal && !actor.Role.IsStaff() {
		return nil, domain.Forbidden("only agents and admins can write internal notes")
	}
	body := strings.TrimSpace(in.Body)
	fields := map[string]string{}
	checkLen(fields, "body", body, 1, 5000)
	if len(fields) > 0 {
		return nil, domain.Validation(fields)
	}
	if t.Status == domain.StatusClosed {
		return nil, domain.Conflict("ticket_closed", fmt.Sprintf("ticket #%d is closed and no longer accepts comments", t.ID))
	}

	c := &domain.Comment{TicketID: ticketID, Author: actor.Ref(), Body: body, IsInternal: in.IsInternal}
	if err := s.tickets.AddComment(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}
