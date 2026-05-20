package route

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestProtectedRoute_RequiresAuth locks in that the JWT middleware is
// active for non-loopback callers on the protected /v1/users group.
// Before the echo-jwt/v4 migration this package couldn't compile
// (echo v4.13.x removed JWTWithConfig), so this also guards the build
// fix. A request with no token from a non-loopback IP must be rejected
// at the middleware (401) before reaching any handler.
func TestProtectedRoute_RequiresAuth(t *testing.T) {
	router := InitRouter()

	for _, path := range []string{"/v1/users/current"} {
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "192.168.1.100:12345" // non-loopback → JWT must fire
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: want 401, got %d", path, w.Code)
		}
	}
}
