package v1

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"reflect"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/common_err"

	model1 "github.com/neochaotic/powerlab/backend/local-storage/model"
	model2 "github.com/neochaotic/powerlab/backend/local-storage/service/model"

	"github.com/neochaotic/powerlab/backend/local-storage/service"
)

func GetStorageList(ctx echo.Context) error {
	system := ctx.QueryParam("system")

	blkList := service.MyService.Disk().LSBLK(false)
	foundSystem := false

	storages := []model1.Storages{}
	df, err := service.MyService.Disk().GetSystemDf()
	// db, err := service.MyService.Disk().GetSerialAllFromDB()
	// if err != nil {
	// 	logger.Error("error when getting all volumes from database", zap.Error(err))
	// 	return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	// 	return
	// }
	// mapdb := make(map[string]string)
	// for _, v := range db {
	// 	mapdb[v.MountPoint] = v.MountPoint
	// }
	for _, currentDisk := range blkList {
		// if currentDisk.Tran == "usb" {
		// 	continue
		// }

		tempSystemDisk := false
		children := 1
		tempDisk := model1.Storages{
			DiskName: currentDisk.Model,
			Path:     currentDisk.Path,
			Size:     currentDisk.Size,
			Type:     currentDisk.Tran,
		}

		storageArr := []model1.Storage{}
		temp := service.MyService.Disk().SmartCTL(currentDisk.Path)
		if reflect.DeepEqual(temp, model1.SmartctlA{}) {
			temp.SmartStatus.Passed = true
		}
		if len(currentDisk.Children) == 0 && service.IsDiskSupported(currentDisk) {
			currentDisk.Children = append(currentDisk.Children, currentDisk)
		}
		for _, blkChild := range currentDisk.Children {
			if err == nil {
				if blkChild.Path == df.FileSystem {
					tempDisk.DiskName = "System"
					foundSystem = true
					tempSystemDisk = true
					_log.Info(ctx.Request().Context(), "found system disk", slog.String("disk", blkChild.Path))
				}
			}
			if blkChild.MountPoint == "" {
				continue
			}
			if !foundSystem {
				if blkChild.MountPoint == "/" {
					tempDisk.DiskName = "System"
					foundSystem = true
					tempSystemDisk = true
				} else {
					for _, c := range blkChild.Children {
						if c.MountPoint == "/" {
							tempDisk.DiskName = "System"
							foundSystem = true
							tempSystemDisk = true
							break
						}
					}
				}
			}
			stor := model1.Storage{
				UUID:        blkChild.UUID,
				MountPoint:  blkChild.MountPoint,
				Size:        blkChild.FSSize.String(),
				Avail:       blkChild.FSAvail.String(),
				Used:        blkChild.FSUsed.String(),
				Path:        blkChild.Path,
				Type:        blkChild.FsType,
				DriveName:   blkChild.Name,
				PersistedIn: service.MyService.Disk().GetPersistentTypeByUUID(blkChild.UUID),
			}
			if len(blkChild.Label) == 0 {
				if stor.MountPoint == "/" {
					stor.Label = "System"
				} else {
					stor.Label = filepath.Base(stor.MountPoint)
				}

				children++
			} else {
				stor.Label = blkChild.Label
			}
			// if _, ok := mapdb[stor.MountPoint]; ok || stor.Label == "System" {
			storageArr = append(storageArr, stor)
			//}

		}

		if len(storageArr) == 0 {
			continue
		}

		if tempSystemDisk && len(system) > 0 {
			tempStorageArr := []model1.Storage{}
			for i := 0; i < len(storageArr); i++ {
				if storageArr[i].MountPoint != "/boot/efi" && storageArr[i].Type != "swap" {
					tempStorageArr = append(tempStorageArr, storageArr[i])
				}
			}
			tempDisk.Children = tempStorageArr
			storages = append(storages, tempDisk)
		} else if !tempSystemDisk {
			tempDisk.Children = storageArr
			storages = append(storages, tempDisk)
		}
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: storages})
}

