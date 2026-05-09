package v2

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/IceWhaleTech/CasaOS-Common/utils"
	"github.com/IceWhaleTech/CasaOS-Common/utils/file"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/codegen"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/pkg/mergerfs"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/pkg/partition"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/pkg/utils/command"
	model2 "github.com/IceWhaleTech/CasaOS-LocalStorage/service/model"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

var (
	ErrMergeMountPointAlreadyExists  = errors.New("merge mount point already exists")
	ErrMergeMountPointDoesNotExist   = errors.New("merge mount point does not exist")
	ErrMergeMountPointSourceConflict = errors.New("source mount point should not be a child path of the merge mount point")
	ErrNilReference                  = errors.New("reference is nil")
)

// Make sure the serial disk is removed from the merge list when it is deleted from database, to keep the database consistent.
func hookAfterDeleteVolume(db *gorm.DB, model interface{}) {
	var targetVolumes []model2.Volume

	switch t := model.(type) {
	case model2.Volume:
		targetVolumes = []model2.Volume{t}
	case *model2.Volume:
		targetVolumes = []model2.Volume{*t}
	case []model2.Volume:
		targetVolumes = t
	case *[]model2.Volume:
		targetVolumes = *t
	default:
		return
	}

	var merges []model2.Merge

	if err := db.Model(&model2.Merge{}).Preload(model2.MergeSourceVolumes).Find(&merges).Error; err != nil {
		_log.Error(context.Background(), "failed to get merge list from database", err)
		return
	}

	for i := range merges {
		updatedVolumes := make([]*model2.Volume, 0)
		for _, sourceVolume := range merges[i].SourceVolumes {
			for _, targetVolume := range targetVolumes {
				if sourceVolume.ID == targetVolume.ID {
					break // skip including the volume to be deleted
				}
				updatedVolumes = append(updatedVolumes, sourceVolume)
			}
		}

		if err := db.Model(&merges[i]).Association(model2.MergeSourceVolumes).Error; err != nil {
			_log.Error(context.Background(), "failed to enter association mode between merges and volumes", err, slog.Any("merge", merges[i]))
			return
		}

		if err := db.Model(&merges[i]).Association(model2.MergeSourceVolumes).Replace(updatedVolumes); err != nil {
			_log.Error(context.Background(), "failed to update merge source volumes", err, slog.Any("merge", merges[i]), slog.Any("updatedVolumes", updatedVolumes))
			return
		}
	}
}

func (s *LocalStorageService) GetMerges(mountPoint *string) ([]model2.Merge, error) {
	mergesFromDB, err := s.GetMergeAllFromDB(mountPoint)
	if err != nil {
		return nil, err
	}

	for _, merge := range mergesFromDB {
		merge.SourceVolumes = excludeVolumesWithWrongMountPointAndUUID(merge.SourceVolumes)
	}

	return mergesFromDB, nil
}

func (s *LocalStorageService) CreateMerge(merge *model2.Merge) error {
	ctx := context.Background()
	if merge == nil {
		_log.Error(ctx, "`merge` should not be nil", nil)
		return ErrNilReference
	}

	if err := file.IsNotExistMkDir(merge.MountPoint); err != nil {
		return err
	}

	merge.SourceVolumes = excludeVolumesWithWrongMountPointAndUUID(merge.SourceVolumes)

	sources, err := buildSources(merge)
	if err != nil {
		_log.Error(ctx, "failed to build sources", err)
		return err
	}

	// check if the mount point is empty before creating a new mergerfs mount
	if bool, err := file.IsDirEmpty(merge.MountPoint); err != nil {
		_log.Error(ctx, "failed to check if the mount point is empty", err)
		return err
	} else if !bool {
		_log.Error(ctx, "mount point is not empty", nil, slog.String("mountPoint", merge.MountPoint))
		return ErrMountPointIsNotEmpty
	}

	// create a new merge by mounting mergerfs
	source := strings.Join(sources, ":")
	if _, err := s.Mount(codegen.Mount{
		MountPoint: merge.MountPoint,
		Fstype:     &merge.FSType,
		Source:     &source,
	}); err != nil {
		_log.Error(ctx, "failed to mount mergerfs", err, slog.String("mountPoint", merge.MountPoint), slog.String("source", source))
		return err
	}

	return nil
}

func (s *LocalStorageService) UpdateMerge(merge *model2.Merge) error {
	ctx := context.Background()
	if merge == nil {
		_log.Error(ctx, "`merge` should not be nil", nil)
		return ErrNilReference
	}

	if !file.Exists(merge.MountPoint) {
		return ErrMergeMountPointDoesNotExist
	}

	merge.SourceVolumes = excludeVolumesWithWrongMountPointAndUUID(merge.SourceVolumes)

	sources, err := buildSources(merge)
	if err != nil {
		_log.Error(ctx, "failed to build sources", err)
		return err
	}

	// if it is already a merge point, check if the mount point is a mergerfs mount with the same sources
	existingSources, err := mergerfs.GetSource(merge.MountPoint)
	if err != nil {
		_log.Error(ctx, "failed to get mergerfs sources", err, slog.String("mountPoint", merge.MountPoint))
		return err
	}

	if !utils.CompareStringSlices(sources, existingSources) {
		// update the mergerfs sources if different sources
		if err := mergerfs.SetSource(merge.MountPoint, sources); err != nil {
			_log.Error(ctx, "failed to set mergerfs sources", err, slog.String("mountPoint", merge.MountPoint), slog.Any("sources", sources))
			return err
		}
	}

	return nil
}

