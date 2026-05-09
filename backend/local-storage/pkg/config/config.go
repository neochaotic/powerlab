package config

import (
	"path/filepath"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

// LocalStorageConfigFilePath is the in-binary default config location used
// when the binary is started without `-c`. In production install.sh ships
// the sample to this exact path AND systemd passes the same path via
// `-c`, so the constants.DefaultConfigPath base must agree with both.
// Resolved per-platform via constants.DefaultConfigPath:
//
//	Linux  → /etc/powerlab/local-storage.conf
//	darwin → not shipped (no fuse on darwin)
//	dev    → <repo>/backend/conf/local-storage.conf  (sandbox)
var LocalStorageConfigFilePath = filepath.Join(constants.DefaultConfigPath, "local-storage.conf")
