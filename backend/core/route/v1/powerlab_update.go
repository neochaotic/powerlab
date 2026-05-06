package v1

import (
	"net/http"

	"github.com/IceWhaleTech/CasaOS-Common/model"
	common_err "github.com/IceWhaleTech/CasaOS-Common/utils/common_err"
	"github.com/IceWhaleTech/CasaOS/service"
	"github.com/labstack/echo/v4"
)

// GetPowerLabUpdate handles `GET /v1/powerlab-update`. Fetches the
// release manifest from GitHub and returns a structured "what should
// the user do" decision the UI renders directly.
//
// See docs/UPDATE_MANIFEST.md for the contract this endpoint honours.
func GetPowerLabUpdate(ctx echo.Context) error {
	res, err := powerLabUpdater().Check(ctx.Request().Context())
	if err != nil {
		return ctx.JSON(http.StatusBadGateway, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: "Could not reach the release manifest: " + err.Error(),
		})
	}
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    res,
	})
}

// GetPowerLabUpdatePreflight handles `GET /v1/powerlab-update/preflight`.
// Re-fetches the manifest (rather than trusting the caller to send it
// back) and runs each pre_install_check on the host. Returns the
// per-check results.
//
// We re-fetch on purpose: the manifest could have been edited between
// the user's last "check" and "install" click (the maintainer might
// have set skip_release: true after a bad release was discovered).
// Always reading the fresh state is the safe default.
func GetPowerLabUpdatePreflight(ctx echo.Context) error {
	u := powerLabUpdater()
	res, err := u.Check(ctx.Request().Context())
	if err != nil {
		return ctx.JSON(http.StatusBadGateway, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
		})
	}
	if res.Manifest == nil {
		return ctx.JSON(http.StatusOK, model.Result{
			Success: common_err.SUCCESS,
			Message: common_err.GetMsg(common_err.SUCCESS),
			Data:    map[string]any{"checks": []any{}, "decision": res.Decision},
		})
	}
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data: map[string]any{
			"decision": res.Decision,
			"checks":   u.RunPreflight(res.Manifest),
		},
	})
}

// PostPowerLabUpdateInstall handles `POST /v1/powerlab-update/install`.
// Phase 2 returns 501 Not Implemented — the actual install path
// (download → snapshot → swap → health-check → rollback) lands in
// Phase 4 of issue #21. The endpoint exists now so the frontend can
// be wired against the final URL.
func PostPowerLabUpdateInstall(ctx echo.Context) error {
	u := powerLabUpdater()
	res, err := u.Check(ctx.Request().Context())
	if err != nil {
		return ctx.JSON(http.StatusBadGateway, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
		})
	}
	if err := u.RunInstall(ctx.Request().Context(), res.Manifest); err != nil {
		return ctx.JSON(http.StatusNotImplemented, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
		})
	}
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
	})
}

// powerLabUpdater is a tiny factory the package keeps so the singleton
// can be swapped out in tests. Production builds use whatever version
// is baked into common.VERSION at compile time.
var _powerLabUpdater *service.PowerLabUpdater

func powerLabUpdater() *service.PowerLabUpdater {
	if _powerLabUpdater == nil {
		_powerLabUpdater = service.NewPowerLabUpdater(currentPowerLabVersion())
	}
	return _powerLabUpdater
}

// currentPowerLabVersion is the version stamp this binary advertises
// to the updater. It comes from `common.VERSION` (the upstream
// CasaOS-style version constant we still inherit) until we wire in a
// PowerLab-specific version constant — separate cleanup task.
func currentPowerLabVersion() string {
	return powerLabVersionAtCompileTime
}

// powerLabVersionAtCompileTime is overwritten by `-ldflags
// "-X .../route/v1.powerLabVersionAtCompileTime=0.2.4"` from the
// package script. Defaults to "dev" so a developer running `./dev.sh`
// gets a stable, recognisable string in the API responses.
var powerLabVersionAtCompileTime = "dev"
