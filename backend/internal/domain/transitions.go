package domain

import "fmt"

// transition describes one allowed edge of the ticket workflow and who may take it.
// "Owner" means the user who created the ticket (the requester of that ticket).
type transition struct {
	From, To Status
	Agent    bool
	Admin    bool
	Owner    bool
}

// workflow is the single source of truth for status changes. It mirrors the
// table in the task brief; anything not listed here is rejected.
var workflow = []transition{
	{From: StatusOpen, To: StatusInProgress, Agent: true, Admin: true},
	{From: StatusInProgress, To: StatusResolved, Agent: true, Admin: true},
	{From: StatusResolved, To: StatusClosed, Agent: true, Admin: true, Owner: true},
	{From: StatusResolved, To: StatusOpen, Owner: true}, // requester reopens
	{From: StatusOpen, To: StatusClosed, Admin: true, Owner: true}, // requester cancels
}

func (t transition) permits(role Role, isOwner bool) bool {
	switch {
	case isOwner && t.Owner:
		return true
	case role == RoleAgent && t.Agent:
		return true
	case role == RoleAdmin && t.Admin:
		return true
	}
	return false
}

// CheckTransition validates a status change for an actor.
//
//   - the edge does not exist in the workflow  -> KindInvalidTransition (422)
//   - the edge exists but the actor may not use it -> KindForbidden (403)
func CheckTransition(from, to Status, role Role, isOwner bool) error {
	for _, t := range workflow {
		if t.From == from && t.To == to {
			if t.permits(role, isOwner) {
				return nil
			}
			return Forbidden(fmt.Sprintf("your role is not allowed to move a ticket from %q to %q", from, to))
		}
	}
	return InvalidTransition(from, to)
}

// AllowedTransitions lists the statuses the actor may move a ticket to right now.
// The API returns this so the UI never has to duplicate the workflow.
func AllowedTransitions(from Status, role Role, isOwner bool) []Status {
	out := []Status{}
	for _, t := range workflow {
		if t.From == from && t.permits(role, isOwner) {
			out = append(out, t.To)
		}
	}
	return out
}
