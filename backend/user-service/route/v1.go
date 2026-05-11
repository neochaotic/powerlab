package route

import (
	"crypto/ecdsa"
	"net/http"
	"strconv"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	v1 "github.com/neochaotic/powerlab/backend/user-service/route/v1"
	"github.com/neochaotic/powerlab/backend/user-service/service"
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
)

// InitRouter constructs the user-service's V1 HTTP router with the
// standard PowerLab middleware chain (CORS / gzip / logger / JWT
// auth). Returns an http.Handler ready to mount under /v1/users.
func InitRouter() http.Handler {
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

	e.Use(echo_middleware.Logger())

	e.POST("/v1/users/register", v1.PostUserRegister)
	e.POST("/v1/users/login", v1.PostUserLogin)
	e.GET("/v1/users/status", v1.GetUserStatus) // init/check

	v1Group := e.Group("/v1")

	v1UsersGroup := v1Group.Group("/users")
	v1UsersGroup.Use(echo_middleware.JWTWithConfig(echo_middleware.JWTConfig{
		Skipper: func(c echo.Context) bool {
			return c.RealIP() == "::1" || c.RealIP() == "127.0.0.1"
		},
		ParseTokenFunc: func(token string, c echo.Context) (interface{}, error) {
			valid, claims, err := jwt.Validate(
				token,
				func() (*ecdsa.PublicKey, error) {
					_, publicKey := service.MyService.User().GetKeyPair()
					return publicKey, nil
				})
			if err != nil || !valid {
				return nil, echo.ErrUnauthorized
			}

			c.Request().Header.Set("user_id", strconv.Itoa(claims.ID))

			return claims, nil
		},
		TokenLookupFuncs: []echo_middleware.ValuesExtractor{
			func(c echo.Context) ([]string, error) {
				if len(c.Request().Header.Get(echo.HeaderAuthorization)) > 0 {
					return []string{c.Request().Header.Get(echo.HeaderAuthorization)}, nil
				}
				return []string{c.QueryParam("token")}, nil
			},
		},
	}))
	{
		v1UsersGroup.Use()
		v1UsersGroup.GET("/current", v1.GetUserInfo)
		v1UsersGroup.PUT("/current", v1.PutUserInfo)
		v1UsersGroup.PUT("/current/password", v1.PutUserPassword)
		// 10 v1 user endpoints removed in Sprint 9 PR K (#252 follow-up):
		// /v1/users/{name, refresh, image, avatar, current/custom/*,
		// current/image/*, /:id DELETE, /:username, '' DELETE all}.
		// UI never consumed any of them; backend/common/external/
		// did not either. Drops ~570 LOC of CasaOS-era user-mgmt
		// handlers (single-user PowerLab doesn't need them).
	}

	return e
}
