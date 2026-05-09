package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/neochaotic/powerlab/backend/pkg/logging"
)

// Error is PowerLab's typed error.
//
// It carries enough metadata for the gateway to render a stable JSON
// response, the UI to translate the message, the logger to attach
// structured attributes, and standard library errors.Is / errors.As to
// reach into the chain.
type Error struct {
	// Code is a stable, machine-readable identifier
	// (e.g. "ports.conflict"). Used as a search key in logs and a
	// dispatch key in the UI.
	Code string

	// I18nKey is the translation key the UI uses to render the
	// user-facing message (e.g. "errors.ports_in_use"). Kept separate
	// from Code so wording can evolve without breaking log searches.
	I18nKey string

	// HTTPStatus is the HTTP status the handler emits when this error
	// reaches the response.
	HTTPStatus int

	// Cause is the underlying error, preserved through the chain.
	// errors.Is and errors.As reach this via Unwrap.
	Cause error

	// Fields is structured incident detail (e.g. {"port": 8080}).
	// Surfaced as a "details" object on the HTTP response and as
	// structured attributes in logs.
	Fields map[string]any
}

// Error implements the standard error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Cause.Error())
	}
	return e.Code
}

// Unwrap returns the underlying cause so errors.Is and errors.As reach
// it through the chain.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// WithField returns a new *Error with the given field added. The
// receiver is not mutated; the catalog entries (ErrConflict, etc.) stay
// pristine across concurrent callers.
func (e *Error) WithField(key string, value any) *Error {
	return e.WithFields(map[string]any{key: value})
}

// WithFields returns a new *Error with all given fields merged in. The
// receiver is not mutated.
func (e *Error) WithFields(fields map[string]any) *Error {
	if e == nil {
		return nil
	}
	merged := make(map[string]any, len(e.Fields)+len(fields))
	for k, v := range e.Fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &Error{
		Code:       e.Code,
		I18nKey:    e.I18nKey,
		HTTPStatus: e.HTTPStatus,
		Cause:      e.Cause,
		Fields:     merged,
	}
}

// New constructs a fresh *Error.
func New(code, i18nKey string, status int) *Error {
	return &Error{
		Code:       code,
		I18nKey:    i18nKey,
		HTTPStatus: status,
	}
}

// Wrap attaches typed metadata to an existing error, preserving the
// chain. Returns nil if err is nil — callers can use it inline:
//
//	return errors.Wrap(doSomething(), "io.failed", "errors.io_failed", 500)
func Wrap(err error, code, i18nKey string, status int) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:       code,
		I18nKey:    i18nKey,
		HTTPStatus: status,
		Cause:      err,
	}
}

// Catalog: universal HTTP-tier errors. Domain-specific errors live with
// the domain (e.g. ports.conflict belongs in the compose package, not
// here). Keep this list small and stable.
var (
	ErrBadRequest         = New("common.bad_request", "errors.bad_request", http.StatusBadRequest)
	ErrUnauthorized       = New("common.unauthorized", "errors.unauthorized", http.StatusUnauthorized)
	ErrForbidden          = New("common.forbidden", "errors.forbidden", http.StatusForbidden)
	ErrNotFound           = New("common.not_found", "errors.not_found", http.StatusNotFound)
	ErrConflict           = New("common.conflict", "errors.conflict", http.StatusConflict)
	ErrTooManyRequests    = New("common.too_many_requests", "errors.too_many_requests", http.StatusTooManyRequests)
	ErrInternal           = New("common.internal", "errors.internal", http.StatusInternalServerError)
	ErrServiceUnavailable = New("common.service_unavailable", "errors.service_unavailable", http.StatusServiceUnavailable)
)

// httpBody is the shape every error response takes. Stable contract;
// the UI client deserializes against it.
type httpBody struct {
	Code          string         `json:"code"`
	I18nKey       string         `json:"i18n_key"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
}

// WriteHTTP writes err to w as a JSON response with the right status,
// Content-Type, and body shape.
//
// If err is not (or does not wrap) an *Error, it is treated as
// ErrInternal and the original is preserved by the standard library
// error chain so callers can still see the underlying cause via
// errors.Unwrap. The handler always emits a valid JSON body — closes
// the class of bug where a raw http.Error left the user staring at a
// plain-text error page (#50).
func WriteHTTP(ctx context.Context, w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	var typed *Error
	// Try to extract through the chain. asTyped lets us treat any
	// stdlib-wrapped *Error consistently.
	if !asTyped(err, &typed) {
		typed = ErrInternal
	}

	body := httpBody{
		Code:    typed.Code,
		I18nKey: typed.I18nKey,
		Details: typed.Fields,
	}
	if id, ok := ctx.Value(logging.CorrelationIDKey{}).(string); ok && id != "" {
		body.CorrelationID = id
	}

	w.WriteHeader(typed.HTTPStatus)
	_ = json.NewEncoder(w).Encode(body)
}

// asTyped is a small wrapper around errors.As that returns a bool so
// WriteHTTP reads naturally.
func asTyped(err error, target **Error) bool {
	for cur := err; cur != nil; {
		if e, ok := cur.(*Error); ok {
			*target = e
			return true
		}
		cur = unwrap(cur)
	}
	return false
}

// unwrap is a tiny shim over errors.Unwrap to keep this file's import
// list short. Could call errors.Unwrap from "errors" stdlib instead;
// using a method-direct call avoids the alias collision with our own
// package name.
func unwrap(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}
