package v1

import (
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"os"
	osuser "os/user"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/CasaOS-Common/external"
	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS-Common/utils/jwt"
	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/config"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/labstack/echo/v4"
)

// extractUsername returns the username from the JWT in the request,
// works even when the JWT middleware skipped (localhost auth bypass).
// Falls back to the "user_name" header set by the middleware on the
// happy path.
func extractUsername(ctx echo.Context) string {
	if u := ctx.Request().Header.Get("user_name"); u != "" {
		return u
	}
	tok := ctx.Request().Header.Get("Authorization")
	if tok == "" {
		tok = ctx.QueryParam("token")
	}
	tok = strings.TrimPrefix(tok, "Bearer ")
	if tok == "" {
		return ""
	}
	valid, claims, err := jwt.Validate(tok, func() (*ecdsa.PublicKey, error) {
		return external.GetPublicKey(config.CommonInfo.RuntimePath)
	})
	if err != nil || !valid {
		return ""
	}
	return claims.Username
}

// FileHomeResponse — body of GET /v1/file/home. The UI navigates the
// Files page to `path` on first load instead of dropping the user into
// `/DATA` (which doesn't exist on dev hosts) or the filesystem root
// (which is hostile for a typical user).
type FileHomeResponse struct {
	Path   string `json:"path"`
	Source string `json:"source"` // "os-home" | "system-fallback"
}

// GetFilerHome returns a sensible starting path for the Files page.
//
// Logic:
//
//  1. If the JWT carries a username AND that username matches a real
//     OS account (PAM on Linux, dscl on macOS), suggest
//     `<user.HomeDir>/PowerLab/` and `mkdir -p` it. The user already
//     has write permission to their own home; no chmod games.
//
//  2. Otherwise (SetupWizard bcrypt user with no OS account), fall
//     back to a system-managed dir under PowerLab's data tree —
//     `/var/lib/powerlab/files` on Linux, `~powerlab/files` on
//     macOS dev. The daemon owns this dir, so writes work
//     regardless of which user "owns" the JWT.
//
// The /v1/file/home endpoint is JWT-protected like the rest of /v1/file/*,
// so the username is available via the `user_name` request header that
// the JWT middleware sets (see route/v1.go).
func GetFilerHome(ctx echo.Context) error {
	username := extractUsername(ctx)

	// Path 1: real OS user → ~/PowerLab/
	if username != "" {
		if u, err := osuser.Lookup(username); err == nil && u.HomeDir != "" {
			home := filepath.Join(u.HomeDir, "PowerLab")
			// mkdir -p with the OS user's permissions implied by the
			// daemon's process context. On Linux production the
			// daemon runs as root so this works for any /home/<x>/;
			// on macOS dev the daemon runs as the developer, so this
			// only works for that one user (which is fine — single
			// developer = single home dir touched).
			if err := os.MkdirAll(home, 0o755); err == nil {
				return ctx.JSON(http.StatusOK, model.Result{
					Success: common_err.SUCCESS,
					Message: common_err.GetMsg(common_err.SUCCESS),
					Data: FileHomeResponse{
						Path:   home,
						Source: "os-home",
					},
				})
			}
		}
	}

	// Path 2: system fallback. Use the platform-aware data path
	// (Linux production: /var/lib/powerlab/files; macOS production:
	// /opt/powerlab/lib/files; dev sandbox: <projectdir>/backend/data/files).
	// constants.DefaultDataPath is set per-platform and is what every
	// other PowerLab service treats as canonical writable storage,
	// so the daemon definitely has permissions there.
	fallback := filepath.Join(constants.DefaultDataPath, "files")
	if err := os.MkdirAll(fallback, 0o755); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: fmt.Sprintf("could not prepare default Files path: %v", err),
		})
	}
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data: FileHomeResponse{
			Path:   fallback,
			Source: "system-fallback",
		},
	})
}
