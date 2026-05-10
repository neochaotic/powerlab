// Package mergerfs is a typed Go wrapper around the mergerfs control
// file (the magic ".mergerfs" extended-attribute interface mergerfs
// exposes inside every mounted union). All operations boil down to
// listxattr/getxattr/setxattr on that file.
//
// Used by the disk-management service to add/remove branches from
// the /DATA mergerfs union as disks are inserted + ejected.
package mergerfs

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"syscall"
)

// ControlFile returns the path of the .mergerfs control file inside
// the union mounted at fspath.
func ControlFile(fspath string) string {
	return filepath.Join(fspath, ".mergerfs")
}

// ListValues returns every user.mergerfs.* xattr key/value pair on
// the union's control file. Used by the admin UI's "merge config"
// inspector.
func ListValues(fspath string) (map[string]string, error) {
	ctrlfile := ControlFile(fspath)

	buf := make([]byte, 4096)
	size, err := syscall.Listxattr(ctrlfile, buf)
	if err != nil {
		return nil, err
	}

	buf = buf[:size]

	values := make(map[string]string)
	for _, keyBuf := range bytes.Split(buf, []byte{0}) {
		if len(keyBuf) == 0 {
			continue
		}
		key := string(keyBuf)
		value := make([]byte, 512)
		size, err := syscall.Getxattr(ctrlfile, key, value)
		if err != nil {
			return nil, err
		}
		value = value[:size]
		values[key] = string(value)
	}

	return values, nil
}

// SetSource replaces the union's branch list with sources. Dedupes
// to avoid bouncing the union when the same branch is passed twice.
func SetSource(fspath string, sources []string) error {
	ctrlfile := ControlFile(fspath)

	key := "user.mergerfs.branches"

	sourceMap := make(map[string]interface{})
	for _, source := range sources {
		sourceMap[source] = true
	}

	dedupedSources := make([]string, 0)
	for source := range sourceMap {
		dedupedSources = append(dedupedSources, source)
	}

	value := []byte(strings.Join(dedupedSources, ":"))
	//str, err := command.ExecResultStr("setfattr -n " + key + " -v " + string(string(value)) + " " + ctrlfile)
	err := syscall.Setxattr(ctrlfile, key, value, 0)
	//_log.Error(context.Background(), "SetSourceStr", nil, slog.String("str", str))
	if err != nil {
		_log.Error(context.Background(), "SetSource", err)
		return err
	}
	return err
}

// GetSource returns the union's current source branch list as
// reported by the user.mergerfs.srcmounts xattr.
func GetSource(fspath string) ([]string, error) {
	values, err := ListValues(fspath)
	if err != nil {
		return nil, err
	}

	return strings.Split(values["user.mergerfs.srcmounts"], ":"), nil
}

// AddSource adds source to the union's branch list. Mergerfs's
// "+source" syntax means "append, don't replace".
func AddSource(fspath string, source string) error {
	ctrlfile := ControlFile(fspath)

	key := "user.mergerfs.branches"
	value := []byte("+" + source)

	return syscall.Setxattr(ctrlfile, key, value, 0)
}

// RemoveSource removes source from the union's branch list via
// mergerfs's "-source" syntax.
func RemoveSource(fspath string, source string) error {
	ctrlfile := ControlFile(fspath)

	key := "user.mergerfs.branches"
	value := []byte("-" + source)

	return syscall.Setxattr(ctrlfile, key, value, 0)
}

// AddPath is the legacy alias for AddSource — kept for callers
// that import the older API name.
func AddPath(fspath string, path string) error {
	ctrlfile := ControlFile(fspath)
	return AddSource(ctrlfile, path)
}

// RemovePath is the legacy alias for RemoveSource.
func RemovePath(fspath string, path string) error {
	ctrlfile := ControlFile(fspath)
	return RemoveSource(ctrlfile, path)
}
