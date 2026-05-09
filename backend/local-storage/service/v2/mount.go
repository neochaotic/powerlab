package v2

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/local-storage/codegen"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/mount"

	"github.com/neochaotic/powerlab/backend/local-storage/service/v2/fs"
	"github.com/moby/sys/mountinfo"
)

var (
	ErrNotMounted           = errors.New("not mounted")
	ErrAlreadyMounted       = errors.New("volume is already mounted")
	ErrMountPointIsNotEmpty = errors.New("mountpoint is not empty")
)

func (s *LocalStorageService) GetMounts(params codegen.GetMountsParams) ([]codegen.Mount, error) {
	mounts, err := s._mountinfo.GetMounts(func(i *mountinfo.Info) (skip bool, stop bool) {
		if params.Id != nil {
			if strconv.Itoa(i.ID) != *params.Id {
				return true, false
			}
		}
		if params.MountPoint != nil {
			if i.Mountpoint != *params.MountPoint {
				return true, false
			}
		}
		if params.Type != nil {
			if i.FSType != *params.Type {
				return true, false
			}
		}
		if params.Source != nil {
			if i.Source != *params.Source {
				return true, false
			}
		}
		return false, false
	})
	if err != nil {
		_log.Error(context.Background(), "Error when trying to get mounted volume(s)", err)
		return nil, err
	}

	results := make([]codegen.Mount, len(mounts))

	for i, mountInfo := range mounts {
		results[i] = *fs.ExtendAll(MountAdapter(mountInfo))
	}

	return results, nil
}

func (s *LocalStorageService) Mount(m codegen.Mount) (*codegen.Mount, error) {
	ctx := context.Background()
	m = *fs.PreMountAll(m)

	// check if mountpoint is already mounted
	results, err := s.GetMounts(codegen.GetMountsParams{
		MountPoint: &m.MountPoint,
		Type:       m.Fstype,
	})
	if err != nil {
		_log.Error(ctx, "Error when trying to get mounted volume", err, slog.Any("mount", m))
		return nil, err
	}

	if len(results) > 0 {
		_log.Info(ctx, "Volume is already mounted", slog.Any("mount", results[0]))
		return &results[0], ErrAlreadyMounted
	}

	// check if mountpoint is empty
	//_log.Info(ctx, "checking if mount point exist", slog.String("mount point", m.MountPoint))
	if empty, err := file.IsDirEmpty(m.MountPoint); err != nil {
		_log.Error(ctx, "error when trying to check if mount point is empty", err, slog.Any("mount", m))
		return nil, err
	} else if !empty {
		_log.Error(ctx, "mount point is not empty", nil, slog.Any("mount", m))
		return nil, ErrMountPointIsNotEmpty
	}

	if err := mount.Mount(*m.Source, m.MountPoint, m.Fstype, m.Options); err != nil {
		_log.Error(ctx, "error when trying to mount", err, slog.Any("mount", m))
		return nil, err
	}

	results, err = s.GetMounts(codegen.GetMountsParams{
		MountPoint: &m.MountPoint,
		Type:       m.Fstype,
	})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	if len(results) > 1 {
		_log.Error(ctx, "More than one mount with same mount point and fstype found", nil, slog.Any("mounts", results))
	}

	results[0] = *fs.PostMountAll(results[0])

	return &results[0], nil
}

func (s *LocalStorageService) Umount(mountpoint string) error {
	ctx := context.Background()
	// check if mountpoint is already mounted
	results, err := s.GetMounts(codegen.GetMountsParams{
		MountPoint: &mountpoint,
	})
	if err != nil {
		_log.Error(ctx, "Error when trying to get mounted volume", err, slog.String("mount point", mountpoint))
		return err
	}

	if len(results) == 0 {
		_log.Info(ctx, "not mounted", slog.String("mount point", mountpoint))
		return ErrNotMounted
	}

	if err := mount.UmountByMountPoint(mountpoint); err != nil {
		_log.Error(ctx, "error when trying to umount by mount point", err, slog.String("mount point", mountpoint))
		return err
	}

	return nil
}

func MountAdapter(m *mountinfo.Info) codegen.Mount {
	return codegen.Mount{
		MountPoint: m.Mountpoint,

		Id:      &m.ID,
		Options: &m.Options,
		Source:  &m.Source,
		Fstype:  &m.FSType,
	}
}
