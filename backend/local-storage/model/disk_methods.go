package model

import (
	"path/filepath"

	"github.com/neochaotic/powerlab/backend/common/utils/file"
)

const defaultMountPath = "/media"

func (m *LSBLKModel) GetMountPoint(name string) string {
	if name == "" {
		name = "Storage_" + m.Name
	}

	if m.Label != "" {
		name += "_" + m.Label
	}

	if m.Model != "" {
		name += "_" + m.Model
	}
	mountPoint := filepath.Join(defaultMountPath, name)
	if file.CheckNotExist(mountPoint) {
		return mountPoint
	}
	return mountPoint + "_" + m.Name
}
