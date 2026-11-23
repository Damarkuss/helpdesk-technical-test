package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/paska/helpdesk/internal/domain"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeProblem(w http.ResponseWriter, status int, code, msg string, fields map[string]string) {
	writeJSON(w, status, errorBody{Error: errorPayload{Code: code, Message: msg, Fields: fields}})
}

// httpStatus maps a domain error kind to an HTTP status code.
//
//	validation          -> 422 Unprocessable Entity
//	unauthenticated     -> 401 Unauthorized
//	forbidden           -> 403 Forbidden
//	not found           -> 404 Not Found
//	conflict            -> 409 Conflict
//	invalid transition  -> 422 Unprocessable Entity (code "invalid_transition")
func httpStatus(k domain.Kind) int {
	switch k {
	case domain.KindValidation, domain.KindInvalidTransition:
		return http.StatusUnprocessableEntity
	case domain.KindUnauthenticated:
		return http.StatusUnauthorized
	case domain.KindForbidden:
		return http.StatusForbidden
	case domain.KindNotFound:
		return http.StatusNotFound
	case domain.KindConflict:
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// writeError turns any error into a JSON response. Unexpected errors are
// logged in full but only a generic message is sent to the client.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var de *domain.Error
	if errors.As(err, &de) {
		if de.Kind == domain.KindUnauthenticated {
			w.Header().Set("WWW-Authenticate", `Bearer realm="helpdesk"`)
		}
		writeProblem(w, httpStatus(de.Kind), de.Code, de.Message, de.Fields)
		return
	}
	h.log.Error("unhandled error", "method", r.Method, "path", r.URL.Path, "request_id", w.Header().Get("X-Request-ID"), "error", err)
	writeProblem(w, http.StatusInternalServerError, "internal_error", "something went wrong on our side", nil)
}

// decodeJSON reads a single JSON object from the body, rejecting unknown
// fields, oversized bodies and trailing data.
func (h *Handler) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err == nil && dec.More() {
		err = errors.New("body must contain a single JSON object")
	}
	if err == nil {
		return true
	}

	var (
		syntax *json.SyntaxError
		typeEr *json.UnmarshalTypeError
		tooBig *http.MaxBytesError
	)
	switch {
	case errors.As(err, &tooBig):
		writeProblem(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large", nil)
	case errors.Is(err, io.EOF):
		writeProblem(w, http.StatusBadRequest, "bad_request", "request body is empty", nil)
	case errors.As(err, &syntax), errors.Is(err, io.ErrUnexpectedEOF):
		writeProblem(w, http.StatusBadRequest, "bad_request", "request body is not valid JSON", nil)
	case errors.As(err, &typeEr):
		writeProblem(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("field %q has the wrong type", typeEr.Field), nil)
	default:
		writeProblem(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
	}
	return false
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NotFound("resource")
	}
	return id, nil
}

// queryInt reads an optional integer query parameter.
func queryInt(r *http.Request, name string) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, domain.FieldError(name, "must be an integer")
	}
	return n, nil
}