// PostAddStorage adds a storage device to the PowerLab pool.
// Optionally re-formats the disk first (when ?format=true), then
// mounts each child partition + persists the mount in the volume
// table + emits an ADDED notification. Per-child mount errors are
// collected and returned as a partial-success response.
//
// Sprint 7 #7 (per #227 §F): the original 146-LOC body was split
// into three orthogonal helpers so the orchestration reads top-
// down without scrolling.
func PostAddStorage(ctx echo.Context) error {
	path, name, format, errResp := parseAndValidateAddStorageRequest(ctx)
	if errResp != nil {
		return errResp
	}

	diskMap[path] = "busying"
	defer service.MyService.Disk().RemoveLSBLKCache()
	defer delete(diskMap, path)

	currentDisk := service.MyService.Disk().GetDiskInfo(path)
	if format {
		if errResp := optionallyFormatStorage(ctx, currentDisk, path); errResp != nil {
			return errResp
		}
	}

	currentDisk = service.MyService.Disk().GetDiskInfo(path)
	if len(currentDisk.Children) == 0 && service.IsDiskSupported(currentDisk) {
		currentDisk.Children = append(currentDisk.Children, currentDisk)
	}

	failedPaths := mountStorageChildren(ctx, currentDisk.Children, name)
	if len(failedPaths) > 0 {
		return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SERVICE_ERROR, Message: failedPaths})
	}
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// parseAndValidateAddStorageRequest binds the request body, extracts
// path/name/format, and runs the two early-out checks (empty path +
// disk-already-busying). Returns either the parsed values or a
// ready-to-return error response.
func parseAndValidateAddStorageRequest(ctx echo.Context) (path, name string, format bool, errResp error) {
	js := make(map[string]interface{})
	if err := ctx.Bind(&js); err != nil {
		return "", "", false, ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS), Data: err.Error()})
	}

	path, _ = js["path"].(string)
	name, _ = js["name"].(string)
	format, _ = js["format"].(bool)

	if len(path) == 0 {
		return "", "", false, ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}
	if _, ok := diskMap[path]; ok {
		return "", "", false, ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.DISK_BUSYING, Message: common_err.GetMsg(common_err.DISK_BUSYING)})
	}

	return path, name, format, nil
}

// optionallyFormatStorage runs the destructive format flow:
// unmount the disk's existing mount + delete the partition table +
// add a fresh single partition. Returns a ready-to-return error
// response on any step failure (each step has its own error code
// preserved from the pre-extract behaviour).
func optionallyFormatStorage(ctx echo.Context, currentDisk model1.LSBLKModel, path string) error {
	if err := service.MyService.Disk().UmountPointAndRemoveDir(currentDisk); err != nil {
		_log.Error(ctx.Request().Context(), "error when trying to umount storage", err, slog.String("path", path))
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.REMOVE_MOUNT_POINT_ERROR, Message: err.Error()})
	}

	_log.Info(ctx.Request().Context(), "deleting storage...", slog.String("path", path))
	if err := service.MyService.Disk().DeletePartition(path); err != nil {
		_log.Error(ctx.Request().Context(), "error when trying to delete partition", err, slog.String("path", path))
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	_log.Info(ctx.Request().Context(), "formatting storage...", slog.String("path", path))
	if err := service.MyService.Disk().AddPartition(path); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return nil
}

// mountStorageChildren mounts each child partition under
// /media/<name>, persists the mount in the volume table, and
// emits an ADDED notification. Per-child mount/persist failures
// are collected into a newline-joined string of failed paths
// rather than aborting the loop — matches pre-extract behaviour.
//
// The 3-second UUID-poll-retry workaround for lsblk is preserved.
// On SaveMountPointToDB failure the just-mounted child is
// unmounted to avoid leaking a half-registered mount.
func mountStorageChildren(ctx echo.Context, children []model1.LSBLKModel, name string) string {
	failedPaths := ""
	for _, blkChild := range children {
		mountPoint := blkChild.GetMountPoint(name)
		if output, err := service.MyService.Disk().MountDisk(blkChild.Path, mountPoint); err != nil {
			_log.Error(ctx.Request().Context(), "err", err, slog.String("mountPoint", mountPoint), slog.String("output", output))
			failedPaths += blkChild.Path + "\n"
			continue
		}

		var b model1.LSBLKModel
		retry := 3 // ugly workaround for lsblk not returning UUID after creating partition on time - need a better solution
		for b.UUID == "" && retry > 0 {
			time.Sleep(1 * time.Second)
			b = service.MyService.Disk().GetDiskInfo(blkChild.Path)
			retry--
		}

		m := model2.Volume{
			MountPoint: b.MountPoint,
			UUID:       b.UUID,
			CreatedAt:  time.Now().Unix(),
		}

		if err := service.MyService.Disk().SaveMountPointToDB(m); err != nil {
			blkChild.MountPoint = mountPoint
			service.MyService.Disk().UmountPointAndRemoveDir(blkChild)
			failedPaths += blkChild.Path + "\n"
			continue
		}

		// send notify to client
		go func(blkChild model1.LSBLKModel) {
			message := map[string]interface{}{
				"data": StorageMessage{
					Action: "ADDED",
					Path:   blkChild.Path,
					Volume: "/media/",
					Size:   blkChild.Size,
					Type:   blkChild.Tran,
				},
			}

			if err := service.MyService.Notify().SendNotify(messagePathStorageStatus, message); err != nil {
				_log.Error(ctx.Request().Context(), "error when sending notification", err, slog.String("message path", messagePathStorageStatus), slog.Any("message", message))
			}
		}(blkChild)
	}
	return failedPaths
}

