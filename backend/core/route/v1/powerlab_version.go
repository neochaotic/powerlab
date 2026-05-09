package v1

import (
	"net/http"
	"runtime"

	"github.com/neochaotic/powerlab/backend/core/common"
	"github.com/labstack/echo/v4"
)

// PowerLabVersionResponse is the body of GET /v1/powerlab/version. The
// UI uses `version` for the handshake and `arch`/`go_version` purely
// for diagnostic display in Settings → About.
type PowerLabVersionResponse struct {
	Version   string `json:"version"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
}

// GetPowerLabVersion returns the version this running binary was built
// with (link-time -X override of common.POWERLAB_VERSION). The UI calls
// this once on app boot, compares to its compiled-in __APP_VERSION__,
// and shows a "UI cached — please reload" banner on mismatch.
//
// Returns "dev" for unflagged developer builds.
//
// This is intentionally NOT JWT-protected: the handshake must work
// before the user authenticates, otherwise we cannot warn a user
// staring at a stale login screen that the backend has moved on.
func GetPowerLabVersion(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, PowerLabVersionResponse{
		Version:   common.POWERLAB_VERSION,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
	})
}
