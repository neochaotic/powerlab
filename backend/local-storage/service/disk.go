package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/moby/sys/mountinfo"
	command2 "github.com/neochaotic/powerlab/backend/common/utils/command"
	"github.com/neochaotic/powerlab/backend/common/utils/constants"
	"github.com/neochaotic/powerlab/backend/common/utils/exec"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/local-storage/codegen/message_bus"
	"github.com/neochaotic/powerlab/backend/local-storage/common"
	"github.com/neochaotic/powerlab/backend/local-storage/model"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/config"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/fstab"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/mount"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/partition"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/utils/command"
	model2 "github.com/neochaotic/powerlab/backend/local-storage/service/model"
	v2 "github.com/neochaotic/powerlab/backend/local-storage/service/v2"
	"github.com/neochaotic/powerlab/backend/local-storage/service/v2/fs"
	"gorm.io/gorm"
)

// DiskService is the disk-management surface — block-device
// inspection (lsblk + smartctl), partition + format ops, mount-on-
// boot persistence, and the merge-pool seeding logic that wires
// /DATA up on first boot.
type DiskService interface {
	// EnsureDefaultMergePoint creates the /DATA mergerfs mount on
	// first boot and re-uses it on subsequent boots. Returns true
	// when the mount point exists + is healthy.
	EnsureDefaultMergePoint() bool
	// AddPartition partitions the given block device with a single
	// powerlab-formatted partition spanning the full disk.
	AddPartition(path string) error
	// DeletePartition removes the partition table from the given
	// block device. Destructive.
	DeletePartition(path string) error
	// CheckSerialDiskMount re-validates every saved mount entry
	// against the live block-device list and removes stale ones.
	// Called at startup + on hot-plug.
	CheckSerialDiskMount()
	// FormatDisk re-formats the given block device with the
	// powerlab default filesystem (ext4). Destructive.
	FormatDisk(path string) error
	// GetDiskInfo returns the lsblk row for the given block device,
	// or a zero LSBLKModel if not found.
	GetDiskInfo(path string) model.LSBLKModel
	// GetPersistentTypeByUUID reports whether the volume identified
	// by uuid is persisted via fstab, the powerlab DB, or not at
	// all. Drives the "Auto-mount on boot" UI badge.
	GetPersistentTypeByUUID(uuid string) string
	// GetUSBDriveStatusList returns the USB drive lifecycle list
	// (mount status + label) used by the USB widget.
	GetUSBDriveStatusList() []model.USBDriveStatus
	// LSBLK returns the system block-device list. isUseCache true
	// returns the cached snapshot (cheap); false re-runs lsblk.
	LSBLK(isUseCache bool) []model.LSBLKModel
	// MountDisk mounts path at /mnt/<volume>, creating the dir if
	// missing. Returns the actual mount point on success.
	MountDisk(path, volume string) (string, error)
	// RemoveLSBLKCache invalidates the LSBLK cache so the next
	// LSBLK(true) re-shells out.
	RemoveLSBLKCache()
	// SmartCTL returns the smartctl readout for the given device,
	// from cache when available.
	SmartCTL(path string) model.SmartctlA
	// UmountPointAndRemoveDir unmounts the given block device's
	// mount point and removes the now-empty directory.
	UmountPointAndRemoveDir(m model.LSBLKModel) error
	// UmountUSB unmounts a USB device by path. Used by the eject
	// button.
	UmountUSB(path string) error

	// UpdateMountPointInDB upserts a volume row.
	UpdateMountPointInDB(m model2.Volume) error
	// DeleteMountPointFromDB removes the volume row identified by
	// (path, mountPoint).
	DeleteMountPointFromDB(path, mountPoint string) error
	// GetSerialAllFromDB returns every persisted volume row.
	GetSerialAllFromDB() ([]model2.Volume, error)
	// SaveMountPointToDB inserts a new volume row, returning
	// ErrVolumeWithEmptyUUID if the volume has no UUID.
	SaveMountPointToDB(m model2.Volume) error
	// InitCheck runs at process start: validates the merge point,
	// re-mounts persisted volumes, and warns on schema drift.
	InitCheck()
	// GetSystemDf returns df-style free-space stats for the root
	// filesystem.
	GetSystemDf() (model.DFDiskSpace, error)
}

