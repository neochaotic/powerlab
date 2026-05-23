package v1

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/neochaotic/powerlab/backend/core/model"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
	"go.uber.org/zap"
)

// GetFileUpload reports whether a chunk for the given upload has
// already been received. The UI's chunked-upload library calls this
// before each chunk to decide whether to skip the actual transfer.
//
// @Summary upload file
// @Produce  application/json
// @Accept  application/json
// @Tags file
// @Security ApiKeyAuth
// @Param path formData string false "file path"
// @Param file formData file true "file"
// @Success 200 {string} string "ok"
// @Router /file/upload [get]
func GetFileUpload(ctx echo.Context) error {
	relative := ctx.QueryParam("relativePath")
	fileName := ctx.QueryParam("filename")
	chunkNumber := ctx.QueryParam("chunkNumber")
	totalChunks, _ := strconv.Atoi(utils.DefaultQuery(ctx, "totalChunks", "0"))
	path := ctx.QueryParam("path")
	dirPath := ""
	hash := file.GetHashByContent([]byte(fileName))
	if file.Exists(path + "/" + relative) {
		return ctx.JSON(http.StatusConflict, model.Result{Success: http.StatusConflict, Message: common_err.GetMsg(common_err.FILE_ALREADY_EXISTS)})
	}
	tempDir := filepath.Join(path, ".temp", hash+strconv.Itoa(totalChunks)) + "/"
	if fileName != relative {
		dirPath = strings.TrimSuffix(relative, fileName)
		tempDir += dirPath
		file.MkDir(path + "/" + dirPath)
	}
	tempDir += chunkNumber
	if !file.CheckNotExist(tempDir) {
		return ctx.JSON(200, model.Result{Success: 200, Message: common_err.GetMsg(common_err.FILE_ALREADY_EXISTS)})
	}

	return ctx.JSON(204, model.Result{Success: 204, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// PostFileUpload writes a single multipart chunk to disk. When all
// chunks have arrived, splices them together into the final file
// and schedules cleanup of the temp dir.
//
// @Summary upload file
// @Produce  application/json
// @Accept  multipart/form-data
// @Tags file
// @Security ApiKeyAuth
// @Param path formData string false "file path"
// @Param file formData file true "file"
// @Success 200 {string} string "ok"
// @Router /file/upload [post]
func PostFileUpload(ctx echo.Context) error {
	// Don't swallow the FormFile error — without this, a missing or
	// malformed multipart `file` part returns nil for `f` and the
	// io.Copy(out, f) below nil-derefs. The original CasaOS code
	// silently dropped the error; we surface it as 400 instead.
	f, _, formErr := ctx.Request().FormFile("file")
	if formErr != nil || f == nil {
		logger.Error("upload: missing or invalid `file` part", zap.Error(formErr))
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: "upload: missing or invalid `file` part"})
	}
	defer f.Close()
	relative := ctx.FormValue("relativePath")
	fileName := ctx.FormValue("filename")
	totalChunks, _ := strconv.Atoi(utils.DefaultPostForm(ctx, "totalChunks", "0"))
	chunkNumber := ctx.FormValue("chunkNumber")
	dirPath := ""
	path := ctx.FormValue("path")

	hash := file.GetHashByContent([]byte(fileName))

	if len(path) == 0 {
		logger.Error("path should not be empty")
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}
	// Auto-create the destination directory if it doesn't exist.
	// Mirrors `mkdir -p` semantics — the user is asking us to put a
	// file at this path; if the parent dir is missing we create it
	// rather than 500'ing. This is what the Files page expects for
	// a folder the user navigated to (e.g. /DATA which exists in
	// production but not on a fresh dev machine). Permissions match
	// the install.sh defaults (0755) so subsequent reads work.
	if err := os.MkdirAll(path, 0o755); err != nil {
		logger.Error("upload: failed to create destination dir", zap.String("path", path), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: fmt.Sprintf("could not create destination directory %q: %v", path, err)})
	}
	tempDir := filepath.Join(path, ".temp", hash+strconv.Itoa(totalChunks)) + "/"

	if fileName != relative {
		dirPath = strings.TrimSuffix(relative, fileName)
		tempDir += dirPath
		if err := file.MkDir(path + "/" + dirPath); err != nil {
			logger.Error("error when trying to create `"+path+"/"+dirPath+"`", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}
	}

	path += "/" + relative

	if !file.CheckNotExist(tempDir + chunkNumber) {
		if err := file.RMDir(tempDir + chunkNumber); err != nil {
			logger.Error("error when trying to remove existing `"+tempDir+chunkNumber+"`", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}
	}

	if totalChunks > 1 {
		if err := file.IsNotExistMkDir(tempDir); err != nil {
			logger.Error("error when trying to create `"+tempDir+"`", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}

		out, err := os.OpenFile(tempDir+chunkNumber, os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			logger.Error("error when trying to open `"+tempDir+chunkNumber+"` for creation", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}

		defer out.Close()

		if _, err := io.Copy(out, f); err != nil { // recommend to use https://github.com/iceber/iouring-go for faster copy
			logger.Error("error when trying to write to `"+tempDir+chunkNumber+"`", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}

		fileNum, err := ioutil.ReadDir(tempDir)
		if err != nil {
			logger.Error("error when trying to read number of files under `"+tempDir+"`", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}

		if totalChunks == len(fileNum) {
			if err := file.SpliceFiles(tempDir, path, totalChunks, 1); err != nil {
				logger.Error("error when trying to splice files under `"+tempDir+"`", zap.Error(err))
				return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
			}
			go func() {
				time.Sleep(11 * time.Second)
				if err := file.RMDir(tempDir); err != nil {
					logger.Error("error when trying to remove `"+tempDir+"`", zap.Error(err))
				}
			}()
		}
	} else {
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			logger.Error("error when trying to open `"+path+"` for creation", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}

		defer out.Close()

		if _, err := io.Copy(out, f); err != nil { // recommend to use https://github.com/iceber/iouring-go for faster copy
			logger.Error("error when trying to write to `"+path+"`", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: common_err.GetMsg(common_err.SERVICE_ERROR), Data: err.Error()})
		}
	}
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// PostFileOctet is the raw multipart-stream upload variant — used
// by clients that can't or won't pre-chunk. Parses boundary by
// hand, writes each part to disk in 24KB blocks. Lower-level than
// PostFileUpload; the chunked path is preferred for large files.
func PostFileOctet(ctx echo.Context) error {
	content_length := ctx.Request().ContentLength
	if content_length <= 0 || content_length > 1024*1024*1024*2*1024 {
		log.Printf("content_length error\n")
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: "content_length error"})
	}
	content_type_, has_key := ctx.Request().Header["Content-Type"]
	if !has_key {
		log.Printf("Content-Type error\n")
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: "Content-Type error"})
	}
	if len(content_type_) != 1 {
		log.Printf("Content-Type count error\n")
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: "Content-Type count error"})
	}
	content_type := content_type_[0]
	const BOUNDARY string = "; boundary="
	loc := strings.Index(content_type, BOUNDARY)
	if loc == -1 {
		log.Printf("Content-Type error, no boundary\n")
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: "Content-Type error, no boundary"})
	}
	boundary := []byte(content_type[(loc + len(BOUNDARY)):])
	log.Printf("[%s]\n\n", boundary)
	read_data := make([]byte, 1024*24)
	var read_total int = 0
	for {
		file_header, file_data, err := file.ParseFromHead(read_data, read_total, append(boundary, []byte("\r\n")...), ctx.Request().Body)
		if err != nil {
			log.Printf("%v", err)
		}
		log.Printf("file :%s\n", file_header)
		//
		//os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o644)
		f, err := os.OpenFile(file_header["path"]+"/"+file_header["filename"], os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			log.Printf("create file fail:%v\n", err)
		}
		f.Write(file_data)
		file_data = nil

		temp_data, reach_end, err := file.ReadToBoundary(boundary, ctx.Request().Body, f)
		f.Close()
		if err != nil {
			log.Printf("%v\n", err)
		}
		if reach_end {
			break
		} else {
			copy(read_data[0:], temp_data)
			read_total = len(temp_data)
			continue
		}
	}
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}
