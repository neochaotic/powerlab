package v2

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/utils/constants"
	"github.com/neochaotic/powerlab/backend/core/codegen"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
	"github.com/neochaotic/powerlab/backend/core/service"
)

func (s *Server) GetHealthServices(ctx echo.Context) error {
	services, err := service.MyService.Health().Services()
	if err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{
			Message: &message,
		})
	}

	return ctx.JSON(http.StatusOK, codegen.GetHealthServicesOK{
		Data: &codegen.HealthServices{
			Running:    services[true],
			NotRunning: services[false],
		},
	})
}

func (s *Server) GetHealthPorts(ctx echo.Context) error {
	tcpPorts, udpPorts, err := service.MyService.Health().Ports()
	if err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{
			Message: &message,
		})
	}

	return ctx.JSON(http.StatusOK, codegen.GetHealthPortsOK{
		Data: &codegen.HealthPorts{
			TCP: &tcpPorts,
			UDP: &udpPorts,
		},
	})
}
func (c *Server) GetHealthlogs(ctx echo.Context) error {
	// constants.DefaultLogPath resolves per-platform: /var/log/powerlab
	// on Linux, /Library/Logs/PowerLab on darwin, dev sandbox in dev.
	logDir := constants.DefaultLogPath
	fileList, err := os.ReadDir(logDir)
	if err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{
			Message: &message,
		})
	}
	extension, format, err := file.GetCompressionAlgorithm("zip")
	if err != nil {
		ctx.Response().Header().Set("Content-Type", "application/json")
		message := err.Error()
		return ctx.JSON(http.StatusNotFound, codegen.ResponseInternalServerError{
			Message: &message,
		})
	}

	commonDir := logDir
	name := filepath.Base(commonDir) + extension
	ctx.Response().Header().Add("Content-Type", "application/octet-stream")
	ctx.Response().Header().Add("Content-Transfer-Encoding", "binary")
	ctx.Response().Header().Add("Cache-Control", "no-cache")
	ctx.Response().Header().Add("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(name))

	paths := make([]string, 0, len(fileList))
	for _, fname := range fileList {
		paths = append(paths, filepath.Join(logDir, fname.Name()))
	}
	if err := file.ArchiveFiles(ctx.Request().Context(), ctx.Response().Writer, format, paths, commonDir); err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{
			Message: &message,
		})
	}
	return nil
}