type diskService struct {
	db *gorm.DB
}

// PersistentType* constants are the wire-format tokens returned via
// the /v1/storage API in the `PersistedIn` field. Sprint 3 Phase 3
// rebrand: "casaos" → "powerlab" + const renamed
// PersistentTypeCasaOS → PersistentTypePowerLab so the API surface
// is self-describing. Pre-v1.0 wire-format change; verified via grep
// that no PowerLab UI consumer switches on the literal "casaos".
const (
	PersistentTypeNone     = "none"
	PersistentTypeFStab    = "fstab"
	PersistentTypePowerLab = "powerlab"
)

var (
	ErrVolumeWithEmptyUUID = errors.New("cannot save volume with empty uuid")
	json2                  = jsoniter.ConfigCompatibleWithStandardLibrary
)

func (d *diskService) EnsureDefaultMergePoint() bool {
	mountPoint := common.DefaultMountPoint
	sourceBasePath := constants.DefaultFilePath

	existingMerges, err := MyService.LocalStorage().GetMergeAllFromDB(&mountPoint)
	if err != nil {
		// Audit #216 §C item 2 follow-up: was panic(err) — converted
		// to a logged error + return false so callers (main.go +
		// route/v2/merge.go) get the "mergerfs disabled" path they
		// already handle, instead of relying on the recover middleware
		// to dress up a process crash as a 500. Same pattern as
		// PR #230 (GetDownloadSingleFile fix).
		_log.Error(context.Background(), "failed to read existing merges from DB", err, slog.String("mount point", mountPoint))
		return false
	}

	// check if /DATA is already a merge point
	if len(existingMerges) > 0 {
		if len(existingMerges) > 1 {
			_log.Error(context.Background(), "more than one merge point with the same mount point found", nil, slog.String("mount point", mountPoint))
		}
		config.ServerInfo.EnableMergerFS = "true"
		return true
	}

	merge := &model2.Merge{
		FSType:         fs.MergerFSFullName,
		MountPoint:     mountPoint,
		SourceBasePath: &sourceBasePath,
	}
	if err := MyService.LocalStorage().CreateMerge(merge); err != nil {
		if errors.Is(err, v2.ErrMergeMountPointAlreadyExists) {
			_log.Info(context.Background(), err.Error(), slog.String("mount point", mountPoint))
		} else if errors.Is(err, v2.ErrMountPointIsNotEmpty) {
			_log.Error(context.Background(), "Mount point "+mountPoint+" is not empty", nil, slog.String("mount point", mountPoint))
			return false
		} else {
			// Audit #216 §C item 2 follow-up: was panic(err).
			_log.Error(context.Background(), "failed to create merge", err, slog.String("mount point", mountPoint))
			return false
		}
	}

	// mounts, err := MyService.LocalStorage().GetMounts(codegen.GetMountsParams{})
	// if err != nil {
	// 	logger.Error("failed to get mount list from system", zap.Error(err))
	// 	return false
	// }
	// isExist := false
	// for _, v := range mounts {
	// 	if v.MountPoint == mountPoint {
	// 		config.ServerInfo.EnableMergerFS = "true"
	// 		isExist = true
	// 		merge.SourceBasePath = v.Source
	// 		break
	// 	}
	// }

	// if !isExist {
	// 	if err := MyService.LocalStorage().CreateMerge(merge); err != nil {
	// 		if errors.Is(err, v2.ErrMergeMountPointAlreadyExists) {
	// 			logger.Info(err.Error(), zap.String("mount point", mountPoint))
	// 		} else if errors.Is(err, v2.ErrMountPointIsNotEmpty) {
	// 			logger.Error("Mount point "+mountPoint+" is not empty", zap.String("mount point", mountPoint))
	// 			return false
	// 		} else {
	// 			panic(err)
	// 		}
	// 	}
	// }

	if err := MyService.LocalStorage().CreateMergeInDB(merge); err != nil {
		// Audit #216 §C item 2 follow-up: was panic(err).
		_log.Error(context.Background(), "failed to persist merge to DB", err, slog.String("mount point", mountPoint))
		return false
	}
	config.ServerInfo.EnableMergerFS = "true"
	return true
}

