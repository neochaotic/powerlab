package route_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/app-management/codegen"
)

// Regression for #32: in v0.1.x the UI sent PUT /compose/{id}/status
// with a JSON object ({"status":"stop"}) while the OpenAPI contract
// declares the body as a bare JSON string ("stop"). The request
// validator rejected it with 400 before the handler ran, the store
// swallowed the error and the UI looked frozen. These tests pin the
// contract at the validator layer: bare strings pass, objects don't.
//
// The echo instance mirrors the validator wiring in route/v2.go
// (OapiRequestValidatorWithOptions over codegen.GetSwagger() with a
// no-op auth func) and puts a stub handler behind it, so no Docker or
// app-management service is needed.
func newComposeStatusValidatorServer(t *testing.T, reached *bool) *echo.Echo {
	t.Helper()
	swagger, err := codegen.GetSwagger()
	if err != nil {
		t.Fatalf("load swagger: %v", err)
	}
	e := echo.New()
	e.Use(middleware.OapiRequestValidatorWithOptions(swagger, &middleware.Options{
		Options: openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}))
	e.PUT("/v2/app_management/compose/:id/status", func(c echo.Context) error {
		*reached = true
		return c.NoContent(http.StatusOK)
	})
	return e
}

func putComposeStatus(e *echo.Echo, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/v2/app_management/compose/x/status", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestComposeAppStatus_BareStringBodyPassesValidation(t *testing.T) {
	for _, status := range []string{"stop", "start", "restart"} {
		t.Run(status, func(t *testing.T) {
			var reached bool
			e := newComposeStatusValidatorServer(t, &reached)

			rec := putComposeStatus(e, `"`+status+`"`)

			if rec.Code != http.StatusOK {
				t.Fatalf("PUT body %q: status=%d body=%s; want 200 (bare JSON string is the contract)", status, rec.Code, rec.Body.String())
			}
			if !reached {
				t.Fatalf("PUT body %q: handler not reached", status)
			}
		})
	}
}

func TestComposeAppStatus_ObjectBodyRejectedByValidator(t *testing.T) {
	var reached bool
	e := newComposeStatusValidatorServer(t, &reached)

	rec := putComposeStatus(e, `{"status":"stop"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT body {\"status\":\"stop\"}: status=%d body=%s; want 400", rec.Code, rec.Body.String())
	}
	if reached {
		t.Fatal("handler reached despite object body; validator must reject before the handler")
	}
}
