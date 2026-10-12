// Package apperr carries application errors across layers without importing any
// HTTP framework. The HTTP boundary maps Kind to a status code and renders the
// response.
package apperr

// Kind classifies an error by intent. The HTTP boundary maps it to a status.
type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindNotFound
	KindConflict
	KindUnauthorized
	KindForbidden
)

// Error is an application error. Message is safe to show a client; the wrapped
// cause is not.
type Error struct {
	Kind    Kind
	Code    string // stable, machine-readable, e.g. "insufficient_balance"
	Message string
	Details any // optional payload rendered into the response
	err     error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.err }

// New builds an error without a cause.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Wrap builds an error from a cause. The cause is logged, never returned.
func Wrap(err error, kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message, err: err}
}

// WithDetails attaches a structured payload, e.g. []FieldError for validation.
func (e *Error) WithDetails(details any) *Error {
	e.Details = details
	return e
}

// Constructors for the common kinds.
func Internal(code, message string) *Error     { return New(KindInternal, code, message) }
func Invalid(code, message string) *Error      { return New(KindInvalid, code, message) }
func NotFound(code, message string) *Error     { return New(KindNotFound, code, message) }
func Conflict(code, message string) *Error     { return New(KindConflict, code, message) }
func Unauthorized(code, message string) *Error { return New(KindUnauthorized, code, message) }
func Forbidden(code, message string) *Error    { return New(KindForbidden, code, message) }

// FieldError is one validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validation builds an Invalid error describing which fields failed.
func Validation(fields ...FieldError) *Error {
	return &Error{
		Kind:    KindInvalid,
		Code:    "validation_failed",
		Message: "request validation failed",
		Details: fields,
	}
}