func (d *diskService) RemoveLSBLKCache() {
	key := "system_lsblk"
	Cache.Delete(key)
}

func (d *diskService) UmountUSB(path string) error {
	_, err := command2.ExecResultStr("source " + config.AppInfo.ShellPath + "/local-storage-helper.sh ;UDEVILUmount " + path)
	if err != nil {
		return err
	}

	return nil
}

func (d *diskService) SmartCTL(path string) model.SmartctlA {
	key := "system_smart_" + path
	if result, ok := Cache.Get(key); ok {

		res, ok := result.(model.SmartctlA)
		if ok {
			return res
		}
	}
	var m model.SmartctlA
	buf := command.ExecSmartCTLByPath(path)
	if buf == nil {
		if err := Cache.Add(key, m, time.Minute*10); err != nil {
			// logger.Error("failed to add cache", zap.Error(err), zap.String("key", key))
		}
		return m
	}

	err := json2.Unmarshal(buf, &m)
	if err != nil {
		// logger.Error("failed to unmarshal json", zap.Error(err), zap.String("json", string(buf)))
	}
	if !reflect.DeepEqual(m, model.SmartctlA{}) {
		if err := Cache.Add(key, m, time.Hour*24); err != nil {
			// logger.Error("failed to add cache", zap.Error(err), zap.String("key", key))
		}
	}
	return m
}

// 格式化硬盘
func (d *diskService) FormatDisk(path string) error {
	// wait for partition path to be ready
	count := 5
	for count > 0 {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				time.Sleep(1 * time.Second)
				count--
				continue
			}
			_log.Error(context.Background(), "error when checking partition path", err, slog.String("path", path))
			return err
		}
		break
	}

	_log.Info(context.Background(), "formatting partition...", slog.String("path", path))
	if err := partition.FormatPartition(path); err != nil {
		_log.Error(context.Background(), "failed to format partition", err, slog.String("path", path))
		return err
	}

	return nil
}

// 移除挂载点,删除目录
func (d *diskService) UmountPointAndRemoveDir(m model.LSBLKModel) error {
	if len(m.MountPoint) > 0 {
		if err := mount.UmountByMountPoint(m.MountPoint); err != nil {
			_log.Error(context.Background(), "error when umounting partition", err, slog.String("path", m.Path), slog.String("mount point", m.MountPoint))
			return err
		}
		if err := file.RMDir(m.MountPoint); err != nil {
			_log.Error(context.Background(), "error when removing mount point directory", err, slog.String("path", m.Path), slog.String("mount point", m.MountPoint))
			return err
		}
	}
	for _, p := range m.Children {
		if len(p.MountPoint) > 0 {

			if err := mount.UmountByMountPoint(p.MountPoint); err != nil {
				_log.Error(context.Background(), "error when umounting partition", err, slog.String("path", p.Path), slog.String("mount point", p.MountPoint))
				return err
			}
			if err := file.RMDir(p.MountPoint); err != nil {
				_log.Error(context.Background(), "error when removing mount point directory", err, slog.String("path", p.Path), slog.String("mount point", p.MountPoint))
				return err
			}
		}
	}

	return nil
}

// part
func (d *diskService) AddPartition(path string) error {
	_log.Info(context.Background(), "creating partition table...", slog.String("path", path))
	if err := partition.CreatePartitionTable(path); err != nil {
		_log.Error(context.Background(), "failed to create partition table", err, slog.String("path", path))
		return err
	}

	_log.Info(context.Background(), "creating partition...", slog.String("path", path))
	partitions, err := partition.AddPartition(path)
	if err != nil {
		_log.Error(context.Background(), "failed to create partition", err, slog.String("path", path))
		return err
	}

	for _, p := range partitions {
		partitionPath := p.LSBLKProperties["PATH"]

		// wait for partition path to be ready
		count := 5
		for count > 0 {
			if _, err := os.Stat(partitionPath); err != nil {
				if os.IsNotExist(err) {
					time.Sleep(1 * time.Second)
					count--
					continue
				}
				_log.Error(context.Background(), "error when checking partition path", err, slog.String("path", partitionPath))
				return err
			}
			break
		}

		_log.Info(context.Background(), "formatting partition...", slog.String("path", partitionPath))
		if err := partition.FormatPartition(partitionPath); err != nil {
			_log.Error(context.Background(), "failed to format partition", err, slog.String("path", partitionPath))
			return err
		}
	}

	return nil
}

