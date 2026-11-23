package domain

import "fmt"

// Kind classifies a domain error so the transport layer can pick an HTTP status
// without the domain knowing anything about HTTP.
type Kind int

const (
	KindValidation Kind = iota + 1
	KindUnauthenticated
	KindForbidden
	KindNotFound
	KindConflict
	KindInvalidTransition
)

// Error is the single error type returned by the business layer.
type Error struct {
	Kind    Kind
	Code    string            // stable machine-readable code, e.g. "invalid_transition"
	Message string            // human-readable explanation
	Fields  map[string]string // per-field messages for validation errors
}

func (e *Error) Error() string { return e.Message }

// Validation builds a validation error from per-field messages.
func Validation(fields map[string]string) *Error {
	return &Error{Kind: KindValidation, Code: "validation_error", Message: "one or more fields are invalid", Fields: fields}
}

// FieldError is a shortcut for a validation error on a single field.
func FieldError(field, msg string) *Error { return Validation(map[string]string{field: msg}) }

func Unauthenticated(msg string) *Error {
	return &Error{Kind: KindUnauthenticated, Code: "unauthenticated", Message: msg}
}

func Forbidden(msg string) *Error {
	return &Error{Kind: KindForbidden, Code: "forbidden", Message: msg}
}

func NotFound(what string) *Error {
	return &Error{Kind: KindNotFound, Code: "not_found", Message: what + " not found"}
}

func Conflict(code, msg string) *Error {
	return &Error{Kind: KindConflict, Code: code, Message: msg}
}

// InvalidTransition is returned when a status change is not part of the workflow at all.
func InvalidTransition(from, to Status) *Error {
	return &Error{
		Kind:    KindInvalidTransition,
		Code:    "invalid_transition",
		Message: fmt.Sprintf("a ticket cannot move from %q to %q", from, to),
	}
}
