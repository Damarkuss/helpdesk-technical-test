package domain_test

import (
	"errors"
	"testing"

	"github.com/paska/helpdesk/internal/domain"
)

// brief is the transition table from the task description, written out
// independently of the implementation so the test is not a mirror of the code.
// Each edge lists who may take it: "agent", "admin" and/or "owner".
var brief = map[[2]domain.Status][]string{
	{domain.StatusOpen, domain.StatusInProgress}:     {"agent", "admin"},
	{domain.StatusInProgress, domain.StatusResolved}: {"agent", "admin"},
	{domain.StatusResolved, domain.StatusClosed}:     {"owner", "agent", "admin"},
	{domain.StatusResolved, domain.StatusOpen}:       {"owner"},
	{domain.StatusOpen, domain.StatusClosed}:         {"owner", "admin"},
}

func allowedByBrief(from, to domain.Status, role domain.Role, owner bool) (edgeExists, permitted bool) {
	who, ok := brief[[2]domain.Status{from, to}]
	if !ok {
		return false, false
	}
	for _, w := range who {
		if (w == "owner" && owner) || w == string(role) {
			return true, true
		}
	}
	return true, false
}

// TestWorkflowMatrix checks every combination of from/to status, role and
// ownership against the brief: 16 status pairs x 3 roles x 2 ownership = 96 cases.
func TestWorkflowMatrix(t *testing.T) {
	for _, from := range domain.Statuses {
		for _, to := range domain.Statuses {
			for _, role := range domain.Roles {
				for _, owner := range []bool{false, true} {
					exists, permitted := allowedByBrief(from, to, role, owner)
					err := domain.CheckTransition(from, to, role, owner)

					var de *domain.Error
					switch {
					case permitted:
						if err != nil {
							t.Errorf("%s->%s role=%s owner=%v: want allowed, got %v", from, to, role, owner, err)
						}
					case exists:
						if !errors.As(err, &de) || de.Kind != domain.KindForbidden {
							t.Errorf("%s->%s role=%s owner=%v: want forbidden, got %v", from, to, role, owner, err)
						}
					default:
						if !errors.As(err, &de) || de.Kind != domain.KindInvalidTransition {
							t.Errorf("%s->%s role=%s owner=%v: want invalid_transition, got %v", from, to, role, owner, err)
						}
					}
				}
			}
		}
	}
}

// AllowedTransitions (used by the UI to draw buttons) must agree with CheckTransition.
func TestAllowedTransitionsAgreeWithCheck(t *testing.T) {
	for _, from := range domain.Statuses {
		for _, role := range domain.Roles {
			for _, owner := range []bool{false, true} {
				allowed := map[domain.Status]bool{}
				for _, to := range domain.AllowedTransitions(from, role, owner) {
					allowed[to] = true
				}
				for _, to := range domain.Statuses {
					ok := domain.CheckTransition(from, to, role, owner) == nil
					if ok != allowed[to] {
						t.Errorf("%s->%s role=%s owner=%v: CheckTransition=%v but AllowedTransitions=%v", from, to, role, owner, ok, allowed[to])
					}
				}
			}
		}
	}
}

func TestClosedIsTerminal(t *testing.T) {
	for _, role := range domain.Roles {
		if got := domain.AllowedTransitions(domain.StatusClosed, role, true); len(got) != 0 {
			t.Errorf("role %s can leave closed: %v", role, got)
		}
	}
}