func (d *diskService) DeletePartition(path string) error {
	// check if path exists
	if !file.Exists(path) {
		return errors.New("device " + path + " does not exists")
	}

	_log.Info(context.Background(), "trying to get all partitions of device...", slog.String("path", path))
	partitions, err := partition.GetPartitions(path)
	if err != nil {
		_log.Error(context.Background(), "error when getting all partitions of device", err, slog.String("path", path))
		return err
	}

	for _, p := range partitions {

		n, err := strconv.Atoi(p.PARTXProperties["NR"])
		if err != nil {
			_log.Error(context.Background(), "error when converting partition number to int", err, slog.String("path", path), slog.String("partition number", p.PARTXProperties["NR"]))
			return err
		}

		_log.Info(context.Background(), "trying to delete partition...", slog.String("path", p.LSBLKProperties["PATH"]))
		if err := partition.DeletePartition(path, n); err != nil {
			_log.Error(context.Background(), "error when deleting partition", err, slog.String("path", p.LSBLKProperties["PATH"]))
			return err
		}
	}

	return nil
}

// get disk details
func (d *diskService) LSBLK(isUseCache bool) []model.LSBLKModel {
	key := "system_lsblk"

	if isUseCache {
		if result, ok := Cache.Get(key); ok {
			if res, ok := result.([]model.LSBLKModel); ok {
				return res
			}
		}
	}

	str := command.ExecLSBLK()
	if str == nil {
		_log.Error(context.Background(), "Failed to exec shell - lsblk exec error", nil)
		return nil
	}

	blkList, err := ParseBlockDevices(str)
	if err != nil {
		_log.Error(context.Background(), "Failed to parse block devices from output of lsblk", err)
	}

	var fsused uint64

	result := make([]model.LSBLKModel, 0)

	for _, blk := range blkList {

		if blk.Type == "loop" || blk.RO {
			continue
		}

		fsused = 0

		var blkChildren []model.LSBLKModel
		smart := MyService.Disk().SmartCTL(blk.Path)
		for _, child := range blk.Children {
			if child.RM {

				// if strings.ToLower(strings.TrimSpace(child.State)) != "ok" {
				// 	health = false
				// }
				f, _ := strconv.ParseUint(child.FSUsed.String(), 10, 64)
				fsused += f
			}
			blkChildren = append(blkChildren, child)
		}
		if smart.SmartStatus.Passed {
			blk.Health = "OK"
		} else {
			for _, v := range smart.Smartctl.Messages {
				if strings.Contains(v.String, "STANDBY") {
					blk.Health = "OK"
					break
				}
			}
		}

		blk.FSUsed = json.Number(fmt.Sprintf("%d", fsused))
		blk.Children = blkChildren
		if fsused > 0 {
			blk.UsedPercent, err = strconv.ParseFloat(fmt.Sprintf("%.4f", float64(fsused)/float64(blk.Size)), 64)
			if err != nil {
				_log.Error(context.Background(), "Failed to parse float", err)
			}
		}
		result = append(result, blk)
	}

	if len(result) > 0 {
		Cache.Set(key, result, time.Second*100)
	}

	return result
}

func (d *diskService) GetDiskInfo(path string) model.LSBLKModel {
	str := command.ExecLSBLKByPath(path)
	if str == nil {
		_log.Error(context.Background(), "Failed to exec shell - lsblk exec error", nil)
		return model.LSBLKModel{}
	}

	blkList, err := ParseBlockDevices(str)
	if err != nil {
		_log.Error(context.Background(), "Failed to parse block devices from output of lsblk", err)
		return model.LSBLKModel{}
	}

	blk := model.LSBLKModel{}
	if len(blkList) > 0 {
		blk = blkList[0]
	}
	return blk
}

