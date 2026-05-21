package v2

import (
	"net/http"

	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/labstack/echo/v4"
)

// catalogSource is the fixed catalog origin. It is intentionally not
// operator-editable (no arbitrary-URL surface): the catalog is the
// bundled powerlab-store release. Operators can only enable/disable it.
const catalogSource = "github.com/neochaotic/powerlab-store"

type catalogStatus struct {
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
}

// GetCatalogStatus reports whether the catalog is enabled and its fixed
// source. The store UI uses this to decide between the app grid and the
// "enable catalog" opt-in prompt.
func (a *AppManagement) GetCatalogStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, catalogStatus{
		Enabled: config.ServerInfo.CatalogEnabled,
		Source:  catalogSource,
	})
}

// SetCatalogEnabled flips the opt-in flag and persists it. The source is
// never changed here — only the enabled state.
func (a *AppManagement) SetCatalogEnabled(ctx echo.Context) error {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := ctx.Bind(&body); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"message": err.Error()})
	}

	config.ServerInfo.CatalogEnabled = body.Enabled
	if err := config.SaveSetup(); err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{"message": err.Error()})
	}

	return ctx.JSON(http.StatusOK, catalogStatus{
		Enabled: config.ServerInfo.CatalogEnabled,
		Source:  catalogSource,
	})
}
