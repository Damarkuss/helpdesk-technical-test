package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/service"
	"github.com/paska/helpdesk/internal/testutil"
)

var ctx = context.Background()

func str(s string) *string { return &s }

func wantKind(t *testing.T, err error, kind domain.Kind) {
	t.Helper()
	var de *domain.Error
	if !errors.As(err, &de) {
		t.Fatalf("want domain error of kind %d, got %v", kind, err)
	}
	if de.Kind != kind {
		t.Fatalf("want kind %d, got kind %d (%s: %s)", kind, de.Kind, de.Code, de.Message)
	}
}

// Access control: a requester must not be able to read someone else's ticket.
func TestRequesterCannotReadAnotherRequestersTicket(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)
	bob := app.NewUser("bob", domain.RoleRequester)
	agent := app.NewUser("agent", domain.RoleAgent)
	ticket := app.NewTicket(alice, "Alice's laptop")

	_, err := app.Tickets.Get(ctx, bob, ticket.ID)
	wantKind(t, err, domain.KindForbidden)

	// They cannot modify or comment either.
	_, err = app.Tickets.Update(ctx, bob, ticket.ID, service.UpdateTicketInput{Status: str("closed")})
	wantKind(t, err, domain.KindForbidden)
	_, err = app.Tickets.AddComment(ctx, bob, ticket.ID, service.AddCommentInput{Body: "hi"})
	wantKind(t, err, domain.KindForbidden)

	// The owner and staff can read it.
	if _, err := app.Tickets.Get(ctx, alice, ticket.ID); err != nil {
		t.Errorf("owner: %v", err)
	}
	if _, err := app.Tickets.Get(ctx, agent, ticket.ID); err != nil {
		t.Errorf("agent: %v", err)
	}
}