func (d *diskService) MountDisk(path, mountPoint string) (string, error) {
	_log.Info(context.Background(), "trying to mount...", slog.String("path", path), slog.String("mountPoint", mountPoint))

	// check if path is already mounted at mountPoint
	if mountInfoList, err := mountinfo.GetMounts(func(i *mountinfo.Info) (skip bool, stop bool) {
		if i.Source == path && i.Mountpoint == mountPoint {
			return false, true
		}
		return true, false
	}); err != nil {
		_log.Error(context.Background(), "error when trying to get mount info", err)
		return "", err
	} else if len(mountInfoList) > 0 {
		_log.Info(context.Background(), "already mounted", slog.String("path", path), slog.String("mount point", mountPoint))
		return "", nil
	}

	if err := file.IsNotExistMkDir(mountPoint); err != nil {
		_log.Error(context.Background(), "error when checking if mount point already exists, or when creating the mount point if it does not exists", err, slog.String("mount point", mountPoint))
		return "", err
	}

	if out, err := command2.OnlyExec("source " + config.AppInfo.ShellPath + "/local-storage-helper.sh ;do_mount " + path + " " + mountPoint); err != nil {
		_log.Error(context.Background(), "error when mounting", err, slog.String("path", path), slog.String("mount point", mountPoint), slog.String("output", string(out)))
		return out, err
	}

	// return "", partition.ProbePartition(path)
	return "", nil
}

func (d *diskService) SaveMountPointToDB(m model2.Volume) error {
	if m.UUID == "" {
		return ErrVolumeWithEmptyUUID
	}

	var existing model2.Volume

	result := d.db.Where(&model2.Volume{UUID: m.UUID}).Limit(1).Find(&existing)

	if result.Error != nil {
		_log.Error(context.Background(), "error when querying volume by UUID", result.Error, slog.Any("uuid", m.UUID))
		return result.Error
	}

	if result.RowsAffected > 0 {
		m.ID = existing.ID
	}

	if result := d.db.Save(&m); result.Error != nil {
		_log.Error(context.Background(), "error when saving volume to db", result.Error, slog.Any("volume", m))
		return result.Error
	}

	return nil
}

func (d *diskService) UpdateMountPointInDB(m model2.Volume) error {
	result := d.db.Model(&model2.Volume{}).Where(&model2.Volume{UUID: m.UUID}).Update("mount_point", m.MountPoint)
	if result.Error != nil {
		_log.Error(context.Background(), "error when updating mount point in db by UUID", result.Error, slog.String("uuid", m.UUID), slog.String("mount point", m.MountPoint))
		return result.Error
	}

	_log.Info(context.Background(), strconv.Itoa(int(result.RowsAffected))+" volume(s) with mount point updated in db by UUID", slog.String("uuid", m.UUID), slog.String("mount point", m.MountPoint))

	return nil
}

func (d *diskService) DeleteMountPointFromDB(path, mountPoint string) error {
	partitions, err := partition.GetPartitions(path)
	if err != nil {
		_log.Error(context.Background(), "error when getting partitions by path", err, slog.String("path", path))
		return err
	}

	if len(partitions) != 1 {
		_log.Error(context.Background(), "there should be only 1 partition returned", nil, slog.Any("partitions", partitions))
	}

	var existingVolumes []model2.Volume
	f := model2.Volume{MountPoint: mountPoint}
	if len(partitions) > 0 {
		f.UUID = partitions[0].LSBLKProperties[`UUID`]
		_log.Info(context.Background(), "trying to delete volume by path and mount point", slog.String("path", path), slog.String("mount point", mountPoint), slog.Any("uuid", partitions[0].LSBLKProperties[`UUID`]), slog.Any("partitons", partitions))
	}

	result := d.db.Where(&f).Limit(1).Find(&existingVolumes)
	_log.Info(context.Background(), "result", slog.Any("result", result))
	if result.Error != nil {
		_log.Error(context.Background(), "error when finding the volume by path and mount point", result.Error, slog.String("path", path), slog.String("mount point", mountPoint))
	}

	if result.RowsAffected == 0 {
		_log.Info(context.Background(), "no volume found by path and mount point", slog.String("path", path), slog.String("mount point", mountPoint))
		return nil
	}

	if result := d.db.Delete(&existingVolumes); result.Error != nil {
		_log.Error(context.Background(), "error when deleting volume", result.Error, slog.Any("volume", existingVolumes))
		return result.Error
	}

	return nil
}

