package route

import (
	"crypto/ecdsa"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/neochaotic/powerlab/backend/local-storage/codegen"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/config"
	v2 "github.com/neochaotic/powerlab/backend/local-storage/route/v2"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	echojwt "github.com/labstack/echo-jwt/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
	common_middleware "github.com/neochaotic/powerlab/backend/common/middleware"
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
	localStorage := v2.NewLocalStorage()

	e := echo.New()

	e.Use(common_middleware.Cors())

	e.Use(echo_middleware.Gzip())

	e.Use(echo_middleware.Logger())

	e.Use(echojwt.WithConfig(echojwt.Config{
		Skipper: func(c echo.Context) bool {
			return c.RealIP() == "::1" || c.RealIP() == "127.0.0.1"
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
			func(c echo.Context) ([]string, error) {
				return []string{c.Request().Header.Get(echo.HeaderAuthorization)}, nil
			},
		},
	}))

	e.Use(middleware.OapiRequestValidatorWithOptions(_swagger, &middleware.Options{Options: openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}))

	codegen.RegisterHandlersWithBaseURL(e, localStorage, V2APIPath)

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
