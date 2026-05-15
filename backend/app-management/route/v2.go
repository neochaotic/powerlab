package route

import (
	"crypto/ecdsa"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"

	v2Route "github.com/neochaotic/powerlab/backend/app-management/route/v2"
	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
)

var (
	_swagger *openapi3.T

	V2APIPath string
	V2DocPath string
)

func init() {
	swagger, err := codegen.GetSwagger()
	if err != nil {
		panic(err)
	}

	_swagger = swagger

	u, err := url.Parse(_swagger.Servers[0].URL)
	if err != nil {
		panic(err)
	}

	V2APIPath = strings.TrimRight(u.Path, "/")
	V2DocPath = "/doc" + V2APIPath
}

func InitV2Router() http.Handler {
	appManagement := v2Route.NewAppManagement()

	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}

		// Don't leak internal errors if not needed, but for PowerLab we usually want the message
		message := err.Error()
		if he, ok := err.(*echo.HTTPError); ok {
			if s, ok := he.Message.(string); ok {
				message = s
			}
		}

		c.JSON(code, codegen.ErrorResponse{
			Code:    strconv.Itoa(code),
			Message: message,
		})
	}

	e.Use((echo_middleware.CORSWithConfig(echo_middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{echo.POST, echo.GET, echo.OPTIONS, echo.PUT, echo.DELETE},
		AllowHeaders:     []string{echo.HeaderAuthorization, echo.HeaderContentLength, echo.HeaderXCSRFToken, echo.HeaderContentType, echo.HeaderAccessControlAllowOrigin, echo.HeaderAccessControlAllowHeaders, echo.HeaderAccessControlAllowMethods, echo.HeaderConnection, echo.HeaderOrigin, echo.HeaderXRequestedWith},
		ExposeHeaders:    []string{echo.HeaderContentLength, echo.HeaderAccessControlAllowOrigin, echo.HeaderAccessControlAllowHeaders},
		MaxAge:           172800,
		AllowCredentials: true,
	})))

	// SSE endpoints must NEVER be gzip-compressed — Echo's Gzip wraps
	// the response writer in a deflate stream that batches bytes
	// before emitting, so the browser's EventSource sees the install
	// modal "fica travado na tela de progresso" while events pile up
	// on the wire. Skipper bypasses any path ending in "/logs" (the
	// compose task SSE endpoint). Sister fix to the audit middleware
	// Flusher forwarding in this same PR — both required for
	// end-to-end SSE streaming.
	e.Use(echo_middleware.GzipWithConfig(echo_middleware.GzipConfig{
		Skipper: func(c echo.Context) bool {
			return strings.HasSuffix(c.Request().URL.Path, "/logs")
		},
	}))

	e.Use(echo_middleware.Logger())

	e.Use(echo_middleware.JWTWithConfig(echo_middleware.JWTConfig{
		Skipper: func(c echo.Context) bool {
			return c.RealIP() == "::1" || c.RealIP() == "127.0.0.1"
		},
		ParseTokenFunc: func(token string, c echo.Context) (interface{}, error) {
			valid, claims, err := jwt.Validate(token, func() (*ecdsa.PublicKey, error) { return external.GetPublicKey(config.CommonInfo.RuntimePath) })
			if err != nil || !valid {
				return nil, echo.ErrUnauthorized
			}

			c.Request().Header.Set("user_id", strconv.Itoa(claims.ID))

			return claims, nil
		},
		TokenLookupFuncs: []echo_middleware.ValuesExtractor{
			// Header → ?token= fallback + RFC 6750 Bearer-prefix stripping
			// is centralised in common/utils/jwt.ExtractTokenFromRequest
			// (#342). Browser EventSource (custom-app deploy log stream,
			// SSE task-log viewer) can't send custom headers, hence the
			// query fallback inside the helper.
			func(c echo.Context) ([]string, error) {
				return []string{jwt.ExtractTokenFromRequest(c)}, nil
			},
		},
	}))

	// e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
	// 	return func(c echo.Context) error {
	// 		switch c.Request().Header.Get(echo.HeaderContentType) {
	// 		case common.MIMEApplicationYAML: // in case request contains a compose content in YAML
	// 			return middleware.OapiRequestValidatorWithOptions(_swagger, &middleware.Options{
	// 				Options: openapi3filter.Options{
	// 					AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	// 					// ExcludeRequestBody:  true,
	// 					// ExcludeResponseBody: true,
	// 				},
	// 			})(next)(c)

	// 		default:
	// 			return middleware.OapiRequestValidatorWithOptions(_swagger, &middleware.Options{
	// 				Options: openapi3filter.Options{
	// 					AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	// 				},
	// 			})(next)(c)
	// 		}
	// 	}
	// })

	e.Use(middleware.OapiRequestValidatorWithOptions(_swagger, &middleware.Options{
		Options: openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		// Skip OpenAPI validation for endpoints registered manually below
		// (not in the codegen OpenAPI spec).
		Skipper: func(c echo.Context) bool {
			p := c.Request().URL.Path
			return strings.HasPrefix(p, V2APIPath+"/compose/task/") ||
				strings.HasPrefix(p, V2APIPath+"/ports/check") ||
				// /compose/:id/disk-usage — handler exists in codegen-generated
				// ServerInterface (compose_app.go:546) but the OpenAPI spec
				// shipped with this fork never declared it, so the validator
				// rejects with "no matching operation was found" before the
				// handler runs. Skip validation; route registered manually.
				(strings.HasPrefix(p, V2APIPath+"/compose/") && strings.HasSuffix(p, "/disk-usage")) ||
				// /config — same pattern: GetAppManagementConfig exists in
				// info.go:26 but the OpenAPI spec doesn't declare it, so
				// without this skip + manual registration the validator
				// returns 400 and Settings → About cannot fetch the
				// app-management config to show the user.
				p == V2APIPath+"/config"
		},
	}))

	e.GET(V2APIPath+"/compose/task/:id/logs", appManagement.(*v2Route.AppManagement).GetTaskLogs)
	e.GET(V2APIPath+"/ports/check", appManagement.(*v2Route.AppManagement).CheckPorts)
	// disk-usage handler signature is `(ctx, id ComposeAppID)` because
	// it was added to the codegen ServerInterface; wrap to read :id off
	// the echo path param ourselves.
	e.GET(V2APIPath+"/compose/:id/disk-usage", func(c echo.Context) error {
		return appManagement.(*v2Route.AppManagement).ComposeAppDiskUsage(c, c.Param("id"))
	})
	e.GET(V2APIPath+"/config", appManagement.(*v2Route.AppManagement).GetAppManagementConfig)

	codegen.RegisterHandlersWithBaseURL(e, appManagement, V2APIPath)

	return e
}

func InitV2DocRouter(docHTML string, docYAML string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == V2DocPath {
			if _, err := w.Write([]byte(docHTML)); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		if r.URL.Path == V2DocPath+"/openapi.yaml" {
			if _, err := w.Write([]byte(docYAML)); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	})
}
