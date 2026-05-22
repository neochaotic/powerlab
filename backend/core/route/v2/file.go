package v2

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/core/codegen"
	"github.com/neochaotic/powerlab/backend/core/pkg/config"
	fileutil "github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
)

// Path: route/v2/file.go

func (s *Server) GetFileTest(ctx echo.Context) error {

	//http.ServeFile(w, r, r.URL.Path[1:])
	http.ServeFile(ctx.Response().Writer, ctx.Request(), "/DATA/test.img")

	return ctx.String(200, "pong")
}

func (c *Server) CheckUploadChunk(ctx echo.Context, params codegen.CheckUploadChunkParams) error {
	identifier := ctx.QueryParam("identifier")
	chunkNumber, err := strconv.ParseInt(ctx.QueryParam("chunkNumber"), 10, 64)
	if err != nil {
		return ctx.NoContent(http.StatusBadRequest)
	}

	err = c.fileUploadService.TestChunk(ctx, identifier, chunkNumber)
	if err != nil {
		return ctx.NoContent(http.StatusNoContent)
	}
	return ctx.NoContent(http.StatusOK)
}

func (c *Server) PostUploadFile(ctx echo.Context) error {
	path := ctx.FormValue("path")

	// Sandbox the upload destination (#36): reject paths that escape the
	// configured file scope. Empty scope = legacy whole-fs.
	scope := ""
	if config.FileSettingInfo != nil {
		scope = strings.TrimSpace(config.FileSettingInfo.Scope)
	}
	abs, err := fileutil.ResolveWithinScope(scope, path)
	if err != nil {
		return ctx.JSON(http.StatusForbidden, echo.Map{"message": "path is outside the permitted file scope"})
	}
	path = abs

	// handle the request
	chunkNumber, err := strconv.ParseInt(ctx.FormValue("chunkNumber"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err)
	}
	chunkSize, err := strconv.ParseInt(ctx.FormValue("chunkSize"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err)
	}
	currentChunkSize, err := strconv.ParseInt(ctx.FormValue("currentChunkSize"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err)
	}
	totalChunks, err := strconv.ParseInt(ctx.FormValue("totalChunks"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err)
	}
	totalSize, err := strconv.ParseInt(ctx.FormValue("totalSize"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err)
	}

	identifier := ctx.FormValue("identifier")
	fileName := ctx.FormValue("filename")
	relativePath := ctx.FormValue("relativePath")
	bin, err := ctx.FormFile("file")

	if err != nil {
		return ctx.JSON(http.StatusBadRequest, err)
	}

	err = c.fileUploadService.UploadFile(
		ctx,
		path,
		chunkNumber,
		chunkSize,
		currentChunkSize,
		totalChunks,
		totalSize,
		identifier,
		relativePath,
		fileName,
		bin,
	)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, err)
	}
	return ctx.NoContent(http.StatusOK)
}
