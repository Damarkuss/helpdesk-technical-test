// Package seed inserts demo accounts and sample tickets. It is idempotent:
// existing accounts are left alone and sample tickets are only created on an empty database.
package seed

import (
	"context"
	"errors"
	"log/slog"

	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/service"
)

// Accounts are the demo logins. The password comes from configuration (SEED_PASSWORD).
var Accounts = []struct {
	Name  string
	Email string
	Role  domain.Role
}{
	{"Adi Admin", "admin@helpdesk.local", domain.RoleAdmin},
	{"Agus Agent", "agent@helpdesk.local", domain.RoleAgent},
	{"Sari Support", "agent2@helpdesk.local", domain.RoleAgent},
	{"Rina Requester", "requester@helpdesk.local", domain.RoleRequester},
	{"Budi Karyawan", "requester2@helpdesk.local", domain.RoleRequester},
}

type Deps struct {
	Users    service.UserRepository
	Tickets  service.TicketRepository
	Hasher   service.PasswordHasher
	Password string
	Log      *slog.Logger
}

func Run(ctx context.Context, d Deps) error {
	byEmail := map[string]*domain.User{}
	for _, a := range Accounts {
		u, err := d.Users.GetByEmail(ctx, a.Email)
		var de *domain.Error
		switch {
		case err == nil:
		case errors.As(err, &de) && de.Kind == domain.KindNotFound:
			hash, herr := d.Hasher.Hash(d.Password)
			if herr != nil {
				return herr
			}
			u = &domain.User{Name: a.Name, Email: a.Email, PasswordHash: hash, Role: a.Role}
			if err := d.Users.Create(ctx, u); err != nil {
				return err
			}
			d.Log.Info("seeded user", "email", a.Email, "role", a.Role)
		default:
			return err
		}
		byEmail[a.Email] = u
	}

	_, total, err := d.Tickets.List(ctx, service.TicketFilter{Page: 1, PageSize: 1})
	if err != nil {
		return err
	}
	if total > 0 {
		return nil // already has data; do not add duplicates on restart
	}
	return sampleTickets(ctx, d, byEmail)
}

func sampleTickets(ctx context.Context, d Deps, u map[string]*domain.User) error {
	rina, budi := u["requester@helpdesk.local"], u["requester2@helpdesk.local"]
	agus, sari := u["agent@helpdesk.local"], u["agent2@helpdesk.local"]

	type comment struct {
		by       *domain.User
		body     string
		internal bool
	}
	samples := []struct {
		owner       *domain.User
		title, desc string
		category    string
		priority    domain.Priority
		status      domain.Status
		assignee    *domain.User
		comments    []comment
	}{
		{rina, "Laptop will not power on", "My laptop does not turn on since this morning. The charging light is blinking orange.",
			"hardware", domain.PriorityHigh, domain.StatusOpen, nil, nil},
		{rina, "VPN connection fails from home", "The VPN client says \"authentication failed\" although my password works on the intranet.",
			"network", domain.PriorityUrgent, domain.StatusInProgress, agus, []comment{
				{rina, "I need this before the 3pm client call.", false},
				{agus, "Thanks Rina, I am checking your account on the VPN gateway now.", false},
				{agus, "Account was locked after 5 failed attempts. Unlocking and resetting the VPN token.", true},
			}},
		{rina, "Please install Figma", "Design team asked me to review mockups. I need Figma installed on my work laptop.",
			"software", domain.PriorityLow, domain.StatusResolved, agus, []comment{
				{agus, "Installed and licence assigned. Please confirm it works and close the ticket.", false},
			}},
		{budi, "Reset my email password", "I forgot my password after the holiday and the self-service reset link does not arrive.",
			"account_access", domain.PriorityMedium, domain.StatusClosed, sari, []comment{
				{sari, "Password reset done. A temporary password was sent to your phone.", false},
				{budi, "Works now, thank you!", false},
			}},
		{budi, "Printer on 3rd floor keeps jamming", "Paper jams every time we print double-sided. Several people are affected.",
			"hardware", domain.PriorityMedium, domain.StatusOpen, nil, nil},
	}

	for _, s := range samples {
		t := &domain.Ticket{
			Title: s.title, Description: s.desc, Category: s.category, Priority: s.priority,
			Status: domain.StatusOpen, Requester: s.owner.Ref(),
		}
		if err := d.Tickets.Create(ctx, t); err != nil {
			return err
		}
		if s.status != domain.StatusOpen || s.assignee != nil {
			upd := service.TicketUpdate{Status: &s.status}
			if s.assignee != nil {
				upd.SetAssignee, upd.AssigneeID = true, &s.assignee.ID
			}
			if _, err := d.Tickets.Update(ctx, t.ID, upd); err != nil {
				return err
			}
		}
		for _, c := range s.comments {
			if err := d.Tickets.AddComment(ctx, &domain.Comment{TicketID: t.ID, Author: c.by.Ref(), Body: c.body, IsInternal: c.internal}); err != nil {
				return err
			}
		}
	}
	d.Log.Info("seeded sample tickets", "count", len(samples))
	return nil
}
