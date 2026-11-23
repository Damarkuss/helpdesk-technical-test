package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/testutil"
	"github.com/paska/helpdesk/internal/transport/httpapi"
)

// client drives the real router over httptest, like a browser would.
type client struct {
	t   *testing.T
	srv *httptest.Server
	app *testutil.App
}

func newClient(t *testing.T) *client {
	app := testutil.NewApp(t)
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		Auth: app.Auth, Tickets: app.Tickets, Users: app.Users,
	}))
	t.Cleanup(srv.Close)
	return &client{t: t, srv: srv, app: app}
}

type resp struct {
	Status int
	Body   map[string]any
}

func (r resp) errCode() string {
	if e, ok := r.Body["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

func (c *client) do(method, path, token string, body any) resp {
	c.t.Helper()
	var rd bytes.Reader
	if s, ok := body.(string); ok { // raw body, for malformed-JSON tests
		rd = *bytes.NewReader([]byte(s))
	} else if body != nil {
		b, _ := json.Marshal(body)
		rd = *bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.srv.URL+path, &rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	out := resp{Status: res.StatusCode, Body: map[string]any{}}
	_ = json.NewDecoder(res.Body).Decode(&out.Body)
	return out
}

// login creates a user directly in the store, then signs in through the API.
func (c *client) login(name string, role domain.Role) (token string, id int64) {
	c.t.Helper()
	u := c.app.NewUser(name, role)
	r := c.do("POST", "/api/auth/login", "", map[string]string{"email": u.Email, "password": testutil.Password})
	if r.Status != 200 {
		c.t.Fatalf("login %s: %d %v", name, r.Status, r.Body)
	}
	return r.Body["token"].(string), u.ID
}

func (c *client) createTicket(token, title string) int64 {
	c.t.Helper()
	r := c.do("POST", "/api/tickets", token, map[string]string{"title": title, "description": "Please help, it is broken."})
	if r.Status != 201 {
		c.t.Fatalf("create ticket: %d %v", r.Status, r.Body)
	}
	return int64(r.Body["id"].(float64))
}

func path(format string, a ...any) string { return fmt.Sprintf(format, a...) }

// ---- authentication ---------------------------------------------------------

func TestRegisterLoginAndMe(t *testing.T) {
	c := newClient(t)

	reg := c.do("POST", "/api/auth/register", "", map[string]string{"name": "Dewi", "email": "Dewi@Example.com", "password": "supersecret1"})
	if reg.Status != 201 {
		t.Fatalf("register: %d %v", reg.Status, reg.Body)
	}
	user := reg.Body["user"].(map[string]any)
	if user["role"] != "requester" || user["email"] != "dewi@example.com" {
		t.Errorf("registered user = %v (want requester, lowercased email)", user)
	}
	if _, leaked := user["password_hash"]; leaked {
		t.Error("password hash must never be serialised")
	}

	login := c.do("POST", "/api/auth/login", "", map[string]string{"email": "dewi@example.com", "password": "supersecret1"})
	if login.Status != 200 || login.Body["token"] == "" {
		t.Fatalf("login: %d %v", login.Status, login.Body)
	}
	me := c.do("GET", "/api/me", login.Body["token"].(string), nil)
	if me.Status != 200 || me.Body["name"] != "Dewi" {
		t.Errorf("me: %d %v", me.Status, me.Body)
	}
}

func TestAuthErrors(t *testing.T) {
	c := newClient(t)
	c.app.NewUser("known", domain.RoleRequester)

	cases := []struct {
		name   string
		r      resp
		status int
		code   string
	}{
		{"wrong password", c.do("POST", "/api/auth/login", "", map[string]string{"email": "known@example.com", "password": "nope-nope"}), 401, "unauthenticated"},
		{"unknown email gets the same answer", c.do("POST", "/api/auth/login", "", map[string]string{"email": "ghost@example.com", "password": "whatever1"}), 401, "unauthenticated"},
		{"no token", c.do("GET", "/api/me", "", nil), 401, "unauthenticated"},
		{"garbage token", c.do("GET", "/api/tickets", "not.a.token", nil), 401, "unauthenticated"},
		{"weak registration", c.do("POST", "/api/auth/register", "", map[string]string{"name": "A", "email": "bad", "password": "short"}), 422, "validation_error"},
		{"duplicate email", c.do("POST", "/api/auth/register", "", map[string]string{"name": "Known", "email": "KNOWN@example.com", "password": "longenough1"}), 409, "email_taken"},
		{"malformed JSON", c.do("POST", "/api/auth/login", "", `{"email": `), 400, "bad_request"},
		{"unknown field", c.do("POST", "/api/auth/register", "", map[string]string{"name": "Eve", "email": "eve@example.com", "password": "longenough1", "role": "admin"}), 400, "bad_request"},
	}
	for _, tc := range cases {
		if tc.r.Status != tc.status || tc.r.errCode() != tc.code {
			t.Errorf("%s: got %d/%q, want %d/%q (%v)", tc.name, tc.r.Status, tc.r.errCode(), tc.status, tc.code, tc.r.Body)
		}
	}

	// Validation errors carry per-field messages the UI can show.
	r := c.do("POST", "/api/auth/register", "", map[string]string{"name": "A", "email": "bad", "password": "short"})
	fields := r.Body["error"].(map[string]any)["fields"].(map[string]any)
	for _, f := range []string{"name", "email", "password"} {
		if fields[f] == nil {
			t.Errorf("missing field error for %q: %v", f, fields)
		}
	}
}

// ---- access control ---------------------------------------------------------

func TestRequesterCannotAccessAnotherUsersTicketOverHTTP(t *testing.T) {
	c := newClient(t)
	alice, _ := c.login("alice", domain.RoleRequester)
	bob, _ := c.login("bob", domain.RoleRequester)
	agent, _ := c.login("agent", domain.RoleAgent)
	id := c.createTicket(alice, "Alice's monitor")

	for _, tc := range []struct {
		method, p string
		body      any
	}{
		{"GET", path("/api/tickets/%d", id), nil},
		{"PATCH", path("/api/tickets/%d", id), map[string]string{"status": "closed"}},
		{"POST", path("/api/tickets/%d/comments", id), map[string]string{"body": "hello"}},
	} {
		r := c.do(tc.method, tc.p, bob, tc.body)
		if r.Status != 403 || r.errCode() != "forbidden" {
			t.Errorf("bob %s %s: %d %v, want 403 forbidden", tc.method, tc.p, r.Status, r.Body)
		}
	}

	// Bob's list does not contain Alice's ticket; the agent's does.
	if r := c.do("GET", "/api/tickets", bob, nil); r.Body["total"].(float64) != 0 {
		t.Errorf("bob's list: %v", r.Body)
	}
	if r := c.do("GET", "/api/tickets", agent, nil); r.Body["total"].(float64) != 1 {
		t.Errorf("agent's list: %v", r.Body)
	}

	// Alice herself is fine.
	if r := c.do("GET", path("/api/tickets/%d", id), alice, nil); r.Status != 200 {
		t.Errorf("alice: %d", r.Status)
	}
	// Unknown ticket and non-numeric id.
	if r := c.do("GET", "/api/tickets/9999", agent, nil); r.Status != 404 {
		t.Errorf("missing ticket: %d", r.Status)
	}
	if r := c.do("GET", "/api/tickets/abc", agent, nil); r.Status != 404 {
		t.Errorf("bad id: %d", r.Status)
	}
}

func TestAdminEndpointsRequireAdminRole(t *testing.T) {
	c := newClient(t)
	req, _ := c.login("req", domain.RoleRequester)
	agent, _ := c.login("agent", domain.RoleAgent)
	admin, adminID := c.login("admin", domain.RoleAdmin)
	_, reqID := c.login("req2", domain.RoleRequester)

	for name, tok := range map[string]string{"requester": req, "agent": agent} {
		if r := c.do("GET", "/api/users", tok, nil); r.Status != 403 {
			t.Errorf("%s GET /api/users = %d, want 403", name, r.Status)
		}
		if r := c.do("PATCH", path("/api/users/%d/role", reqID), tok, map[string]string{"role": "admin"}); r.Status != 403 {
			t.Errorf("%s PATCH role = %d, want 403", name, r.Status)
		}
	}
	if r := c.do("GET", "/api/users", admin, nil); r.Status != 200 || r.Body["total"].(float64) != 4 {
		t.Errorf("admin list: %d %v", r.Status, r.Body)
	}

	// Promote req2 to agent; the change applies on their very next request.
	r := c.do("PATCH", path("/api/users/%d/role", reqID), admin, map[string]string{"role": "agent"})
	if r.Status != 200 || r.Body["role"] != "agent" {
		t.Fatalf("promote: %d %v", r.Status, r.Body)
	}
	if r := c.do("PATCH", path("/api/users/%d/role", reqID), admin, map[string]string{"role": "superuser"}); r.Status != 422 {
		t.Errorf("invalid role = %d, want 422", r.Status)
	}
	// An admin cannot demote themselves (would risk locking everyone out).
	if r := c.do("PATCH", path("/api/users/%d/role", adminID), admin, map[string]string{"role": "requester"}); r.Status != 409 {
		t.Errorf("self-demotion = %d, want 409", r.Status)
	}
}

// ---- workflow over HTTP -----------------------------------------------------

func TestStatusTransitionsOverHTTP(t *testing.T) {
	c := newClient(t)
	alice, _ := c.login("alice", domain.RoleRequester)
	agent, agentID := c.login("agent", domain.RoleAgent)
	id := c.createTicket(alice, "Cannot log in")
	url := path("/api/tickets/%d", id)

	// Not in the workflow at all -> 422 invalid_transition.
	r := c.do("PATCH", url, agent, map[string]string{"status": "resolved"})
	if r.Status != 422 || r.errCode() != "invalid_transition" {
		t.Errorf("open->resolved: %d %v", r.Status, r.Body)
	}
	// In the workflow, but not for this role -> 403.
	r = c.do("PATCH", url, alice, map[string]string{"status": "in_progress"})
	if r.Status != 403 || r.errCode() != "forbidden" {
		t.Errorf("requester start: %d %v", r.Status, r.Body)
	}
	// Unknown status value -> 422 validation_error.
	r = c.do("PATCH", url, agent, map[string]string{"status": "done"})
	if r.Status != 422 || r.errCode() != "validation_error" {
		t.Errorf("unknown status: %d %v", r.Status, r.Body)
	}

	// Valid: agent starts work. Response lists what the agent may do next.
	r = c.do("PATCH", url, agent, map[string]string{"status": "in_progress"})
	if r.Status != 200 || r.Body["status"] != "in_progress" {
		t.Fatalf("start: %d %v", r.Status, r.Body)
	}
	if a := r.Body["assignee"].(map[string]any); int64(a["id"].(float64)) != agentID {
		t.Errorf("assignee = %v, want the agent", a)
	}
	next := r.Body["allowed_transitions"].([]any)
	if len(next) != 1 || next[0] != "resolved" {
		t.Errorf("allowed_transitions = %v", next)
	}

	c.do("PATCH", url, agent, map[string]string{"status": "resolved"})
	if r := c.do("PATCH", url, alice, map[string]string{"status": "closed"}); r.Status != 200 {
		t.Errorf("requester close: %d %v", r.Status, r.Body)
	}
	// Closed is final.
	if r := c.do("PATCH", url, agent, map[string]string{"status": "open"}); r.Status != 422 {
		t.Errorf("closed->open: %d", r.Status)
	}
	// Empty patch is rejected.
	if r := c.do("PATCH", url, agent, map[string]string{}); r.Status != 422 {
		t.Errorf("empty patch: %d", r.Status)
	}
}

func TestInternalNotesOverHTTP(t *testing.T) {
	c := newClient(t)
	alice, _ := c.login("alice", domain.RoleRequester)
	agent, _ := c.login("agent", domain.RoleAgent)
	id := c.createTicket(alice, "Account locked")
	url := path("/api/tickets/%d/comments", id)

	if r := c.do("POST", url, agent, map[string]any{"body": "Customer is VIP, reset manually", "is_internal": true}); r.Status != 201 {
		t.Fatalf("internal note: %d %v", r.Status, r.Body)
	}
	if r := c.do("POST", url, agent, map[string]any{"body": "We are looking into it"}); r.Status != 201 {
		t.Fatalf("public comment: %d", r.Status)
	}
	if r := c.do("POST", url, alice, map[string]any{"body": "x", "is_internal": true}); r.Status != 403 {
		t.Errorf("requester internal note: %d, want 403", r.Status)
	}
	if r := c.do("POST", url, alice, map[string]any{"body": "   "}); r.Status != 422 {
		t.Errorf("blank comment: %d, want 422", r.Status)
	}

	count := func(tok string) int {
		r := c.do("GET", path("/api/tickets/%d", id), tok, nil)
		return len(r.Body["comments"].([]any))
	}
	if n := count(alice); n != 1 {
		t.Errorf("requester sees %d comments, want 1", n)
	}
	if n := count(agent); n != 2 {
		t.Errorf("agent sees %d comments, want 2", n)
	}
}

func TestListQueryParameters(t *testing.T) {
	c := newClient(t)
	agent, _ := c.login("agent", domain.RoleAgent)
	alice, _ := c.login("alice", domain.RoleRequester)
	for i := 0; i < 25; i++ {
		c.createTicket(alice, fmt.Sprintf("Ticket number %02d", i))
	}

	r := c.do("GET", "/api/tickets?page=2&page_size=10&sort=title&order=asc", agent, nil)
	if r.Status != 200 || r.Body["total"].(float64) != 25 || r.Body["total_pages"].(float64) != 3 {
		t.Fatalf("page 2: %d %v", r.Status, r.Body)
	}
	items := r.Body["data"].([]any)
	if len(items) != 10 || items[0].(map[string]any)["title"] != "Ticket number 10" {
		t.Errorf("page 2 items wrong, first = %v", items[0])
	}

	if r := c.do("GET", "/api/tickets?q=number%2007", agent, nil); r.Body["total"].(float64) != 1 {
		t.Errorf("search: %v", r.Body)
	}
	if r := c.do("GET", "/api/tickets?page=abc", agent, nil); r.Status != 422 || r.errCode() != "validation_error" {
		t.Errorf("bad page: %d %v", r.Status, r.Body)
	}
	if r := c.do("GET", "/api/tickets?sort=drop_table", agent, nil); r.Status != 422 {
		t.Errorf("bad sort: %d", r.Status)
	}
}

func TestUnknownRouteAndHealth(t *testing.T) {
	c := newClient(t)
	if r := c.do("GET", "/healthz", "", nil); r.Status != 200 {
		t.Errorf("healthz: %d", r.Status)
	}
	if r := c.do("GET", "/api/nope", "", nil); r.Status != 404 || r.errCode() != "not_found" {
		t.Errorf("unknown route: %d %v", r.Status, r.Body)
	}
}
