package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/paska/helpdesk/internal/domain"
)

type ctxKey struct{}

func userFrom(ctx context.Context) *domain.User {
	u, _ := ctx.Value(ctxKey{}).(*domain.User)
	return u
}

// requireAuth checks the bearer token and puts the current user in the request context.
func (h *Handler) requireAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			h.writeError(w, r, domain.Unauthenticated("missing or malformed Authorization header"))
			return
		}
		u, err := h.auth.Authenticate(r.Context(), strings.TrimSpace(token))
		if err != nil {
			h.writeError(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

// requireRole must be used inside requireAuth. It rejects users whose role is not listed.
func (h *Handler) requireRole(roles ...domain.Role) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u := userFrom(r.Context())
			if u == nil || !slices.Contains(roles, u.Role) {
				h.writeError(w, r, domain.Forbidden("your role does not have access to this resource"))
				return
			}
			next(w, r)
		}
	}
}

// secured chains authentication and an optional role check.
func (h *Handler) secured(next http.HandlerFunc, roles ...domain.Role) http.Handler {
	if len(roles) > 0 {
		next = h.requireRole(roles...)(next)
	}
	return h.requireAuth(next)
}

// ---- global middleware ------------------------------------------------------

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (h *Handler) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		h.log.Info("request", "id", id, "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (h *Handler) withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				h.log.Error("panic", "panic", rec, "stack", string(debug.Stack()))
				writeProblem(w, http.StatusInternalServerError, "internal_error", "something went wrong on our side", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// withCORS allows browsers on the listed origins to call the API directly.
// In docker-compose the frontend proxies /api so CORS is not needed there.
func withCORS(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (slices.Contains(origins, "*") || slices.Contains(origins, origin)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "600")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