func (d *diskService) GetSerialAllFromDB() ([]model2.Volume, error) {
	var volumes []model2.Volume

	result := d.db.Find(&volumes)
	if result.Error != nil {
		_log.Error(context.Background(), "error when querying all volumes from db", result.Error)
		return nil, result.Error
	}

	return volumes, nil
}

func (d *diskService) GetPersistentTypeByUUID(uuid string) string {
	// check if path is in database
	var m model2.Volume

	if result := d.db.Where(&model2.Volume{UUID: uuid}).Limit(1).Find(&m); result.Error != nil {
		_log.Error(context.Background(), "error when finding the volume by uuid in database", result.Error, slog.String("uuid", uuid))
	} else if result.RowsAffected > 0 {
		return PersistentTypePowerLab
	}

	// check if it is in fstab
	if entry, err := fstab.Get().GetEntryBySource(uuid); err != nil {
		_log.Error(context.Background(), "error when finding the volume by uuid in fstab", err, slog.String("uuid", uuid))
	} else if entry != nil {
		return PersistentTypeFStab
	}

	// return none if not found
	return PersistentTypeNone
}

func (d *diskService) CheckSerialDiskMount() {
	ctx := context.Background()
	_log.Info(ctx, "Checking serial disk mount...")

	// check mount point
	dbList, err := d.GetSerialAllFromDB()
	if err != nil {
		_log.Error(ctx, "error when getting all volumes from db", err)
		return
	}

	list := d.LSBLK(true)
	mountPointMap := make(map[string]string, len(dbList))

	defer d.RemoveLSBLKCache()

	// remount
	for _, v := range dbList {
		_log.Info(ctx, "previously persisted mount point", slog.Any("volume", v))
		mountPointMap[v.UUID] = v.MountPoint
	}

	for _, currentDisk := range list {
		output, err := command.ExecEnabledSMART(currentDisk.Path)
		if err != nil {
			if output != nil {
				_log.Error(ctx, "failed to enable S.M.A.R.T: "+string(output), err, slog.String("path", currentDisk.Path))
			} else {
				_log.Error(ctx, "failed to enable S.M.A.R.T", err, slog.String("path", currentDisk.Path))
			}
		}

		for _, blkChild := range currentDisk.Children {
			m, ok := mountPointMap[blkChild.UUID]
			if !ok {
				continue
			}
			if blkChild.MountPoint == m {
				continue
			}
			_log.Info(ctx, "trying to re-mount...", slog.String("path", blkChild.Path), slog.String("mount point", m))
			// mount point check
			mountPoint := m
			mount.UmountByMountPoint(m)
			dir, _ := ioutil.ReadDir(m)
			if len(dir) > 0 {
				i := 1
				for {
					mountPoint = m + "-" + strconv.Itoa(i)
					if file.CheckNotExist(mountPoint) {
						break
					}
					i++
				}
				_log.Info(ctx, "mount point already exists, using new mount point", slog.String("path", blkChild.Path), slog.String("mount point", mountPoint))
			}

			if output, err := d.MountDisk(blkChild.Path, mountPoint); err != nil {
				_log.Error(ctx, output, err, slog.String("path", blkChild.Path), slog.String("volume", mountPoint))
			}

			// obtain the actual mount path (just in case)
			partitions, err := partition.GetPartitions(blkChild.Path)
			if err != nil {
				_log.Error(ctx, "error when getting partitions by path", err, slog.String("path", blkChild.Path))
				continue
			}

			mountPoint = partitions[0].LSBLKProperties["MOUNTPOINT"]

			if mountPoint != m {
				v := model2.Volume{
					UUID:       blkChild.UUID,
					MountPoint: mountPoint,
				}
				if err := d.UpdateMountPointInDB(v); err != nil {
					_log.Error(ctx, "error when updating mount point in db", err, slog.Any("volume", v))
				}
			}
		}
	}
}

