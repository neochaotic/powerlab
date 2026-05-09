package merge

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

func IsMergerFSInstalled() bool {
	paths := []string{
		"/sbin/mount.mergerfs", "/usr/sbin/mount.mergerfs", "/usr/local/sbin/mount.mergerfs",
		"/bin/mount.mergerfs", "/usr/bin/mount.mergerfs", "/usr/local/bin/mount.mergerfs",
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			_log.Info(context.Background(), "mergerfs is installed", slog.String("path", path))
			return true
		}
	}

	_log.Error(context.Background(), "mergerfs is not installed at any path", nil, slog.String("paths", strings.Join(paths, ", ")))
	return false
}
