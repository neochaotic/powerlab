package config

import (
	"path/filepath"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

// MessageBusConfigFilePath is the in-binary default config location used
// when the binary is started without `-c`. In production install.sh ships
// the sample to this exact path AND systemd passes the same path via
// `-c`, so the constants.DefaultConfigPath base must agree with both.
// Resolved per-platform via constants.DefaultConfigPath:
//
//	Linux  → /etc/powerlab/message-bus.conf
//	darwin → /opt/powerlab/etc/message-bus.conf
//	dev    → <repo>/backend/conf/message-bus.conf  (sandbox)
var MessageBusConfigFilePath = filepath.Join(constants.DefaultConfigPath, "message-bus.conf")