// The list endpoint is scoped to the caller for requesters, even if they ask for more.
func TestRequesterListIsScopedToOwnTickets(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)
	bob := app.NewUser("bob", domain.RoleRequester)
	agent := app.NewUser("agent", domain.RoleAgent)
	app.NewTicket(alice, "A one")
	app.NewTicket(alice, "A two")
	app.NewTicket(bob, "B one")

	res, err := app.Tickets.List(ctx, alice, service.ListTicketsInput{Assignee: "unassigned"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 {
		t.Fatalf("alice sees %d tickets, want 2", res.Total)
	}
	for _, tk := range res.Items {
		if tk.Requester.ID != alice.ID {
			t.Errorf("alice was shown ticket %d owned by %d", tk.ID, tk.Requester.ID)
		}
	}

	all, _ := app.Tickets.List(ctx, agent, service.ListTicketsInput{})
	if all.Total != 3 {
		t.Errorf("agent sees %d tickets, want 3", all.Total)
	}
}

// Internal notes are visible to staff only, and only staff can write them.
func TestInternalNotesAreStaffOnly(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)
	agent := app.NewUser("agent", domain.RoleAgent)
	ticket := app.NewTicket(alice, "VPN down")

	if _, err := app.Tickets.AddComment(ctx, alice, ticket.ID, service.AddCommentInput{Body: "still broken"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Tickets.AddComment(ctx, agent, ticket.ID, service.AddCommentInput{Body: "checking logs", IsInternal: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Tickets.AddComment(ctx, agent, ticket.ID, service.AddCommentInput{Body: "we are on it"}); err != nil {
		t.Fatal(err)
	}

	// A requester cannot write an internal note.
	_, err := app.Tickets.AddComment(ctx, alice, ticket.ID, service.AddCommentInput{Body: "sneaky", IsInternal: true})
	wantKind(t, err, domain.KindForbidden)

	asRequester, _ := app.Tickets.Get(ctx, alice, ticket.ID)
	if len(asRequester.Comments) != 2 {
		t.Fatalf("requester sees %d comments, want 2", len(asRequester.Comments))
	}
	for _, c := range asRequester.Comments {
		if c.IsInternal {
			t.Error("requester was shown an internal note")
		}
	}

	asAgent, _ := app.Tickets.Get(ctx, agent, ticket.ID)
	if len(asAgent.Comments) != 3 {
		t.Errorf("agent sees %d comments, want 3", len(asAgent.Comments))
	}
}

// Full lifecycle with the right actor at each step.
func TestWorkflowHappyPathAndReopen(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)
	agent := app.NewUser("agent", domain.RoleAgent)
	ticket := app.NewTicket(alice, "Printer jam")

	// Agent starts work: the unassigned ticket is auto-assigned to them.
	tv, err := app.Tickets.Update(ctx, agent, ticket.ID, service.UpdateTicketInput{Status: str("in_progress")})
	if err != nil {
		t.Fatal(err)
	}
	if tv.Status != domain.StatusInProgress || tv.Assignee == nil || tv.Assignee.ID != agent.ID {
		t.Fatalf("after start: status=%s assignee=%v", tv.Status, tv.Assignee)
	}

	if _, err := app.Tickets.Update(ctx, agent, ticket.ID, service.UpdateTicketInput{Status: str("resolved")}); err != nil {
		t.Fatal(err)
	}

	// Requester reopens, agent resolves again, requester closes.
	tv, err = app.Tickets.Update(ctx, alice, ticket.ID, service.UpdateTicketInput{Status: str("open")})
	if err != nil || tv.Status != domain.StatusOpen {
		t.Fatalf("reopen: %v / %v", tv, err)
	}
	for _, step := range []struct {
		actor  *domain.User
		status string
	}{{agent, "in_progress"}, {agent, "resolved"}, {alice, "closed"}} {
		if _, err := app.Tickets.Update(ctx, step.actor, ticket.ID, service.UpdateTicketInput{Status: str(step.status)}); err != nil {
			t.Fatalf("-> %s: %v", step.status, err)
		}
	}

	// Closed is terminal and read-only.
	_, err = app.Tickets.Update(ctx, agent, ticket.ID, service.UpdateTicketInput{Status: str("open")})
	wantKind(t, err, domain.KindInvalidTransition)
	_, err = app.Tickets.Update(ctx, agent, ticket.ID, service.UpdateTicketInput{Priority: str("urgent")})
	wantKind(t, err, domain.KindConflict)
	_, err = app.Tickets.AddComment(ctx, alice, ticket.ID, service.AddCommentInput{Body: "thanks"})
	wantKind(t, err, domain.KindConflict)
}

// Transitions outside the table are refused by the service with the right error kind.
func TestInvalidAndUnauthorizedTransitions(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)
	agent := app.NewUser("agent", domain.RoleAgent)
	admin := app.NewUser("admin", domain.RoleAdmin)
	ticket := app.NewTicket(alice, "Slow wifi")

	cases := []struct {
		name   string
		actor  *domain.User
		status string
		kind   domain.Kind
	}{
		{"open->resolved skips a step", agent, "resolved", domain.KindInvalidTransition},
		{"open->open is not a change but unknown status is", agent, "banana", domain.KindValidation},
		{"requester cannot start work", alice, "in_progress", domain.KindForbidden},
		{"agent cannot cancel", agent, "closed", domain.KindForbidden},
	}
	for _, c := range cases {
		_, err := app.Tickets.Update(ctx, c.actor, ticket.ID, service.UpdateTicketInput{Status: str(c.status)})
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		wantKind(t, err, c.kind)
	}

	// Ticket must be unchanged after all the rejected attempts.
	got, _ := app.Tickets.Get(ctx, admin, ticket.ID)
	if got.Status != domain.StatusOpen {
		t.Errorf("status changed to %s despite rejections", got.Status)
	}

	// Admin may cancel an open ticket; so may the requester on their own.
	if _, err := app.Tickets.Update(ctx, admin, ticket.ID, service.UpdateTicketInput{Status: str("closed")}); err != nil {
		t.Errorf("admin cancel: %v", err)
	}
}

func TestAssignmentRules(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)
	agent := app.NewUser("agent", domain.RoleAgent)
	agent2 := app.NewUser("agent2", domain.RoleAgent)
	admin := app.NewUser("admin", domain.RoleAdmin)
	ticket := app.NewTicket(alice, "Install request")

	id := func(u *domain.User) *int64 { return &u.ID }
	assign := func(actor *domain.User, to *int64) error {
		_, err := app.Tickets.Update(ctx, actor, ticket.ID, service.UpdateTicketInput{AssigneeID: service.OptionalID{Set: true, Value: to}})
		return err
	}

	wantKind(t, assign(alice, id(agent)), domain.KindForbidden)  // requester cannot assign
	wantKind(t, assign(agent, id(agent2)), domain.KindForbidden) // agent cannot give work to others
	if err := assign(agent, id(agent)); err != nil {             // agent takes it
		t.Errorf("agent self-assign: %v", err)
	}
	if err := assign(admin, id(agent2)); err != nil { // admin can reassign
		t.Errorf("admin assign: %v", err)
	}
	wantKind(t, assign(admin, id(alice)), domain.KindValidation) // assignee must be staff
	nobody := int64(9999)
	wantKind(t, assign(admin, &nobody), domain.KindValidation) // unknown user
	if err := assign(admin, nil); err != nil {                 // unassign
		t.Errorf("unassign: %v", err)
	}
}

func TestFilterSortAndPagination(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)
	agent := app.NewUser("agent", domain.RoleAgent)

	titles := []string{"Alpha printer", "Bravo VPN", "Charlie printer", "Delta laptop", "Echo printer"}
	for i, title := range titles {
		tv := app.NewTicket(alice, title)
		*app.Clock = app.Clock.Add(time.Minute) // distinct created_at values
		if i == 1 {
			if _, err := app.Tickets.Update(ctx, agent, tv.ID, service.UpdateTicketInput{Priority: str("urgent")}); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Search is a case-insensitive substring match on the title.
	res, err := app.Tickets.List(ctx, agent, service.ListTicketsInput{Query: "PRINTER"})
	if err != nil || res.Total != 3 {
		t.Fatalf("search: total=%v err=%v", res, err)
	}

	// Filter by priority.
	res, _ = app.Tickets.List(ctx, agent, service.ListTicketsInput{Priority: "urgent"})
	if res.Total != 1 || res.Items[0].Title != "Bravo VPN" {
		t.Errorf("priority filter: %+v", res)
	}

	// Default order is newest first; pagination splits 5 rows into 2+2+1.
	p1, _ := app.Tickets.List(ctx, agent, service.ListTicketsInput{PageSize: 2, Page: 1})
	p3, _ := app.Tickets.List(ctx, agent, service.ListTicketsInput{PageSize: 2, Page: 3})
	if p1.Total != 5 || p1.TotalPages != 3 || len(p1.Items) != 2 || p1.Items[0].Title != "Echo printer" {
		t.Errorf("page 1: %+v", p1)
	}
	if len(p3.Items) != 1 || p3.Items[0].Title != "Alpha printer" {
		t.Errorf("page 3: %+v", p3)
	}

	// Sort by title ascending.
	asc, _ := app.Tickets.List(ctx, agent, service.ListTicketsInput{Sort: "title", Order: "asc"})
	if asc.Items[0].Title != "Alpha printer" || asc.Items[4].Title != "Echo printer" {
		t.Errorf("title asc: first=%s last=%s", asc.Items[0].Title, asc.Items[4].Title)
	}

	// Bad parameters are rejected rather than ignored.
	_, err = app.Tickets.List(ctx, agent, service.ListTicketsInput{Sort: "password_hash"})
	wantKind(t, err, domain.KindValidation)
	_, err = app.Tickets.List(ctx, agent, service.ListTicketsInput{Status: "nope"})
	wantKind(t, err, domain.KindValidation)
}

func TestUrgentTicketBecomesOverdue(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)
	agent := app.NewUser("agent", domain.RoleAgent)

	tv, err := app.Tickets.Create(ctx, alice, service.CreateTicketInput{Title: "Server down", Description: "Prod is down", Priority: "urgent"})
	if err != nil {
		t.Fatal(err)
	}
	if tv.IsOverdue {
		t.Fatal("fresh ticket must not be overdue")
	}

	*app.Clock = app.Clock.Add(5 * time.Hour) // SLA in the fixture is 4h
	got, _ := app.Tickets.Get(ctx, agent, tv.ID)
	if !got.IsOverdue {
		t.Error("urgent open ticket older than the SLA should be overdue")
	}

	// Once someone picks it up it is no longer overdue.
	if _, err := app.Tickets.Update(ctx, agent, tv.ID, service.UpdateTicketInput{Status: str("in_progress")}); err != nil {
		t.Fatal(err)
	}
	got, _ = app.Tickets.Get(ctx, agent, tv.ID)
	if got.IsOverdue {
		t.Error("in-progress ticket should not be overdue")
	}
}

func TestCreateTicketValidation(t *testing.T) {
	app := testutil.NewApp(t)
	alice := app.NewUser("alice", domain.RoleRequester)

	_, err := app.Tickets.Create(ctx, alice, service.CreateTicketInput{Title: "x", Description: "", Category: "bogus", Priority: "asap"})
	var de *domain.Error
	if !errors.As(err, &de) || de.Kind != domain.KindValidation {
		t.Fatalf("err = %v", err)
	}
	for _, f := range []string{"title", "description", "category", "priority"} {
		if de.Fields[f] == "" {
			t.Errorf("no message for field %q: %v", f, de.Fields)
		}
	}
}