func (d *diskService) GetUSBDriveStatusList() []model.USBDriveStatus {
	blockList := d.LSBLK(false)
	statusList := []model.USBDriveStatus{}
	for _, v := range blockList {
		if v.Tran != "usb" {
			continue
		}

		isMount := false
		status := model.USBDriveStatus{Model: v.Model, Name: v.Name, Size: v.Size}
		for _, child := range v.Children {
			if len(child.MountPoint) > 0 {
				isMount = true
				avail, _ := strconv.ParseUint(child.FSAvail.String(), 10, 64)
				status.Avail += avail
			}
		}
		if !isMount && len(v.MountPoint) > 0 {
			isMount = true
			avail, _ := strconv.ParseUint(v.FSAvail.String(), 10, 64)
			status.Avail += avail
		}

		if isMount {
			statusList = append(statusList, status)
		}
	}
	return statusList
}

func (d *diskService) InitCheck() {
	time.Sleep(time.Second * 5)
	var fileName string = "local-storage.json"
	diskMap := make(map[string]model.LSBLKModel)
	diskMapNew := make(map[string]model.LSBLKModel)
	diskTempFilePath := filepath.Join(config.AppInfo.DBPath, fileName)
	if file.Exists(diskTempFilePath) {
		tempData := file.ReadFullFile(diskTempFilePath)
		err := json.Unmarshal(tempData, &diskMap)
		if err != nil {
			os.Remove(diskTempFilePath)
		}
	}

	diskList := MyService.Disk().LSBLK(false)
	for _, v := range diskList {
		if IsDiskSupported(v) {
			if _, ok := diskMap[v.Serial]; !ok {
				properties := common.AdditionalProperties(v)
				eventModel := message_bus.Event{
					SourceID:   "local-storage",
					Name:       "local-storage:disk:added",
					Properties: properties,
				}
				// add UI properties to applicable events so that PowerLab UI can render it
				event := common.EventAdapterWithUIProperties(&eventModel)

				bk := false
				for _, k := range v.Children {
					if k.MountPoint == "/" {
						bk = true
						break
					}
					for _, s := range k.Children {
						if s.MountPoint == "/" {
							bk = true
							break
						}
					}
					if bk {
						break
					}
				}
				if bk {
					continue
				}

				_log.Info(context.Background(), "disk added", slog.Any("eventModel", eventModel))

				response, err := MyService.MessageBus().PublishEventWithResponse(context.Background(), event.SourceID, event.Name, event.Properties)
				if err != nil {
					_log.Error(context.Background(), "failed to publish event to message bus", err, slog.Any("event", event))
					continue
				}

				if response.StatusCode() != http.StatusOK {
					_log.Error(context.Background(), "failed to publish event to message bus", nil, slog.String("status", response.Status()), slog.Any("response", response))
				}

			}
			diskMapNew[v.Serial] = v
		}
	}
	for k, v := range diskMap {
		if _, ok := diskMapNew[k]; !ok {
			_log.Info(context.Background(), "disk removed", slog.Any("disk", v))
			properties := common.AdditionalProperties(v)
			eventModel := message_bus.Event{
				SourceID:   "local-storage",
				Name:       "local-storage:disk:removed",
				Properties: properties,
			}
			event := common.EventAdapterWithUIProperties(&eventModel)
			_log.Info(context.Background(), "InitCheck disk removed", slog.Any("eventModel", eventModel))
			response, err := MyService.MessageBus().PublishEventWithResponse(context.Background(), event.SourceID, event.Name, event.Properties)
			if err != nil {
				_log.Error(context.Background(), "failed to publish event to message bus", err, slog.Any("event", event))
			}

			if response.StatusCode() != http.StatusOK {
				_log.Error(context.Background(), "failed to publish event to message bus", nil, slog.String("status", response.Status()), slog.Any("response", response))
			}
		}
	}
	data, err := json.Marshal(diskMapNew)
	if err != nil {
		return
	}
	file.WriteToPath(data, config.AppInfo.DBPath, fileName)
}

