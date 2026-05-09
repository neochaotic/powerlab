package v2

import (
	"context"
	"log/slog"

	"github.com/neochaotic/powerlab/backend/local-storage/codegen"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/fstab"
)

func (s *LocalStorageService) SaveToFStab(m codegen.Mount) error {
	ft := fstab.Get()

	if err := ft.Add(fstab.Entry{
		MountPoint: m.MountPoint,

		Source:  *m.Source,
		FSType:  *m.Fstype,
		Options: *m.Options,
		Dump:    0,
		Pass:    fstab.PassDoNotCheck,
	}, true); err != nil {
		_log.Error(context.Background(), "Error when trying to persist mount", err, slog.Any("mount", m))
		return err
	}
	return nil
}

func (s *LocalStorageService) RemoveFromFStab(mountpoint string) error {
	ft := fstab.Get()

	if err := ft.RemoveByMountPoint(mountpoint, false); err != nil {
		_log.Error(context.Background(), "Error when trying to unpersist mount", err, slog.String("mount point", mountpoint))
		return err
	}
	return nil
}
