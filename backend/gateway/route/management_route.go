package route

import (
	"crypto/ecdsa"
	"net/http"
	"strconv"
	"strings"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/audit"
	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
	"github.com/neochaotic/powerlab/backend/gateway/service"
)

// ManagementRoute owns the management API surface — the routes other
// PowerLab services call into the gateway with (e.g. registering a
// proxied path, querying the live route table, changing the listening
// port). NOT exposed to the public-facing HTTP listener.
//
// The audit field is the optional ADR-0033 audit pipeline. May be
// nil when the audit DB failed to initialise — in that case the
// middleware is not mounted and /v1/audit/* return 503. The gateway
// keeps serving traffic regardless (audit is observability, not a
// hard dependency).
type ManagementRoute struct {
	management *service.Management
	audit      *audit.Service
}

// NewManagementRoute constructs the management route bundle wired to
// the supplied Management service (the in-memory route table). The
// audit.Service is provided by fx; pass nil to disable audit recording.
func NewManagementRoute(management *service.Management, auditSvc *audit.Service) *ManagementRoute {
	return &ManagementRoute{
		management: management,
		audit:      auditSvc,
	}
}

// GetRoute configures and returns the HTTP handler (Echo instance) for the management API surface.
func (m *ManagementRoute) GetRoute() http.Handler {
	e := echo.New()

	e.Use((echo_middleware.CORSWithConfig(echo_middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{echo.POST, echo.GET, echo.OPTIONS, echo.PUT, echo.DELETE},
		AllowHeaders:     []string{echo.HeaderAuthorization, echo.HeaderContentLength, echo.HeaderXCSRFToken, echo.HeaderContentType, echo.HeaderAccessControlAllowOrigin, echo.HeaderAccessControlAllowHeaders, echo.HeaderAccessControlAllowMethods, echo.HeaderConnection, echo.HeaderOrigin, echo.HeaderXRequestedWith},
		ExposeHeaders:    []string{echo.HeaderContentLength, echo.HeaderAccessControlAllowOrigin, echo.HeaderAccessControlAllowHeaders},
		MaxAge:           172800,
		AllowCredentials: true,
	})))

	e.Use(echo_middleware.Gzip())

	// Audit middleware (ADR-0033 + Sprint 16 B1c). Mounted AFTER
	// JWT in the per-endpoint chain so user_id headers are
	// populated; here at the global level we capture the request
	// regardless and the user_id will be NULL for unauthenticated
	// requests (loopback or pre-auth probes). Skipper drops /ping
	// + /v1/audit/* so the audit log doesn't include the health
	// check (every 5s) or its own read-side polling.
	if m.audit != nil {
		e.Use(audit.Middleware(m.audit.Recorder, audit.MiddlewareOptions{
			Skipper: func(c echo.Context) bool {
				p := c.Request().URL.Path
				return p == "/ping" || strings.HasPrefix(p, "/v1/audit/")
			},
		}))
	}

	e.GET("/ping", func(ctx echo.Context) error {
		return ctx.JSON(http.StatusOK, echo.Map{
			"message": "pong from management service",
		})
	})

	m.buildV1Group(e)

	// Audit read endpoints (ADR-0033 + Sprint 16 B1e).
	// /v1/audit/recent and /v1/audit/stats — mounted at the v1
	// level so the auth gate (gateway's JWT middleware) protects
	// them via the existing chain on this group.
	if m.audit != nil {
		auditGroup := e.Group("/v1/audit")
		auditGroup.GET("/recent", audit.RecentHandler(m.audit.DB))
		auditGroup.GET("/stats", audit.StatsHandler(m.audit.DB))
	}

	return e
}

func (m *ManagementRoute) buildV1Group(e *echo.Echo) {
	v1Group := e.Group("/v1")

	v1Group.Use()
	{
		m.buildV1RouteGroup(v1Group)
	}
}