func (d *diskService) GetSystemDf() (model.DFDiskSpace, error) {
	out, err := exec.Command("df", "-kPT").Output()
	if err != nil {
		log.Fatal(err)
	}

	outputStr := string(out)
	// 按行分割字符串
	lines := strings.Split(outputStr, "\n")
	// 忽略第一行（标题行）
	lines = lines[1:]
	// 遍历每一行，解析文件信息
	for _, line := range lines {
		// 分割行，获取各个字段
		fields := strings.Fields(line)
		// 如果行为空，则跳过
		if len(fields) == 0 {
			continue
		}
		if len(fields) == 7 && fields[6] == "/" {
			m := model.DFDiskSpace{
				FileSystem: fields[0],
				Type:       fields[1],

				UsePercent: fields[5],
				MountedOn:  fields[6],
			}
			b, _ := strconv.ParseInt(fields[2], 10, 64)
			u, _ := strconv.ParseInt(fields[3], 10, 64)
			a, _ := strconv.ParseInt(fields[4], 10, 64)
			m.Blocks = strconv.FormatInt(b*1024, 10)
			m.Used = strconv.FormatInt(u*1024, 10)
			m.Available = strconv.FormatInt(a*1024, 10)
			return m, nil
		} else {
			continue
		}
	}
	return model.DFDiskSpace{}, errors.New("not found")
}

// NewDiskService returns a DiskService backed by db.
func NewDiskService(db *gorm.DB) DiskService {
	return &diskService{db: db}
}

// IsDiskSupported reports whether the given block device is one
// PowerLab is willing to manage. Filters out optical drives,
// loop devices, and other non-storage types.
func IsDiskSupported(d model.LSBLKModel) bool {
	return d.Tran == "sata" ||
		d.Tran == "nvme" ||
		d.Tran == "spi" ||
		d.Tran == "sas" ||
		strings.Contains(d.SubSystems, "virtio") ||
		strings.Contains(d.SubSystems, "block:scsi:vmbus:acpi") || // Microsoft Hyper-V
		strings.Contains(d.SubSystems, "block:mmc:mmc_host:pci") ||
		strings.Contains(d.SubSystems, "block:mmc:mmc_host:platform") ||
		strings.Contains(d.SubSystems, "block:scsi:pci") || d.Tran == "usb"
}

// IsFormatSupported reports whether the given block device's
// current filesystem is one PowerLab can mount + manage (vs.
// "unknown" / proprietary types we won't touch).
func IsFormatSupported(d model.LSBLKModel) bool {
	if d.FsType == "vfat" || d.FsType == "ext4" || d.FsType == "ext3" || d.FsType == "ext2" || d.FsType == "exfat" || d.FsType == "ntfs-3g" || d.FsType == "iso9660" {
		return true
	}
	return false
}

// WalkDisk does a depth-limited DFS over a block-device tree
// (lsblk produces a tree because of partitions/LVM/dm). Returns
// the first node where shouldStopAt(blk) returns true, or nil.
func WalkDisk(rootBlk model.LSBLKModel, depth uint, shouldStopAt func(blk model.LSBLKModel) bool) *model.LSBLKModel {
	if shouldStopAt(rootBlk) {
		return &rootBlk
	}

	if depth == 0 {
		return nil
	}

	for _, blkChild := range rootBlk.Children {
		if blk := WalkDisk(blkChild, depth-1, shouldStopAt); blk != nil {
			return blk
		}
	}

	return nil
}

// ParseBlockDevices parses raw `lsblk -J` JSON output into the
// LSBLKModel tree. Exported for use by the migration tool.
func ParseBlockDevices(str []byte) ([]model.LSBLKModel, error) {
	var blkList []model.LSBLKModel
	if err := json2.Unmarshal([]byte(jsoniter.Get(str, "blockdevices").ToString()), &blkList); err != nil {
		return nil, err
	}

	return blkList, nil
}
