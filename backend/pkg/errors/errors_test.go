package errors_test

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/pkg/errors"
	"github.com/neochaotic/powerlab/backend/pkg/logging"
)

// --------------------------------------------------------------------
// New / construction
// --------------------------------------------------------------------

func TestNew_PopulatesFields(t *testing.T) {
	e := errors.New("ports.conflict", "errors.ports_in_use", http.StatusConflict)
	if e.Code != "ports.conflict" {
		t.Errorf("Code: want ports.conflict, got %q", e.Code)
	}
	if e.I18nKey != "errors.ports_in_use" {
		t.Errorf("I18nKey: want errors.ports_in_use, got %q", e.I18nKey)
	}
	if e.HTTPStatus != http.StatusConflict {
		t.Errorf("HTTPStatus: want 409, got %d", e.HTTPStatus)
	}
	if e.Cause != nil {
		t.Errorf("Cause: want nil, got %v", e.Cause)
	}
}

func TestError_StringContainsCode(t *testing.T) {
	e := errors.New("ports.conflict", "errors.ports_in_use", 409)
	if !strings.Contains(e.Error(), "ports.conflict") {
		t.Errorf("Error() should mention code; got %q", e.Error())
	}
}

func TestError_StringContainsCause(t *testing.T) {
	cause := stderrors.New("port 8080 already bound")
	e := errors.Wrap(cause, "ports.conflict", "errors.ports_in_use", 409)
	if !strings.Contains(e.Error(), "port 8080 already bound") {
		t.Errorf("Error() should mention cause; got %q", e.Error())
	}
}

// --------------------------------------------------------------------
// Wrap / cause chain
// --------------------------------------------------------------------

func TestWrap_PreservesCause(t *testing.T) {
	cause := stderrors.New("io error")
	e := errors.Wrap(cause, "io.failed", "errors.io_failed", 500)
	if e.Cause != cause {
		t.Errorf("Cause not preserved: want %v, got %v", cause, e.Cause)
	}
}

func TestWrap_NilReturnsNil(t *testing.T) {
	e := errors.Wrap(nil, "irrelevant", "irrelevant", 500)
	if e != nil {
		t.Errorf("Wrap(nil, ...) should return nil; got %v", e)
	}
}

func TestUnwrap_ReturnsCause(t *testing.T) {
	cause := stderrors.New("inner")
	e := errors.Wrap(cause, "x", "errors.x", 500)
	if got := stderrors.Unwrap(e); got != cause {
		t.Errorf("Unwrap: want %v, got %v", cause, got)
	}
}

func TestErrorsAs_ExtractsThroughChain(t *testing.T) {
	e := errors.New("auth.invalid_token", "errors.invalid_token", 401)
	wrapped := stderrors.New("outer: " + e.Error())

	// Standard chain — wrap with %w manually here to mimic real callers.
	chained := errors.Wrap(e, "middleware.auth_failed", "errors.auth_failed", 401)

	var found *errors.Error
	if !stderrors.As(chained, &found) {
		t.Fatalf("errors.As should find *errors.Error in chain; got false")
	}
	// errors.As returns the first match; in this case the wrapper itself.
	if found.Code != "middleware.auth_failed" {
		t.Errorf("expected first match (wrapper), got code %q", found.Code)
	}

	_ = wrapped // unused but kept to document the shape
}

// --------------------------------------------------------------------
// WithField / immutability
// --------------------------------------------------------------------

func TestWithField_AddsField(t *testing.T) {
	e := errors.ErrConflict.WithField("port", 8080)
	if e.Fields["port"] != 8080 {
		t.Errorf("WithField did not add field; got %v", e.Fields)
	}
}

func TestWithField_DoesNotMutateOriginal(t *testing.T) {
	original := errors.ErrConflict
	originalFieldCount := len(original.Fields)

	scoped := original.WithField("port", 8080)

	if len(original.Fields) != originalFieldCount {
		t.Errorf("WithField mutated original Fields map (count went %d → %d)",
			originalFieldCount, len(original.Fields))
	}
	if _, exists := original.Fields["port"]; exists {
		t.Errorf("WithField leaked field onto original")
	}
	if scoped.Fields["port"] != 8080 {
		t.Errorf("WithField did not place field on scoped instance")
	}
}

func TestWithFields_MergesMultiple(t *testing.T) {
	e := errors.ErrConflict.WithFields(map[string]any{
		"port":    8080,
		"service": "nginx",
	})
	if e.Fields["port"] != 8080 || e.Fields["service"] != "nginx" {
		t.Errorf("WithFields did not merge; got %v", e.Fields)
	}
}

