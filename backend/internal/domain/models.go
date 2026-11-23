// Package domain holds the core business types and rules of the helpdesk.
// It has no dependencies on HTTP, SQL or any framework, so it is trivial to test.
package domain

import "time"

// Role is a user's permission level.
type Role string

const (
	RoleRequester Role = "requester"
	RoleAgent     Role = "agent"
	RoleAdmin     Role = "admin"
)

// Roles lists every valid role.
var Roles = []Role{RoleRequester, RoleAgent, RoleAdmin}

func (r Role) Valid() bool {
	for _, v := range Roles {
		if r == v {
			return true
		}
	}
	return false
}

// IsStaff reports whether the role belongs to a helpdesk agent or admin.
func (r Role) IsStaff() bool { return r == RoleAgent || r == RoleAdmin }

// Status is the lifecycle state of a ticket.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusResolved   Status = "resolved"
	StatusClosed     Status = "closed"
)

// Statuses lists every valid status.
var Statuses = []Status{StatusOpen, StatusInProgress, StatusResolved, StatusClosed}

func (s Status) Valid() bool {
	for _, v := range Statuses {
		if s == v {
			return true
		}
	}
	return false
}

// Priority is how urgent a ticket is.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Priorities lists every valid priority, from least to most urgent.
var Priorities = []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent}

func (p Priority) Valid() bool {
	for _, v := range Priorities {
		if p == v {
			return true
		}
	}
	return false
}

// Rank orders priorities (low=1 ... urgent=4); unknown values rank 0.
func (p Priority) Rank() int {
	for i, v := range Priorities {
		if p == v {
			return i + 1
		}
	}
	return 0
}

// Categories is the fixed set of ticket categories.
var Categories = []string{"general", "hardware", "software", "network", "account_access", "other"}

func ValidCategory(c string) bool {
	for _, v := range Categories {
		if c == v {
			return true
		}
	}
	return false
}

// User is an account. PasswordHash is never serialised.
type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// Ref returns the lightweight public reference to the user.
func (u *User) Ref() UserRef { return UserRef{ID: u.ID, Name: u.Name, Role: u.Role} }

// UserRef is the minimal public view of a user embedded in tickets and comments.
type UserRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role Role   `json:"role"`
}

// Ticket is a support request.
type Ticket struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Priority    Priority  `json:"priority"`
	Status      Status    `json:"status"`
	Requester   UserRef   `json:"requester"`
	Assignee    *UserRef  `json:"assignee"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IsOwnedBy reports whether the user created the ticket.
func (t *Ticket) IsOwnedBy(userID int64) bool { return t.Requester.ID == userID }

// Comment is a message on a ticket. Internal comments are staff-only notes.
type Comment struct {
	ID         int64     `json:"id"`
	TicketID   int64     `json:"ticket_id"`
	Author     UserRef   `json:"author"`
	Body       string    `json:"body"`
	IsInternal bool      `json:"is_internal"`
	CreatedAt  time.Time `json:"created_at"`
}
