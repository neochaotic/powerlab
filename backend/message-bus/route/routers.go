package route

import (
	"crypto/ecdsa"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	echojwt "github.com/labstack/echo-jwt/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
	common_middleware "github.com/neochaotic/powerlab/backend/common/middleware"
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/config"
	"github.com/neochaotic/powerlab/backend/message-bus/service"
)

// NewAPIRouter builds the echo HTTP handler for the message-bus
// REST API. Wires CORS, gzip, recovery, request logger, JWT auth
// (skipped for unix-socket + loopback + websocket-upgrade requests),
// and the OpenAPI request validator before mounting the codegen
// handlers under the API path declared in the loaded swagger spec.
func NewAPIRouter(swagger *openapi3.T, services *service.Services) (http.Handler, error) {
	apiRoute := NewAPIRoute(services)

	e := echo.New()

	e.Use(common_middleware.Cors())

	e.Use(echo_middleware.Gzip())
	e.Use(echo_middleware.Recover())
	e.Use(echo_middleware.Logger())

	e.Use(echojwt.WithConfig(echojwt.Config{
		Skipper: func(c echo.Context) bool {
			// skip when source is unix socket
			if c.Request().Host == "unix" {
				return true
			}

			if c.RealIP() == "::1" || c.RealIP() == "127.0.0.1" {
				return true
			}

			if c.Request().Method == echo.GET && c.Request().Header.Get(echo.HeaderUpgrade) == "websocket" {
				return true
			}

			return false
		},
		ParseTokenFunc: func(c echo.Context, auth string) (interface{}, error) {
			valid, claims, err := jwt.Validate(auth, func() (*ecdsa.PublicKey, error) { return external.GetPublicKey(config.CommonInfo.RuntimePath) })
			if err != nil || !valid {
				return nil, echo.ErrUnauthorized
			}

			c.Request().Header.Set("user_id", strconv.Itoa(claims.ID))

			return claims, nil
		},
		TokenLookupFuncs: []echo_middleware.ValuesExtractor{
			// RFC 6750 Bearer-prefix + query fallback centralised
			// in common/utils/jwt.ExtractTokenFromRequest (#342).
			func(c echo.Context) ([]string, error) {
				return []string{jwt.ExtractTokenFromRequest(c)}, nil
			},
		},
	}))

	e.Use(middleware.OapiRequestValidatorWithOptions(swagger, &middleware.Options{Options: openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}))

	apiPath, err := getAPIPath(getSwaggerURL(swagger))
	if err != nil {
		return nil, err
	}

	codegen.RegisterHandlersWithBaseURL(e, apiRoute, apiPath)

	return e, nil
}

// NewDocRouter serves the embedded OpenAPI HTML viewer (Scalar) at
// /doc<apiPath> and the raw OpenAPI YAML at /doc<apiPath>/openapi.yaml.
// Mounted on the unauthenticated docs port — no JWT, no CORS guard.
func NewDocRouter(swagger *openapi3.T, docHTML string, docYAML string) (http.Handler, error) {
	apiPath, err := getAPIPath(getSwaggerURL(swagger))
	if err != nil {
		return nil, err
	}

	docPath := "/doc" + apiPath

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == docPath {
			if _, err := w.Write([]byte(docHTML)); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		if r.URL.Path == docPath+"/openapi.yaml" {
			if _, err := w.Write([]byte(docYAML)); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}), nil
}

func getSwaggerURL(swagger *openapi3.T) string {
	return swagger.Servers[0].URL
}

func getAPIPath(swaggerURL string) (string, error) {
	u, err := url.Parse(swaggerURL)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(u.Path, "/"), nil
}
