// Package middleware holds echo middleware shared across PowerLab
// backend services. Today this is only the CORS helper — sized as a
// package, not a single file, so future shared middleware (e.g.
// rate-limit, request-id propagation) can land alongside without
// re-shuffling imports.
package middleware

import (
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
)

// CORSConfig returns the permissive CORS config used by every
// PowerLab backend service. Internal-network-only deployment per
// ADR-0007 makes AllowOrigins: "*" + AllowCredentials: true the
// deliberate choice — operators reach the panel from any LAN host
// and the JWT gate is the actual auth surface.
//
// Exported separately from Cors() so tests can inspect the config
// without instantiating the middleware function, and so a future
// caller that needs to override one field (e.g. add a header to
// AllowHeaders) can do so without re-deriving the whole shape.
//
// Sprint 20 PR 5 deduplicated 9 byte-identical inline definitions
// across app-management, gateway, core, user-service, message-bus,
// local-storage. The byte-equal lock lives in cors_test.go.
func CORSConfig() echo_middleware.CORSConfig {
	return echo_middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			echo.POST, echo.GET, echo.OPTIONS, echo.PUT, echo.DELETE,
		},
		AllowHeaders: []string{
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
		},
		ExposeHeaders: []string{
			echo.HeaderContentLength,
			echo.HeaderAccessControlAllowOrigin,
			echo.HeaderAccessControlAllowHeaders,
		},
		MaxAge:           172800,
		AllowCredentials: true,
	}
}

// Cors returns the shared CORS middleware, ready to drop into an
// Echo group's Use() chain. Equivalent to
// `echo_middleware.CORSWithConfig(CORSConfig())` — convenient for
// the 9 services that just want the standard config.
func Cors() echo.MiddlewareFunc {
	return echo_middleware.CORSWithConfig(CORSConfig())
}
