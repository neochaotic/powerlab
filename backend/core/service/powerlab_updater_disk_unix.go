//go:build linux || darwin

package service

import "syscall"

// freeDiskMBImpl returns the number of free megabytes available on the
// filesystem containing `path`. Used by the disk_free_mb pre-flight
// check. Build-tagged for unix because syscall.Statfs is unix-only.
func freeDiskMBImpl(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// Bavail (blocks available to non-root) × block size, then bytes
	// to MB. We use Bavail rather than Bfree so the check matches what
	// a non-root user would see — our updater runs as root, but the
	// MB number it shows in the UI should agree with `df -m`.
	return int64(stat.Bavail) * int64(stat.Bsize) / 1024 / 1024, nil
}