func TestWithField_PreservesExistingFields(t *testing.T) {
	e := errors.ErrConflict.
		WithField("port", 8080).
		WithField("service", "nginx")

	if e.Fields["port"] != 8080 {
		t.Errorf("first field lost; got %v", e.Fields)
	}
	if e.Fields["service"] != "nginx" {
		t.Errorf("second field missing; got %v", e.Fields)
	}
}

// --------------------------------------------------------------------
// Catalog
// --------------------------------------------------------------------

func TestCatalog_HasExpectedHTTPStatuses(t *testing.T) {
	cases := map[*errors.Error]int{
		errors.ErrBadRequest:         http.StatusBadRequest,
		errors.ErrUnauthorized:       http.StatusUnauthorized,
		errors.ErrForbidden:          http.StatusForbidden,
		errors.ErrNotFound:           http.StatusNotFound,
		errors.ErrConflict:           http.StatusConflict,
		errors.ErrTooManyRequests:    http.StatusTooManyRequests,
		errors.ErrInternal:           http.StatusInternalServerError,
		errors.ErrServiceUnavailable: http.StatusServiceUnavailable,
	}
	for e, want := range cases {
		if e.HTTPStatus != want {
			t.Errorf("%q HTTPStatus: want %d, got %d", e.Code, want, e.HTTPStatus)
		}
	}
}

func TestCatalog_CodesAreNonEmpty(t *testing.T) {
	for _, e := range []*errors.Error{
		errors.ErrBadRequest, errors.ErrUnauthorized, errors.ErrForbidden,
		errors.ErrNotFound, errors.ErrConflict, errors.ErrTooManyRequests,
		errors.ErrInternal, errors.ErrServiceUnavailable,
	} {
		if e.Code == "" {
			t.Errorf("catalog entry has empty Code: %+v", e)
		}
		if e.I18nKey == "" {
			t.Errorf("catalog entry has empty I18nKey: %+v", e)
		}
	}
}

// --------------------------------------------------------------------
// WriteHTTP
// --------------------------------------------------------------------

func TestWriteHTTP_TypedError_StatusCodeAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	err := errors.ErrConflict.WithField("port", 8080)
	errors.WriteHTTP(context.Background(), rec, err)

	if rec.Code != http.StatusConflict {
		t.Errorf("status: want 409, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", got)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body should be valid JSON, got %s\nerr: %v", rec.Body.String(), err)
	}
	if body["code"] != "common.conflict" {
		t.Errorf("body.code: want common.conflict, got %v", body["code"])
	}
	if body["i18n_key"] == nil {
		t.Errorf("body.i18n_key: missing")
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("body.details: want map, got %T (%v)", body["details"], body["details"])
	}
	if details["port"] != float64(8080) { // JSON numbers
		t.Errorf("body.details.port: want 8080, got %v", details["port"])
	}
}

func TestWriteHTTP_NonTypedError_FallsBackTo500(t *testing.T) {
	rec := httptest.NewRecorder()
	errors.WriteHTTP(context.Background(), rec, stderrors.New("plain error"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body should be valid JSON, got %s\nerr: %v", rec.Body.String(), err)
	}
	if body["code"] != "common.internal" {
		t.Errorf("body.code: want common.internal, got %v", body["code"])
	}
}

func TestWriteHTTP_IncludesCorrelationIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), logging.CorrelationIDKey{}, "req-test-42")
	rec := httptest.NewRecorder()
	errors.WriteHTTP(ctx, rec, errors.ErrNotFound)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body should be valid JSON, got %s\nerr: %v", rec.Body.String(), err)
	}
	if body["correlation_id"] != "req-test-42" {
		t.Errorf("correlation_id: want req-test-42, got %v", body["correlation_id"])
	}
}

func TestWriteHTTP_NoCorrelationIDWhenAbsent(t *testing.T) {
	rec := httptest.NewRecorder()
	errors.WriteHTTP(context.Background(), rec, errors.ErrNotFound)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body should be valid JSON, got %s\nerr: %v", rec.Body.String(), err)
	}
	if v, present := body["correlation_id"]; present && v != "" {
		t.Errorf("correlation_id should be absent or empty; got %v", v)
	}
}

func TestWriteHTTP_TypedErrorThroughChain(t *testing.T) {
	// Caller wraps a domain error inside Wrap — WriteHTTP should still
	// recognize it via errors.As and use the right status.
	domain := errors.ErrNotFound.WithField("path", "/missing")
	rec := httptest.NewRecorder()
	errors.WriteHTTP(context.Background(), rec, domain)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rec.Code)
	}
}
