package httpapi

import (
	"net/http"
	"strconv"

	"github.com/paska/helpdesk/internal/service"
)

// ---- auth -------------------------------------------------------------------

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in service.RegisterInput
	if !h.decodeJSON(w, r, &in) {
		return
	}
	res, err := h.auth.Register(r.Context(), in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in service.LoginInput
	if !h.decodeJSON(w, r, &in) {
		return
	}
	res, err := h.auth.Login(r.Context(), in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userFrom(r.Context()))
}

// ---- tickets ----------------------------------------------------------------

func (h *Handler) listTickets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	in := service.ListTicketsInput{
		Status: q.Get("status"), Priority: q.Get("priority"), Assignee: q.Get("assignee"),
		Query: q.Get("q"), Sort: q.Get("sort"), Order: q.Get("order"),
	}
	var err error
	if in.Page, err = queryInt(r, "page"); err != nil {
		h.writeError(w, r, err)
		return
	}
	if in.PageSize, err = queryInt(r, "page_size"); err != nil {
		h.writeError(w, r, err)
		return
	}
	res, err := h.tickets.List(r.Context(), userFrom(r.Context()), in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) createTicket(w http.ResponseWriter, r *http.Request) {
	var in service.CreateTicketInput
	if !h.decodeJSON(w, r, &in) {
		return
	}
	t, err := h.tickets.Create(r.Context(), userFrom(r.Context()), in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/tickets/"+strconv.FormatInt(t.ID, 10))
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) getTicket(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	t, err := h.tickets.Get(r.Context(), userFrom(r.Context()), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) updateTicket(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var in service.UpdateTicketInput
	if !h.decodeJSON(w, r, &in) {
		return
	}
	t, err := h.tickets.Update(r.Context(), userFrom(r.Context()), id, in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) deleteTicket(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.tickets.Delete(r.Context(), userFrom(r.Context()), id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) addComment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var in service.AddCommentInput
	if !h.decodeJSON(w, r, &in) {
		return
	}
	c, err := h.tickets.AddComment(r.Context(), userFrom(r.Context()), id, in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// ---- users ------------------------------------------------------------------

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	in := service.ListUsersInput{Query: q.Get("q"), Role: q.Get("role")}
	var err error
	if in.Page, err = queryInt(r, "page"); err != nil {
		h.writeError(w, r, err)
		return
	}
	if in.PageSize, err = queryInt(r, "page_size"); err != nil {
		h.writeError(w, r, err)
		return
	}
	res, err := h.users.List(r.Context(), userFrom(r.Context()), in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) listAssignable(w http.ResponseWriter, r *http.Request) {
	refs, err := h.users.Assignable(r.Context(), userFrom(r.Context()))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": refs})
}

func (h *Handler) changeRole(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if !h.decodeJSON(w, r, &in) {
		return
	}
	u, err := h.users.ChangeRole(r.Context(), userFrom(r.Context()), id, in.Role)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}
