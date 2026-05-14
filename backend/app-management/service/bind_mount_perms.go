package service

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"go.uber.org/zap"
)

// chownBindMountSource ensures a host bind-mount source directory is owned
// by the UID:GID the container expects, so non-root container processes
// (postgres UID 999, blinko UID 1000, etc.) can write to it on first init.
//
// Why this matters (#334): docker auto-creates missing bind-mount source
// directories as root:root. Postgres' entrypoint then runs `chmod 0700`
// on /var/lib/postgresql/data, which only the owner or root may do —
// so postgres-as-999 hits "Operation not permitted" and the init loop
// restarts forever.
//
// Behaviour:
//   - Empty userField: silent no-op (catalog author didn't specify).
//   - Non-numeric userField (e.g. "postgres"): warn-and-continue —
//     names refer to the container's /etc/passwd, not the host.
//   - chown failure: warn-and-continue — don't break the install,
//     let docker surface its own error if the mount is genuinely broken.
func chownBindMountSource(path, userField string) error {
	if userField == "" {
		return nil
	}
	uid, gid, err := parseUserGroup(userField)
	if err != nil {
		logger.Info("non-numeric user field; skipping host chown",
			zap.String("path", path),
			zap.String("user", userField))
		return nil
	}
	if err := os.Chown(path, uid, gid); err != nil {
		logger.Info("failed to chown bind-mount source; container may fail on init",
			zap.String("path", path),
			zap.Int("uid", uid),
			zap.Int("gid", gid),
			zap.Error(err))
		return nil
	}
	return nil
}

// parseUserGroup parses a compose `user:` value into numeric UID/GID.
// Accepts "UID" (gid defaults to uid) or "UID:GID". Returns an error
// for empty, non-numeric, or malformed values.
func parseUserGroup(s string) (int, int, error) {
	if s == "" {
		return 0, 0, fmt.Errorf("empty user field")
	}
	parts := strings.SplitN(s, ":", 2)
	if parts[0] == "" {
		return 0, 0, fmt.Errorf("missing uid")
	}
	uid, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("uid not numeric: %q", parts[0])
	}
	gid := uid
	if len(parts) == 2 {
		if parts[1] == "" {
			return 0, 0, fmt.Errorf("missing gid after colon")
		}
		gid, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("gid not numeric: %q", parts[1])
		}
	}
	return uid, gid, nil
}
