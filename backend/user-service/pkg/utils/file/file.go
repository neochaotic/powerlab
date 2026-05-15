// Package file is a thin os-helpers grab-bag inherited from the
// upstream fork. Most of the original helpers (GetExt, Exists,
// ReadFullFile, CopySingleFile, WriteToPath, SaveUploadedFile, ...)
// were never wired into PowerLab and have been removed. What
// survives is the directory-create pair the SQLite bootstrap +
// the user-data-dir mkdir actually call.
package file

import "os"

// CheckNotExist reports whether src does NOT exist on disk.
// Used by IsNotExistMkDir below; not called externally on its own.
func CheckNotExist(src string) bool {
	_, err := os.Stat(src)
	return os.IsNotExist(err)
}

// IsNotExistMkDir creates the directory at src if it does not
// exist. No-op when src already exists. Used by the SQLite
// bootstrap to ensure the DB parent directory is present before
// gorm opens the file.
func IsNotExistMkDir(src string) error {
	if notExist := CheckNotExist(src); notExist {
		return MkDir(src)
	}
	return nil
}

// MkDir creates src (and any missing parents) with world-rwx
// permissions. The 0o777 chmod is deliberate — user-data
// directories are read by the docker daemon (typically root) AND
// the panel's own process; tightening this caused mount-permission
// regressions in previous releases.
func MkDir(src string) error {
	if err := os.MkdirAll(src, os.ModePerm); err != nil {
		return err
	}
	os.Chmod(src, 0o777)
	return nil
}
