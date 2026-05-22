package v1

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/core/model"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
	"github.com/neochaotic/powerlab/backend/core/service"
)

// RenamePath renames a file or directory. Refuses when the source
// is a mounted directory (mount-renames are unsafe — would silently
// detach the mount).
//
// @Summary rename file or dir
// @Produce  application/json
// @Accept application/json
// @Tags file
// @Security ApiKeyAuth
// @Param oldpath body string true "path of old"
// @Param newpath body string true "path of new"
// @Success 200 {string} string "ok"
// @Router /file/rename [put]
func RenamePath(ctx echo.Context) error {
	json := make(map[string]string)
	ctx.Bind(&json)
	op := json["old_path"]
	np := json["new_path"]
	if len(op) == 0 || len(np) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}
	mounted := service.IsMounted(op)
	if mounted {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.MOUNTED_DIRECTIORIES, Message: common_err.GetMsg(common_err.MOUNTED_DIRECTIORIES), Data: common_err.GetMsg(common_err.MOUNTED_DIRECTIORIES)})
	}

	success, err := service.MyService.System().RenameFile(op, np)
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: success, Message: common_err.GetMsg(success), Data: err})
}

// MkdirAll creates a directory + parents. Idempotent — already-
// exists returns success with the DIR_ALREADY_EXISTS code.
//
// @Summary create folder
// @Produce  application/json
// @Accept  application/json
// @Tags file
// @Security ApiKeyAuth
// @Param path body string true "path of folder"
// @Success 200 {string} string "ok"
// @Router /file/mkdir [post]
func MkdirAll(ctx echo.Context) error {
	json := make(map[string]string)
	ctx.Bind(&json)
	path := json["path"]
	var code int
	if len(path) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}
	abs, ok := scopeOrDeny(ctx, path)
	if !ok {
		return nil
	}
	path = abs
	code, _ = service.MyService.System().MkdirAll(path)
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: code, Message: common_err.GetMsg(code)})
}

// PostOperateFileOrDir queues a copy or move operation on the
// async file-operation worker. Refuses when the source is mounted
// (move would detach the mount). Schedules the worker + status
// notifier on the first queued op.
//
// @Summary copy or move file
// @Produce  application/json
// @Accept  application/json
// @Tags file
// @Security ApiKeyAuth
// @Param body body model.FileOperate true "type:move,copy"
// @Success 200 {string} string "ok"
// @Router /file/operate [post]
func PostOperateFileOrDir(ctx echo.Context) error {
	list := model.FileOperate{}
	ctx.Bind(&list)

	if len(list.Item) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}
	// Sandbox both ends of every copy/move (#36): source and destination
	// must stay within the configured file scope.
	for i := range list.Item {
		absFrom, ok := scopeOrDeny(ctx, list.Item[i].From)
		if !ok {
			return nil
		}
		list.Item[i].From = absFrom
	}
	if absTo, ok := scopeOrDeny(ctx, list.To); ok {
		list.To = absTo
	} else {
		return nil
	}
	if list.To == list.Item[0].From[:strings.LastIndex(list.Item[0].From, "/")] {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SOURCE_DES_SAME, Message: common_err.GetMsg(common_err.SOURCE_DES_SAME)})
	}

	var total int64 = 0
	for i := 0; i < len(list.Item); i++ {

		size, err := file.GetFileOrDirSize(list.Item[i].From)
		if err != nil {
			continue
		}
		list.Item[i].Size = size
		total += size
		if list.Type == "move" {
			mounted := service.IsMounted(list.Item[i].From)
			if mounted {
				return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.MOUNTED_DIRECTIORIES, Message: common_err.GetMsg(common_err.MOUNTED_DIRECTIORIES), Data: common_err.GetMsg(common_err.MOUNTED_DIRECTIORIES)})
			}
		}
	}

	list.TotalSize = total
	list.ProcessedSize = 0

	uid := uuid.NewString()
	service.FileQueue.Store(uid, list)
	service.OpStrArr = append(service.OpStrArr, uid)
	if len(service.OpStrArr) == 1 {
		go service.ExecOpFile()
		go service.CheckFileStatus()

		go service.MyService.Notify().SendFileOperateNotify(false)

	}

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// DeleteFile removes one or more paths via os.RemoveAll. Refuses
// when any path is a mounted directory (would detach the mount).
//
// @Summary delete file
// @Produce  application/json
// @Accept  application/json
// @Tags file
// @Security ApiKeyAuth
// @Param body body string true "paths eg [\"/a/b/c\",\"/d/e/f\"]"
// @Success 200 {string} string "ok"
// @Router /file/delete [delete]
func DeleteFile(ctx echo.Context) error {
	paths := []string{}
	ctx.Bind(&paths)
	if len(paths) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}

	// Sandbox every target before deleting anything (#36).
	scoped := make([]string, 0, len(paths))
	for _, v := range paths {
		abs, ok := scopeOrDeny(ctx, v)
		if !ok {
			return nil
		}
		scoped = append(scoped, abs)
	}
	paths = scoped

	for _, v := range paths {
		mounted := service.IsMounted(v)
		if mounted {
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.MOUNTED_DIRECTIORIES, Message: common_err.GetMsg(common_err.MOUNTED_DIRECTIORIES), Data: common_err.GetMsg(common_err.MOUNTED_DIRECTIORIES)})
		}
	}

	for _, v := range paths {
		err := os.RemoveAll(v)
		if err != nil {
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.FILE_DELETE_ERROR, Message: common_err.GetMsg(common_err.FILE_DELETE_ERROR), Data: err})
		}
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// PutFileContent updates the contents of an EXISTING file. 404 if the
// file does not exist. The editor's "save new file" flow uses POST
// /v1/file (PostFileContent) which creates; PUT is update-only by
// design, mirroring filebrowser's REST semantics:
//
//	POST /v1/file       create new (409 if exists, unless override=true)
//	PUT  /v1/file       update existing (404 if missing)
//
// @Summary update existing file
// @Produce application/json
// @Accept application/json
// @Tags file
// @Security ApiKeyAuth
// @Router /file [put]
func PutFileContent(ctx echo.Context) error {
	fi := model.FileUpdate{}
	if err := ctx.Bind(&fi); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: err.Error()})
	}
	if fi.FilePath == "" {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: "file_path is required"})
	}
	abs, ok := scopeOrDeny(ctx, fi.FilePath)
	if !ok {
		return nil
	}
	fi.FilePath = abs
	if !file.Exists(fi.FilePath) {
		return ctx.JSON(http.StatusNotFound, model.Result{
			Success: common_err.FILE_DOES_NOT_EXIST,
			Message: fmt.Sprintf("file does not exist: %s — POST to create", fi.FilePath),
		})
	}
	existing, err := os.Stat(fi.FilePath)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	mode := existing.Mode().Perm()
	if err := file.WriteToFullPath([]byte(fi.FileContent), fi.FilePath, mode); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// PostFileContent creates a new file. Default semantics: 409 Conflict
