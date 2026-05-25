package utils

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// DefaultQuery returns the query param when present, else the default.
func TestDefaultQuery(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/?present=value", nil)
	ctx := e.NewContext(req, httptest.NewRecorder())

	if got := DefaultQuery(ctx, "present", "fallback"); got != "value" {
		t.Errorf("present param: got %q, want %q", got, "value")
	}
	if got := DefaultQuery(ctx, "absent", "fallback"); got != "fallback" {
		t.Errorf("absent param: got %q, want %q", got, "fallback")
	}
}

// DefaultPostForm returns the posted form value when present, else the default.
func TestDefaultPostForm(t *testing.T) {
	e := echo.New()
	form := url.Values{"present": {"value"}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	// DefaultPostForm reads Request().Form directly, which is nil until parsed.
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}
	ctx := e.NewContext(req, httptest.NewRecorder())

	if got := DefaultPostForm(ctx, "present", "fallback"); got != "value" {
		t.Errorf("present field: got %q, want %q", got, "value")
	}
	if got := DefaultPostForm(ctx, "absent", "fallback"); got != "fallback" {
		t.Errorf("absent field: got %q, want %q", got, "fallback")
	}
}