func (m *ManagementRoute) buildV1RouteGroup(v1Group *echo.Group) {
	v1GatewayGroup := v1Group.Group("/gateway")

	v1GatewayGroup.Use()
	{
		v1GatewayGroup.GET("/routes", func(ctx echo.Context) error {
			return ctx.JSON(http.StatusOK, m.management.GetRoutes())
		})

		v1GatewayGroup.POST("/routes",
			func(ctx echo.Context) error {
				var route *model.Route
				err := ctx.Bind(&route)
				if err != nil {
					return ctx.JSON(http.StatusBadRequest, model.Result{
						Success: common_err.CLIENT_ERROR,
						Message: err.Error(),
					})
				}

				if err := m.management.CreateRoute(route); err != nil {
					return ctx.JSON(http.StatusInternalServerError, model.Result{
						Success: common_err.SERVICE_ERROR,
						Message: err.Error(),
					})
				}

				return ctx.NoContent(http.StatusCreated)
			},
			echo_middleware.JWTWithConfig(echo_middleware.JWTConfig{
				Skipper: func(c echo.Context) bool {
					return c.RealIP() == "::1" || c.RealIP() == "127.0.0.1"
					// return true
				},
				ParseTokenFunc: func(token string, c echo.Context) (interface{}, error) {
					valid, claims, err := jwt.Validate(token, func() (*ecdsa.PublicKey, error) { return external.GetPublicKey(m.management.State.GetRuntimePath()) })
					if err != nil || !valid {
						return nil, echo.ErrUnauthorized
					}
					c.Request().Header.Set("user_id", strconv.Itoa(claims.ID))

					return claims, nil
				},
				TokenLookupFuncs: []echo_middleware.ValuesExtractor{
					// Header → ?token= fallback + RFC 6750 Bearer-prefix
					// stripping is centralised in
					// common/utils/jwt.ExtractTokenFromRequest (#342).
					func(c echo.Context) ([]string, error) {
						return []string{jwt.ExtractTokenFromRequest(c)}, nil
					},
				},
			}))

		v1GatewayGroup.GET("/port", func(ctx echo.Context) error {
			return ctx.JSON(http.StatusOK, model.Result{
				Success: common_err.SUCCESS,
				Message: common_err.GetMsg(common_err.SUCCESS),
				Data:    m.management.GetGatewayPort(),
			})
		})

		v1GatewayGroup.PUT("/port",
			func(ctx echo.Context) error {
				var request *model.ChangePortRequest

				if err := ctx.Bind(&request); err != nil {
					return ctx.JSON(http.StatusBadRequest, model.Result{
						Success: common_err.CLIENT_ERROR,
						Message: err.Error(),
					})
				}

				if err := m.management.SetGatewayPort(request.Port); err != nil {
					return ctx.JSON(http.StatusInternalServerError, model.Result{
						Success: common_err.SERVICE_ERROR,
						Message: err.Error(),
					})
				}

				return ctx.JSON(http.StatusOK, model.Result{
					Success: common_err.SUCCESS,
					Message: common_err.GetMsg(common_err.SUCCESS),
				})
			},
			echo_middleware.JWTWithConfig(echo_middleware.JWTConfig{
				Skipper: func(c echo.Context) bool {
					return c.RealIP() == "::1" || c.RealIP() == "127.0.0.1"
					// return true
				},
				ParseTokenFunc: func(token string, c echo.Context) (interface{}, error) {
					valid, claims, err := jwt.Validate(token, func() (*ecdsa.PublicKey, error) { return external.GetPublicKey(m.management.State.GetRuntimePath()) })
					if err != nil || !valid {
						return nil, echo.ErrUnauthorized
					}
					c.Request().Header.Set("user_id", strconv.Itoa(claims.ID))

					return claims, nil
				},
				TokenLookupFuncs: []echo_middleware.ValuesExtractor{
					// Header → ?token= fallback + RFC 6750 Bearer-prefix
					// stripping is centralised in
					// common/utils/jwt.ExtractTokenFromRequest (#342).
					func(c echo.Context) ([]string, error) {
						return []string{jwt.ExtractTokenFromRequest(c)}, nil
					},
				},
			}))
	}
}