// if the file already exists. Pass `?override=true` to replace.
// Auto-mkdir-p's the parent directory (like filebrowser's resourcePostHandler).
//
// Accepts BOTH request shapes for backwards compatibility with the
// legacy "+ New File" button:
//
//	{"path":"/foo/bar.txt"}                         // legacy, empty content
//	{"file_path":"/foo/bar.txt","file_content":""}  // empty file, new shape
//	{"file_path":"/foo/bar.txt","file_content":"hello"}  // file with content
//
// @Summary create new file
// @Produce application/json
// @Accept application/json
// @Tags file
// @Security ApiKeyAuth
// @Param override query bool false "overwrite if exists"
// @Router /file [post]
func PostFileContent(ctx echo.Context) error {
	// Accept either shape — legacy `{path}` (empty file create from
	// "+ New File" button) or new `{file_path, file_content}` from
	// the editor "Save as new" flow. One handler, two payloads.
	var req struct {
		Path        string `json:"path"`
		FilePath    string `json:"file_path"`
		FileContent string `json:"file_content"`
	}
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: err.Error()})
	}
	target := req.FilePath
	if target == "" {
		target = req.Path
	}
	if target == "" {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: "file_path (or legacy `path`) is required"})
	}
	if abs, ok := scopeOrDeny(ctx, target); ok {
		target = abs
	} else {
		return nil
	}
	override := ctx.QueryParam("override") == "true"
	if file.Exists(target) && !override {
		return ctx.JSON(http.StatusConflict, model.Result{
			Success: common_err.FILE_ALREADY_EXISTS,
			Message: fmt.Sprintf("file already exists: %s — pass ?override=true to overwrite or use PUT to update", target),
		})
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	if err := file.WriteToFullPath([]byte(req.FileContent), target, 0o644); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// DeleteOperateFileOrDir cancels an in-flight file operation by its
// queue id. id == "0" wipes the entire queue (cancel-all).
func DeleteOperateFileOrDir(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "0" {
		service.FileQueue = sync.Map{}
		service.OpStrArr = []string{}
	} else {

		service.FileQueue.Delete(id)
		tempList := []string{}
		for _, v := range service.OpStrArr {
			if v != id {
				tempList = append(tempList, v)
			}
		}
		service.OpStrArr = tempList

	}

	go service.MyService.Notify().SendFileOperateNotify(true)
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}