// @Param  pwd formData string true "user password"
// @Param  volume formData string true "mount point"
// @Success 200 {string} string "ok"
// @Router /disk/format [post]
func PutFormatStorage(ctx echo.Context) error {
	js := make(map[string]string)
	if err := ctx.Bind(&js); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS), Data: err.Error()})
	}

	// requires password from user to confirm the action
	// if claims, err := jwt.ParseToken(c.GetHeader("Authorization"), false); err != nil || encryption.GetMD5ByStr(js["password"]) != claims.Password {
	// 	return ctx.JSON(http.StatusUnauthorized, model.Result{Success: common_err.PWD_INVALID, Message: common_err.GetMsg(common_err.PWD_INVALID)})
	// 	return
	// }

	path := js["path"]
	mountPoint := js["volume"]

	if len(path) == 0 {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}

	if _, ok := diskMap[path]; ok {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.DISK_BUSYING, Message: common_err.GetMsg(common_err.DISK_BUSYING)})
	}

	diskMap[path] = "busying"

	defer service.MyService.Disk().RemoveLSBLKCache()
	defer delete(diskMap, path)
	diskInfo := service.MyService.Disk().GetDiskInfo(path)
	if err := service.MyService.Disk().UmountPointAndRemoveDir(diskInfo); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.REMOVE_MOUNT_POINT_ERROR, Message: err.Error()})
	}

	if err := service.MyService.Disk().FormatDisk(path); err != nil {
		delete(diskMap, path)
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.FORMAT_ERROR, Message: common_err.GetMsg(common_err.FORMAT_ERROR)})
	}
	currentDisk := service.MyService.Disk().GetDiskInfo(path)
	for diskInfo.UUID == currentDisk.UUID {
		time.Sleep(1 * time.Second)
		currentDisk = service.MyService.Disk().GetDiskInfo(path)
	}
	if mountPoint == "" {
		mountPoint = currentDisk.GetMountPoint("")
	}

	if output, err := service.MyService.Disk().MountDisk(path, mountPoint); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: output})
	}

	m := model2.Volume{
		MountPoint: mountPoint,
		UUID:       currentDisk.UUID,
		CreatedAt:  time.Now().Unix(),
	}

	if err := service.MyService.Disk().SaveMountPointToDB(m); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

func DeleteStorage(ctx echo.Context) error {
	js := make(map[string]string)
	if err := ctx.Bind(&js); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS), Data: err.Error()})
	}

	// requires password from user to confirm the action
	// if claims, err := jwt.ParseToken(c.GetHeader("Authorization"), false); err != nil || encryption.GetMD5ByStr(js["password"]) != claims.Password {
	// 	return ctx.JSON(http.StatusUnauthorized, model.Result{Success: common_err.PWD_INVALID, Message: common_err.GetMsg(common_err.PWD_INVALID)})
	// 	return
	// }

	path := js["path"]
	mountPoint := js["volume"]

	if len(path) == 0 || len(mountPoint) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}

	if _, ok := diskMap[path]; ok {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.DISK_BUSYING, Message: common_err.GetMsg(common_err.DISK_BUSYING)})
	}
	diskInfo := service.MyService.Disk().GetDiskInfo(path)
	if err := service.MyService.Disk().UmountPointAndRemoveDir(diskInfo); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.REMOVE_MOUNT_POINT_ERROR, Message: err.Error()})
	}

	// delete data
	defer func() {
		if err := service.MyService.Disk().DeleteMountPointFromDB(path, mountPoint); err != nil {
			_log.Error(ctx.Request().Context(), "error when deleting mount point from database", err)
		}
	}()
	defer service.MyService.Disk().RemoveLSBLKCache()

	// send notify to client
	go func() {
		message := map[string]interface{}{
			"data": StorageMessage{
				Action: "REMOVED",
				Path:   path,
				Volume: mountPoint,
				Size:   0,
				Type:   "",
			},
		}

		if err := service.MyService.Notify().SendNotify(messagePathStorageStatus, message); err != nil {
			_log.Error(ctx.Request().Context(), "error when sending notification", err, slog.String("message path", messagePathStorageStatus), slog.Any("message", message))
		}
	}()

	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}
