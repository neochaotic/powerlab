package jwt_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
)

// Regression lock for issue #342 — backend must accept RFC 6750
// `Authorization: Bearer <token>` form in addition to the legacy
// raw-token form. Before this fix, sending the standard Bearer
// prefix caused the JWT validator to receive the literal string
// "Bearer abc..." and reject it as malformed, producing 401.
//
// The function under test (jwt.ExtractTokenFromRequest) is the
// single source of truth for header → token extraction. All 6
// JWT middleware sites in the repo (gateway, app-management,
// message-bus, core, user-service, common) now share it.
func TestExtractTokenFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		query    string
		expected string
	}{
		{"raw token (legacy)", "abc.def.ghi", "", "abc.def.ghi"},
		{"RFC 6750 Bearer prefix", "Bearer abc.def.ghi", "", "abc.def.ghi"},
		{"RFC 6750 lowercase bearer", "bearer abc.def.ghi", "", "abc.def.ghi"},
		{"RFC 6750 uppercase BEARER", "BEARER abc.def.ghi", "", "abc.def.ghi"},
		{"RFC 6750 mixed-case", "BeArEr abc.def.ghi", "", "abc.def.ghi"},
		{"empty header falls back to query", "", "abc.def.ghi", "abc.def.ghi"},
		{"empty everything", "", "", ""},
		{"header wins over query", "header.tok", "query.tok", "header.tok"},
		{"Bearer-only no token", "Bearer ", "", ""},
		{"Bearer with multiple spaces — only one stripped", "Bearer  abc", "", " abc"},
		{"header without space after Bearer is raw (not stripped)", "Bearerabc", "", "Bearerabc"},
		{"token containing the word Bearer", "Bearer Bearer.is.in.token", "", "Bearer.is.in.token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			path := "/"
			if tt.query != "" {
				path = "/?token=" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if tt.header != "" {
				req.Header.Set(echo.HeaderAuthorization, tt.header)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			got := jwt.ExtractTokenFromRequest(c)
			if got != tt.expected {
				t.Errorf("ExtractTokenFromRequest = %q, want %q", got, tt.expected)
			}
		})
	}
}

// #35 — the HttpOnly access_token cookie lets browser GETs (media,
// downloads) authenticate without the JWT in the URL. Precedence:
// Authorization header > access_token cookie > ?token= query.
func TestExtractTokenFromRequest_Cookie(t *testing.T) {
	newCtx := func(header, cookie, query string) echo.Context {
		e := echo.New()
		path := "/"
		if query != "" {
			path = "/?token=" + query
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if header != "" {
			req.Header.Set(echo.HeaderAuthorization, header)
		}
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "access_token", Value: cookie})
		}
		return e.NewContext(req, httptest.NewRecorder())
	}

	if got := jwt.ExtractTokenFromRequest(newCtx("", "cookie.tok", "")); got != "cookie.tok" {
		t.Errorf("cookie should be used when no header: got %q", got)
	}
	if got := jwt.ExtractTokenFromRequest(newCtx("header.tok", "cookie.tok", "query.tok")); got != "header.tok" {
		t.Errorf("Authorization header must win over cookie: got %q", got)
	}
	if got := jwt.ExtractTokenFromRequest(newCtx("", "cookie.tok", "query.tok")); got != "cookie.tok" {
		t.Errorf("cookie must win over ?token= query: got %q", got)
	}
	if got := jwt.ExtractTokenFromRequest(newCtx("", "", "query.tok")); got != "query.tok" {
		t.Errorf("query is the last-resort fallback: got %q", got)
	}
}
