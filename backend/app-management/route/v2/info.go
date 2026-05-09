package v2

import (
	"net/http"

	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/common/utils"
	"github.com/labstack/echo/v4"
)

func (a *AppManagement) Info(ctx echo.Context) error {
	architecture, err := docker.CurrentArchitecture()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{
			Message: utils.Ptr(err.Error()),
		})
	}

	return ctx.JSON(http.StatusOK, codegen.InfoOK{
		Architecture: utils.Ptr(architecture),
	})
}

func (a *AppManagement) GetAppManagementConfig(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, codegen.AppManagementConfig{
		StoragePath: config.AppInfo.StoragePath,
		AppsPath:    config.AppInfo.AppsPath,
	})
}
