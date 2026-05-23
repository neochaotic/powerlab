package v1

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/pkg/config"
	fileutil "github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
)

// fileScope returns the configured Files sandbox root. An empty string
// means legacy whole-filesystem access — the behavior for configs that
// predate the [file] Scope setting (#36).
func fileScope() string {
	if config.FileSettingInfo == nil {
		return ""
	}
	return strings.TrimSpace(config.FileSettingInfo.Scope)
}

// scopeOrDeny resolves req within the configured scope, writing a 403
// response and returning ok=false when the path escapes. Handlers use it
// as:
//
//	abs, ok := scopeOrDeny(ctx, rawPath)
//	if !ok { return nil }
func scopeOrDeny(ctx echo.Context, req string) (string, bool) {
	abs, err := fileutil.ResolveWithinScope(fileScope(), req)
	if err != nil {
		// Data carries the scope root so a scope-aware client (the Files
		// page) can fall back to it instead of stranding the user on the
		// rejected out-of-scope path (#36).
		_ = ctx.JSON(http.StatusForbidden, model.Result{
			Success: common_err.INSUFFICIENT_PERMISSIONS,
			Message: "path is outside the permitted file scope",
			Data:    fileScope(),
		})
		return "", false
	}
	return abs, true
}
