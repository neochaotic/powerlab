// Power-action HTTP handlers for the Settings → Power pane (#260).
//
// Scope is intentionally narrow:
//   GET  /v1/sys/services                   — list PowerLab services + state
//   POST /v1/sys/services/:name/restart     — restart a whitelisted unit
//   POST /v1/sys/host/reboot                — systemctl reboot (requires {"confirm":true})
//   POST /v1/sys/host/shutdown              — systemctl poweroff (requires {"confirm":true})
//
// Security per memory feedback_security_is_priority:
//   - Service name is validated against service.PowerLabServices BEFORE
//     reaching the shell layer.
//   - Destructive host ops require an explicit {"confirm": true} body
//     so a stray GET / cached POST can't trigger them.
//   - These routes assume the JWT middleware in the gateway has already
//     enforced authentication. Authorization (admin-only role) is a
//     follow-up when the role system lands.

package v1

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/service"
)

// GetPowerLabServices returns ServiceState for every PowerLab unit.
// Partial failures are surfaced as a placeholder entry with
// ActiveState="unknown" so the UI can render the row even when systemctl
// errors on one specific unit.
func GetPowerLabServices(ctx echo.Context) error {
	states, _ := service.QueryAllServiceStates()
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    states,
	})
}

// PostRestartPowerLabService restarts a single whitelisted unit. Path
// param `:name` is validated by service.IsAllowedPowerLabService —
// anything else returns 400.
func PostRestartPowerLabService(ctx echo.Context) error {
	name := ctx.Param("name")
	if !service.IsAllowedPowerLabService(name) {
		return ctx.JSON(http.StatusBadRequest, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: "service not in PowerLab whitelist",
			Data:    name,
		})
	}
	out, err := service.RestartPowerLabService(name)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
			Data:    string(out),
		})
	}
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    name,
	})
}

type powerActionRequest struct {
	Confirm bool `json:"confirm"`
}

// PostHostReboot triggers `systemctl reboot`. Requires an explicit
// {"confirm": true} body so an accidental POST without payload can't
// power-cycle the box.
func PostHostReboot(ctx echo.Context) error {
	var req powerActionRequest
	if err := ctx.Bind(&req); err != nil || !req.Confirm {
		return ctx.JSON(http.StatusBadRequest, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: "host reboot requires {\"confirm\": true} body",
		})
	}
	if _, err := service.RebootHost(); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
		})
	}
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: "host reboot initiated",
	})
}

// PostHostShutdown triggers `systemctl poweroff`. Same confirmation
// contract as PostHostReboot.
func PostHostShutdown(ctx echo.Context) error {
	var req powerActionRequest
	if err := ctx.Bind(&req); err != nil || !req.Confirm {
		return ctx.JSON(http.StatusBadRequest, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: "host shutdown requires {\"confirm\": true} body",
		})
	}
	if _, err := service.ShutdownHost(); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: err.Error(),
		})
	}
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: "host shutdown initiated",
	})
}
