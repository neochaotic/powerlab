package middleware_test

import (
	"reflect"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/middleware"
)

// Locks the shared CORS config to the BYTE-IDENTICAL shape every
// PowerLab service had inline before Sprint 20 PR 5 deduplicated
// them. Any drift in the assertions below is a wire-level change
// that needs an explicit decision (the v0.6.x line is internal-
// network-only per ADR-0007; permissive CORS is intentional).
//
// Memory feedback_no_apagar_test_para_passar: any disagreement
// here is a real CORS behaviour change, not a test to weaken.

func TestCORSConfig_MatchesHistoricalInline(t *testing.T) {
	cfg := middleware.CORSConfig()

	wantAllowOrigins := []string{"*"}
	if !reflect.DeepEqual(cfg.AllowOrigins, wantAllowOrigins) {
		t.Errorf("AllowOrigins: got %v, want %v", cfg.AllowOrigins, wantAllowOrigins)
	}

	wantAllowMethods := []string{echo.POST, echo.GET, echo.OPTIONS, echo.PUT, echo.DELETE}
	if !reflect.DeepEqual(cfg.AllowMethods, wantAllowMethods) {
		t.Errorf("AllowMethods: got %v, want %v", cfg.AllowMethods, wantAllowMethods)
	}

	wantAllowHeaders := []string{
		echo.HeaderAuthorization,
		echo.HeaderContentLength,
		echo.HeaderXCSRFToken,
		echo.HeaderContentType,
		echo.HeaderAccessControlAllowOrigin,
		echo.HeaderAccessControlAllowHeaders,
		echo.HeaderAccessControlAllowMethods,
		echo.HeaderConnection,
		echo.HeaderOrigin,
		echo.HeaderXRequestedWith,
	}
	if !reflect.DeepEqual(cfg.AllowHeaders, wantAllowHeaders) {
		t.Errorf("AllowHeaders: got %v, want %v", cfg.AllowHeaders, wantAllowHeaders)
	}

	wantExposeHeaders := []string{
		echo.HeaderContentLength,
		echo.HeaderAccessControlAllowOrigin,
		echo.HeaderAccessControlAllowHeaders,
	}
	if !reflect.DeepEqual(cfg.ExposeHeaders, wantExposeHeaders) {
		t.Errorf("ExposeHeaders: got %v, want %v", cfg.ExposeHeaders, wantExposeHeaders)
	}

	if cfg.MaxAge != 172800 {
		t.Errorf("MaxAge: got %d, want 172800", cfg.MaxAge)
	}

	if !cfg.AllowCredentials {
		t.Error("AllowCredentials: got false, want true")
	}
}

func TestCors_ReturnsNonNilMiddleware(t *testing.T) {
	if middleware.Cors() == nil {
		t.Error("Cors() returned nil — must return a usable echo.MiddlewareFunc")
	}
}
