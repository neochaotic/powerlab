package v2

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/app-management/service"
	v1 "github.com/neochaotic/powerlab/backend/app-management/service/v1"
	"github.com/neochaotic/powerlab/backend/common/utils"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"go.uber.org/zap"
)

func (a *AppManagement) PullImages(ctx echo.Context, params codegen.PullImagesParams) error {
	// attach context key/value pairs from upstream
	backgroundCtx := context.Background()

	if params.ContainerIds != nil {
		containerIDs := strings.Split(*params.ContainerIds, ",")
		for _, containerID := range containerIDs {

			container, err := docker.Container(backgroundCtx, containerID)
			if err != nil {
				logger.Error("get container info failed", zap.Error(err))
				continue
			}

			imageName := docker.ImageName(container)
			if imageName == "" {
				continue
			}

			go func(containerID, imageName string) {
				backgroundCtx := common.WithProperties(backgroundCtx, PropertiesFromQueryParams(ctx))

				eventProperties := common.PropertiesFromContext(backgroundCtx)
				eventProperties[common.PropertyTypeAppName.Name] = v1.AppName(container)
				eventProperties[common.PropertyTypeAppIcon.Name] = v1.AppIcon(container)

				_, err := service.MyService.Docker().PullLatestImage(backgroundCtx, imageName)
				if err != nil {
					logger.Error("pull new image failed", zap.Error(err), zap.String("image", imageName))
				}
			}(containerID, imageName)
		}
	}

	return ctx.JSON(http.StatusOK, codegen.PullImagesOK{
		Message: utils.Ptr("Images are being pulled asynchronously"),
	})
}
