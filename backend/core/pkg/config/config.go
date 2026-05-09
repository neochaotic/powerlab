package config

import (
	"path/filepath"

	"github.com/neochaotic/powerlab/backend/common/utils/constants"
)

// CoreConfigFilePath is the in-binary default config location used when
// the binary is started without `-c`. In production install.sh ships the
// sample to this exact path AND systemd passes the same path via `-c`,
// so the constants.DefaultConfigPath base must agree with both.
// Resolved per-platform via constants.DefaultConfigPath:
//
//	Linux  → /etc/powerlab/core.conf
//	darwin → /opt/powerlab/etc/core.conf
//	dev    → <repo>/backend/conf/core.conf  (sandbox)
//
// Sprint 3 Phase 3 rename: was `CasaOSConfigFilePath` pointing at
// `casaos.conf`, which disagreed with the systemd unit's `-c
// /etc/powerlab/core.conf`. The disagreement made install.sh ship a
// sample the binary never read.
var CoreConfigFilePath = filepath.Join(constants.DefaultConfigPath, "core.conf")
