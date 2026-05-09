package route

import (
	"crypto/ecdsa"
	"net/http"
	"strconv"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/config"
	v1 "github.com/neochaotic/powerlab/backend/local-storage/route/v1"
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
)

func InitV1Router() http.Handler {
	// check if environment variable is set
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
	e.Use(echo_middleware.Recover())
	e.Use(echo_middleware.Logger())

	// r.GET("/v1/recover/:type", v1.GetRecoverStorage)
	v1Group := e.Group("/v1")

	v1Group.Use(echo_middleware.JWTWithConfig(echo_middleware.JWTConfig{
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
			func(c echo.Context) ([]string, error) {
				if len(c.Request().Header.Get(echo.HeaderAuthorization)) > 0 {
					return []string{c.Request().Header.Get(echo.HeaderAuthorization)}, nil
				}
				return []string{c.QueryParam("token")}, nil
			},
		},
	}))
	{
		v1DisksGroup := v1Group.Group("/disks")
		v1DisksGroup.Use()
		{

			v1DisksGroup.GET("", v1.GetDiskList)
			v1DisksGroup.GET("/usb", v1.GetDisksUSBList)
			v1DisksGroup.DELETE("/usb", v1.DeleteDiskUSB)
			v1DisksGroup.DELETE("", v1.DeleteDisksUmount)
			v1DisksGroup.GET("/size", v1.GetDiskSize)
		}

		v1StorageGroup := v1Group.Group("/storage")
		v1StorageGroup.Use()
		{
			v1StorageGroup.POST("", v1.PostAddStorage)

			v1StorageGroup.PUT("", v1.PutFormatStorage)

			v1StorageGroup.DELETE("", v1.DeleteStorage)
			v1StorageGroup.GET("", v1.GetStorageList)
		}
		// Cloud drive backends (Dropbox, Google Drive) and the
		// /driver list endpoint they fed were removed in Sprint 3
		// Phase 3 (#101 / casaos-strip). Cloud drives depended on
		// the CasaOS-team-hosted OAuth proxy at
		// `cloudoauth.files.casaos.app` — keeping them tethered the
		// product to CasaOS infra forever. Per #101 option 3,
		// dropped entirely for v1.0; revisit post-rebrand if/when
		// PowerLab hosts its own OAuth proxy.
		v1USBGroup := v1Group.Group("/usb")
		v1USBGroup.Use()
		{
			v1USBGroup.PUT("/usb-auto-mount", v1.PutSystemUSBAutoMount) ///sys/usb/:status
			v1USBGroup.GET("/usb-auto-mount", v1.GetSystemUSBAutoMount) ///sys/usb/status
		}
	}

	return e
}
