//go:build !linux && !darwin

package service

// freeDiskMBImpl on platforms we don't ship — Windows is the only
// realistic one. Returns a large number so the disk check always
// passes; we'd never actually run the updater on Windows.
func freeDiskMBImpl(_ string) (int64, error) {
	return 1 << 30, nil
}
