package route

import (
	"crypto/ecdsa"
	"log"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neochaotic/powerlab/backend/core/codegen"
	"github.com/neochaotic/powerlab/backend/core/pkg/config"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"

	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
	"github.com/neochaotic/powerlab/backend/common/external"
	common_middleware "github.com/neochaotic/powerlab/backend/common/middleware"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	v2Route "github.com/neochaotic/powerlab/backend/core/route/v2"
)

var (
	_swagger *openapi3.T

	V2APIPath  string
	V2DocPath  string
	V3FilePath string
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
	V3FilePath = "/v3/file"
}

func InitV2Router() http.Handler {
	appManagement := v2Route.NewServer()

	e := echo.New()

	e.Use(common_middleware.Cors())

	e.Use(echo_middleware.Gzip())

	e.Use(echo_middleware.Logger())

	e.Use(echojwt.WithConfig(echojwt.Config{
		Skipper: func(c echo.Context) bool {
			return c.RealIP() == "::1" || c.RealIP() == "127.0.0.1"
			// return true
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
			func(ctx echo.Context) ([]string, error) {
				if len(ctx.Request().Header.Get(echo.HeaderAuthorization)) > 0 {
					return []string{ctx.Request().Header.Get(echo.HeaderAuthorization)}, nil
				}
				return []string{ctx.QueryParam("token")}, nil
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
		Skipper: func(c echo.Context) bool {
			// Skip validation for local development to avoid strict 400 errors
			if c.RealIP() == "::1" || c.RealIP() == "127.0.0.1" {
				return true
			}
			if len(c.Request().Header[echo.HeaderContentType]) > 0 && strings.Contains(c.Request().Header[echo.HeaderContentType][0], "multipart/form-data") {
				return true
			}
			return false
		},
		Options: openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}))

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

func InitFile() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if len(token) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message": "token not found"}`))
			return
		}

		valid, _, errs := jwt.Validate(token, func() (*ecdsa.PublicKey, error) { return external.GetPublicKey(config.CommonInfo.RuntimePath) })
		if errs != nil || !valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message": "validation failure"}`))
			return
		}
		filePath := r.URL.Query().Get("path")
		fileName := path.Base(filePath)
		w.Header().Add("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(fileName))
		http.ServeFile(w, r, filePath)
		// http.ServeFile(w, r, filePath)
	})
}

func InitDir() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if len(token) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message": "token not found"}`))
			return
		}

		valid, _, errs := jwt.Validate(token, func() (*ecdsa.PublicKey, error) { return external.GetPublicKey(config.CommonInfo.RuntimePath) })
		if errs != nil || !valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message": "validation failure"}`))
			return
		}
		t := r.URL.Query().Get("format")
		files := r.URL.Query().Get("files")

		if len(files) == 0 {
			// w.JSON(common_err.CLIENT_ERROR, model.Result{
			// 	Success: common_err.INVALID_PARAMS,
			// 	Message: common_err.GetMsg(common_err.INVALID_PARAMS),
			// })
			return
		}
		list := strings.Split(files, ",")
		for _, v := range list {
			if !file.Exists(v) {
				// return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
				// 	Success: common_err.FILE_DOES_NOT_EXIST,
				// 	Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
				// })
				return
			}
		}
		w.Header().Add("Content-Type", "application/octet-stream")
		w.Header().Add("Content-Transfer-Encoding", "binary")
		w.Header().Add("Cache-Control", "no-cache")
		// handles only single files not folders and multiple files
		//		if len(list) == 1 {

		// filePath := list[0]
		//			info, err := os.Stat(filePath)
		//			if err != nil {

		// w.JSON(http.StatusOK, model.Result{
		// 	Success: common_err.FILE_DOES_NOT_EXIST,
		// 	Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
		// })
		//return
		//			}
		//}

		extension, format, err := file.GetCompressionAlgorithm(t)
		if err != nil {
			// w.JSON(common_err.CLIENT_ERROR, model.Result{
			// 	Success: common_err.INVALID_PARAMS,
			// 	Message: common_err.GetMsg(common_err.INVALID_PARAMS),
			// })
			return
		}

		commonDir := file.CommonPrefix(filepath.Separator, list...)

		currentPath := filepath.Base(commonDir)

		name := "_" + currentPath
		name += extension
		w.Header().Add("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(name))
		if err := file.ArchiveFiles(r.Context(), w, format, list, commonDir); err != nil {
			log.Printf("Failed to archive: %v", err)
		}
	})
}
