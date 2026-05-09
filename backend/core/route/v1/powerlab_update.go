package v1

import (
	"net/http"

	"github.com/neochaotic/powerlab/backend/common/model"
	common_err "github.com/neochaotic/powerlab/backend/common/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/service"
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
// Spawns install.sh in --upgrade mode and returns 202 Accepted as soon
// as the script is detached. The UI polls
// /v1/powerlab-update/status to learn when it finishes and whether it
// succeeded or rolled back.
func PostPowerLabUpdateInstall(ctx echo.Context) error {
	u := powerLabUpdater()
	res, err := u.Check(ctx.Request().Context())
	if err != nil {
		return ctx.JSON(http.StatusBadGateway, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
		})
	}
	if res.Decision != "update_ok" {
		return ctx.JSON(http.StatusBadRequest, model.Result{
			Success: common_err.CLIENT_ERROR,
			Message: "host is not eligible to upgrade — current decision: " + res.Decision,
		})
	}
	if err := u.RunInstall(ctx.Request().Context(), res.Manifest); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
		})
	}
	// 202 Accepted — the upgrade is now running asynchronously inside
	// install.sh. The HTTP socket closing here is OK because the
	// detached child does not depend on us.
	return ctx.JSON(http.StatusAccepted, model.Result{
		Success: common_err.SUCCESS,
		Message: "upgrade started",
	})
}

// GetPowerLabUpdateStatus handles `GET /v1/powerlab-update/status`.
// Reads /var/lib/powerlab/last-upgrade.json (written by install.sh
// on every --upgrade run). Returns the file's content, or nil data
// when no upgrade has been attempted yet.
//
// The UI polls this every 2 s while the upgrade banner is showing
// "Upgrading…" so it can flip to "Upgrade succeeded" / "Rolled back"
// the moment install.sh finishes.
func GetPowerLabUpdateStatus(ctx echo.Context) error {
	lu, err := powerLabUpdater().LastUpgradeStatus()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
		})
	}
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    lu, // nil → "no upgrade has been attempted on this host"
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
