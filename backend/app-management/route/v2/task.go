package v2

import (
	"fmt"
	"net/http"

	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/labstack/echo/v4"
)

func (a *AppManagement) GetTaskLogs(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"message": "task id is required"})
	}

	task := service.MyTaskService.GetOrCreate(id)

	// Set headers for SSE
	ctx.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	ctx.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	ctx.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	ctx.Response().Header().Set("Access-Control-Allow-Origin", "*")

	ctx.Response().WriteHeader(http.StatusOK)

	ch, cleanup := task.Subscribe()
	defer cleanup()

	for {
		select {
		case line, ok := <-ch:
			if !ok {
				// Task finished
				fmt.Fprintf(ctx.Response(), "event: end\ndata: task finished\n\n")
				ctx.Response().Flush()
				return nil
			}
			// Send log line as SSE data
			fmt.Fprintf(ctx.Response(), "data: %s\n\n", line)
			ctx.Response().Flush()
		case <-ctx.Request().Context().Done():
			// Client disconnected
			return nil
		}
	}
}