func (s *LocalStorageService) CheckMergeMount() {
	ctx := context.Background()

	mergesFromDB, err := s.GetMergeAllFromDB(nil)
	if err != nil {
		_log.Error(ctx, "failed to get merge list from database", err)
		return
	}

	mounts, err := s.GetMounts(codegen.GetMountsParams{})
	if err != nil {
		_log.Error(ctx, "failed to get mount list from system", err)
		return
	}

	for i := range mergesFromDB {

		isMergeExist := false

		// for each merge from database by mount point, check if it already mounted, i.e. a mergerfs mount
		for _, mount := range mounts {
			if mount.MountPoint == mergesFromDB[i].MountPoint {
				if *mount.Fstype == mergesFromDB[i].FSType {
					_log.Info(ctx, "merge already exists", slog.Any("merge", mergesFromDB[i]))
					isMergeExist = true
					break
				}
				_log.Error(ctx, "not a mergerfs mount point", nil, slog.Any("mount", mount))
			}
		}

		if isMergeExist {
			if err := s.UpdateMerge(&mergesFromDB[i]); err != nil {
				_log.Error(ctx, "failed to update merge", err, slog.Any("merge", mergesFromDB[i]))
			}
			continue
		} else {
			if err := s.CreateMerge(&mergesFromDB[i]); err != nil {
				_log.Error(ctx, "failed to create merge", err, slog.Any("merge", mergesFromDB[i]))
			}
		}
	}
}

// filter out any volume that are not mounted based on its UUID and mount point (in reality, could have a different disk mounted on the same path)
func excludeVolumesWithWrongMountPointAndUUID(volumes []*model2.Volume) []*model2.Volume {
	ctx := context.Background()
	return filterVolumes(volumes, func(v *model2.Volume) bool {
		path, err := partition.GetDevicePath(v.UUID)
		if err != nil {
			_log.Error(ctx, "failed to corresponding device path by volume UUID", err, slog.String("uuid", v.UUID))
			return false
		}

		par := command.ExecLSBLKByPath(path)
		pttype := gjson.GetBytes(par, "blockdevices.0.pttype")
		if pttype.String() != "gpt" {
			mountPoint := gjson.GetBytes(par, "blockdevices.0.mountpoint")
			if mountPoint.String() != v.MountPoint {
				_log.Error(ctx, "mount point does not match actual", nil, slog.Any("volume", v), slog.String("actual mount point", mountPoint.String()))
				return false
			}
			return true

		}

		partitions, err := partition.GetPartitions(path)
		if err != nil {
			_log.Error(ctx, "failed to corresponding partition of volume", err, slog.String("path", path))
			return false
		}

		if len(partitions) != 1 {
			_log.Error(ctx, "there should be exactly one partition corresponding to the volume", nil, slog.String("path", path), slog.Int("partitions", len(partitions)))
			return false
		}

		if partitions[0].LSBLKProperties["MOUNTPOINT"] != v.MountPoint {
			_log.Error(ctx, "mount point does not match actual", nil, slog.Any("volume", v), slog.String("actual mount point", partitions[0].LSBLKProperties["MOUNTPOINT"]))
			return false
		}

		return true
	})
}

func filterVolumes(volumes []*model2.Volume, filter func(*model2.Volume) bool) []*model2.Volume {
	var filteredVolumes []*model2.Volume
	for _, volume := range volumes {
		result := filter(volume)
		if result {
			filteredVolumes = append(filteredVolumes, volume)
		}
	}
	return filteredVolumes
}

func buildSources(merge *model2.Merge) ([]string, error) {
	ctx := context.Background()
	sources := make([]string, 0)

	if merge.SourceBasePath != nil && *merge.SourceBasePath != "" {
		// check if sourceBasePath is under mount point
		if strings.HasPrefix(*merge.SourceBasePath, merge.MountPoint) {
			_log.Error(
				ctx,
				"source base path should not be a child path of the merge mount point",
				nil,
				slog.String("sourceBasePath", *merge.SourceBasePath),
				slog.String("merge.MountPoint", merge.MountPoint),
			)
			return nil, ErrMergeMountPointSourceConflict
		}

		// create source path if it does not exists
		if err := file.IsNotExistMkDir(*merge.SourceBasePath); err != nil {
			return nil, err
		}

		sources = append(sources, *merge.SourceBasePath)
	}

	for _, sourceVolume := range merge.SourceVolumes {
		if sourceVolume == nil {
			_log.Error(ctx, "one of the source volumes is nil", nil, slog.Any("sourceVolumes", merge.SourceVolumes))
			return nil, ErrNilReference
		}

		// check if sourceBasePath is under mount point
		if strings.HasPrefix(sourceVolume.MountPoint, merge.MountPoint) {
			_log.Error(
				ctx,
				"mount point of source volume should not be a child path of the mount point",
				nil,
				slog.Any("sourceVolume.MountPoint", sourceVolume.MountPoint),
				slog.Any("merge.MountPoint", merge.MountPoint),
			)
			return nil, ErrMergeMountPointSourceConflict
		}

		sources = append(sources, sourceVolume.MountPoint)
	}

	return sources, nil
}
