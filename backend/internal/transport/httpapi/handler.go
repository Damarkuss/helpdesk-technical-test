// Package httpapi is the HTTP transport: routing, middleware, JSON encoding
// and the mapping from domain errors to status codes. It holds no business rules.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/paska/helpdesk/api"
	"github.com/paska/helpdesk/internal/domain"
	"github.com/paska/helpdesk/internal/service"
)

// Deps are everything the HTTP layer needs.
type Deps struct {
	Auth        *service.AuthService
	Tickets     *service.TicketService
	Users       *service.UserService
	Logger      *slog.Logger
	CORSOrigins []string
	// Ping checks the database for /healthz. Optional.
	Ping func(ctx context.Context) error
}

// Handler holds the dependencies shared by all endpoints.
type Handler struct {
	auth    *service.AuthService
	tickets *service.TicketService
	users   *service.UserService
	log     *slog.Logger
	ping    func(ctx context.Context) error
}

// New builds the full http.Handler with routes and middleware.
func New(d Deps) http.Handler {
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	h := &Handler{auth: d.Auth, tickets: d.Tickets, users: d.Users, log: d.Logger, ping: d.Ping}

	staff := []domain.Role{domain.RoleAgent, domain.RoleAdmin}
	admin := []domain.Role{domain.RoleAdmin}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /api/openapi.yaml", serveSpec)
	mux.HandleFunc("GET /api/docs", serveDocs)
	mux.HandleFunc("GET /api/meta", h.meta)

	mux.HandleFunc("POST /api/auth/register", h.register)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.Handle("GET /api/me", h.secured(h.me))

	mux.Handle("GET /api/tickets", h.secured(h.listTickets))
	mux.Handle("POST /api/tickets", h.secured(h.createTicket))
	mux.Handle("GET /api/tickets/{id}", h.secured(h.getTicket))
	mux.Handle("PATCH /api/tickets/{id}", h.secured(h.updateTicket))
	mux.Handle("DELETE /api/tickets/{id}", h.secured(h.deleteTicket, admin...))
	mux.Handle("POST /api/tickets/{id}/comments", h.secured(h.addComment))

	mux.Handle("GET /api/agents", h.secured(h.listAssignable, staff...))
	mux.Handle("GET /api/users", h.secured(h.listUsers, admin...))
	mux.Handle("PATCH /api/users/{id}/role", h.secured(h.changeRole, admin...))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeProblem(w, http.StatusNotFound, "not_found", "no such endpoint", nil)
	})

	var root http.Handler = mux
	root = withCORS(d.CORSOrigins, root)
	root = withSecurityHeaders(root)
	root = h.withRecover(root)
	root = h.withRequestLog(root)
	return root
}

// ---- misc endpoints ---------------------------------------------------------

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if h.ping != nil {
		if err := h.ping(r.Context()); err != nil {
			h.log.Error("health check failed", "error", err)
			writeProblem(w, http.StatusServiceUnavailable, "unavailable", "database is not reachable", nil)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// meta exposes the allowed enum values so the UI does not hard-code them.
func (h *Handler) meta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": domain.Categories,
		"priorities": domain.Priorities,
		"statuses":   domain.Statuses,
		"roles":      domain.Roles,
	})
}

func serveSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(api.OpenAPISpec)
}

// serveDocs renders Swagger UI (loaded from a CDN) against the embedded spec.
func serveDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html><head><meta charset="utf-8"><title>Helpdesk API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head>
<body><div id="ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url: "/api/openapi.yaml", dom_id: "#ui"});</script>
</body></html>`))
}
