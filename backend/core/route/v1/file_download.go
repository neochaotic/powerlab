package v1

import (
	"log"
	"net/http"
	"net/url"
	url2 "net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/h2non/filetype"
	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/core/model"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
)

// GetDownloadFile streams either a single file or a multi-file
// archive (zip/tar/targz, picked via ?format). For multi-file the
// archive is built on-the-fly to the response writer; per-file
// archive errors are logged but don't abort the stream.
//
// @Summary download
// @Produce  application/json
// @Accept application/json
// @Tags file
// @Security ApiKeyAuth
// @Param format query string false "Compression format" Enums(zip,tar,targz)
// @Param files query string true "file list eg: filename1,filename2,filename3 "
// @Success 200 {string} string "ok"
// @Router /file/download [get]
func GetDownloadFile(ctx echo.Context) error {
	t := ctx.QueryParam("format")

	files := ctx.QueryParam("files")

	if len(files) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: common_err.GetMsg(common_err.INVALID_PARAMS),
		})
	}
	list := strings.Split(files, ",")
	for _, v := range list {
		if !file.Exists(v) {
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
				Success: common_err.FILE_DOES_NOT_EXIST,
				Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
			})
		}
	}
	ctx.Request().Header.Add("Content-Type", "application/octet-stream")
	ctx.Request().Header.Add("Content-Transfer-Encoding", "binary")
	ctx.Request().Header.Add("Cache-Control", "no-cache")
	// handles only single files not folders and multiple files
	if len(list) == 1 {

		filePath := list[0]
		info, err := os.Stat(filePath)
		if err != nil {
			return ctx.JSON(http.StatusOK, model.Result{
				Success: common_err.FILE_DOES_NOT_EXIST,
				Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
			})
		}
		if !info.IsDir() {

			// 打开文件
			fileTmp, _ := os.Open(filePath)
			defer fileTmp.Close()

			// 获取文件的名称
			fileName := path.Base(filePath)
			ctx.Response().Header().Add("Content-Disposition", "attachment; filename*=utf-8''"+url2.PathEscape(fileName))
			ctx.File(filePath)
		}
	}

	extension, ar, err := file.GetCompressionAlgorithm(t)
	if err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: common_err.GetMsg(common_err.INVALID_PARAMS),
		})
	}

	err = ar.Create(ctx.Response().Writer)
	if err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
			Success: common_err.SERVICE_ERROR,
			Message: common_err.GetMsg(common_err.SERVICE_ERROR),
			Data:    err.Error(),
		})
	}
	defer ar.Close()
	commonDir := file.CommonPrefix(filepath.Separator, list...)

	currentPath := filepath.Base(commonDir)

	name := "_" + currentPath
	name += extension
	ctx.Request().Header.Add("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(name))
	for _, fname := range list {
		err = file.AddFile(ar, fname, commonDir)
		if err != nil {
			log.Printf("Failed to archive %s: %v", fname, err)
		}
	}
	return nil
}

// GetDownloadSingleFile streams a single file with full content-
// type detection (filetype magic-byte sniff on first 261 bytes)
// + Last-Modified + Content-Length headers. Used by the file-
// preview drawer for inline-renderable files (images, videos).
func GetDownloadSingleFile(ctx echo.Context) error {
	filePath := ctx.QueryParam("path")
	if len(filePath) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: common_err.GetMsg(common_err.INVALID_PARAMS),
		})
	}
	fileName := path.Base(filePath)
	// c.Header("Content-Disposition", "inline")
	ctx.Request().Header.Add("Content-Disposition", "attachment; filename*=utf-8''"+url2.PathEscape(fileName))

	fi, err := os.Open(filePath)
	if err != nil {
		// Audit #216 §C item 2: was `panic(err)` — converted to a
		// graceful error response matching the existing 267-style
		// pattern further down. The pkg/lifecycle recover middleware
		// still catches process-restart panics, but a missing file
		// is a CLIENT-shaped error (404-shaped semantically) and
		// should not look like a backend crash to the caller.
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
			Success: common_err.FILE_DOES_NOT_EXIST,
			Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
		})
	}

	// We only have to pass the file header = first 261 bytes
	buffer := make([]byte, 261)

	_, _ = fi.Read(buffer)

	kind, _ := filetype.Match(buffer)
	if kind != filetype.Unknown {
		ctx.Request().Header.Add("Content-Type", kind.MIME.Value)
	}
	node, err := os.Stat(filePath)
	// Set the Last-Modified header to the timestamp
	ctx.Request().Header.Add("Last-Modified", node.ModTime().UTC().Format(http.TimeFormat))

	knownSize := node.Size() >= 0
	if knownSize {
		ctx.Request().Header.Add("Content-Length", strconv.FormatInt(node.Size(), 10))
	}
	http.ServeContent(ctx.Response().Writer, ctx.Request(), fileName, node.ModTime(), fi)
	defer fi.Close()
	fileTmp, err := os.Open(filePath)
	if err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
			Success: common_err.FILE_DOES_NOT_EXIST,
			Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
		})
	}
	defer fileTmp.Close()

	return nil
}
