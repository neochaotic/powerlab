// Package fstab is a typed reader/writer for /etc/fstab. The
// disk-management service uses it to persist mount-on-boot entries
// and to clean them up on volume delete. Backed up to .powerlab.bak
// before every mutation (was .casaos.bak before #248 — operators on
// pre-#248 installs may still have leftover .casaos.bak files;
// those are harmless backup artifacts, not consumed by code).
package fstab

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

// fsck Pass values — used in the 6th column of an fstab entry.
// PassDoNotCheck disables boot-time fsck (recommended for non-root
// filesystems); 1 + 2 enable it at root + non-root respectively.
// DefaultPath is /etc/fstab — overridable for tests.
const (
	PassDoNotCheck      = 0
	PassCheckDuringBoot = 1
	PassCheckAfterBoot  = 2

	DefaultPath = "/etc/fstab"

	// backupSuffix is appended to DefaultPath for the pre-mutation
	// backup. Operators recovering from a bad write can `cp <path>
	// .powerlab.bak <path>` to roll back. Renamed from .casaos.bak
	// per #248.
	backupSuffix = ".powerlab.bak"
	// tmpSuffix is the staging file used by RemoveByMountPoint
	// (write-to-tmp + rename for atomicity). Renamed from
	// .casaos.new per #248.
	tmpSuffix = ".powerlab.new"
	// addedMarker is the comment appended to every line PowerLab
	// writes to /etc/fstab so the operator can grep for changes.
	// Renamed from "# Added by the CasaOS" per #248.
	addedMarker = "\t# Added by PowerLab\n"
)

var (
	_fstab *FStab

	ErrInvalidFSTabEntry                     = errors.New("invalid fstab entry")
	ErrDifferentFSTabEntryWithSameMountPoint = errors.New("a different fstab entry with the same mount point already exists")
)

// Entry is one row in /etc/fstab. Fields map directly to the 6 tab-
// separated columns. FStab is the file-handle-shaped wrapper that
// exposes Add/Remove/Get* operations.
type (
	Entry struct {
		// The device name, label, UUID, or other means of specifying the partition or data source this entry refers to.
		Source string

		// Where the contents of the device may be accessed after mounting
		MountPoint string

		// The type of file system to be mounted.
		FSType string

		// Options describing various other aspects of the file system, such as whether it is automatically mounted at boot, which users may mount or access it, whether it may be written to or only read from, its size, and so forth; the special option defaults refers to a pre-determined set of options depending on the file system type.
		Options string

		// A number indicating whether and how often the file system should be backed up by the dump program; a zero indicates the file system will never be automatically backed up.
		Dump int

		// A number indicating the order in which the fsck program will check the devices for errors at boot time
		Pass int
	}

	FStab struct {
		path string
	}
)

// String renders the entry in the 6-column tab-separated format
// /etc/fstab expects. No trailing newline.
func (e *Entry) String() string {
	return e.Source + "\t" + e.MountPoint + "\t" + e.FSType + "\t" + e.Options + "\t" + strconv.Itoa(e.Dump) + "\t" + strconv.Itoa(e.Pass)
}

// Add appends e to /etc/fstab, taking a backup at .powerlab.bak
// first. If a different entry already exists at the same
// MountPoint, returns ErrDifferentFSTabEntryWithSameMountPoint
// unless replace is true (in which case the old entry is removed
// before the new one is added).
func (f *FStab) Add(e Entry, replace bool) error {
	entry, err := f.GetEntryByMountPoint(e.MountPoint)
	if err != nil {
		return err
	}

	if entry != nil {
		if !replace &&
			(entry.Source != e.Source ||
				entry.FSType != e.FSType ||
				entry.Options != e.Options ||
				entry.Dump != e.Dump ||
				entry.Pass != e.Pass) {
			return ErrDifferentFSTabEntryWithSameMountPoint
		}

		if err := f.RemoveByMountPoint(e.MountPoint, false); err != nil {
			return err
		}
	}

	if err := copy(f.path, f.path+backupSuffix); err != nil {
		return err
	}

	fstabFile, err := os.OpenFile(f.path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fstabFile.Close()

	_, err = fstabFile.WriteString(e.String() + addedMarker)
	if err != nil {
		return err
	}

	return err
}

// RemoveByMountPoint removes the entry whose MountPoint matches.
// When comment is true the line is preserved with a leading "#"
// instead of being deleted — useful for revertable changes.
func (f *FStab) RemoveByMountPoint(mountpoint string, comment bool) error {
	FStabPathNew := f.path + tmpSuffix
	FStabFileNew, err := os.OpenFile(FStabPathNew, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	if err := foreachLine(f.path, func(line string) error {
		entry, _ := parseEntry(line)
		if entry != nil && entry.MountPoint == mountpoint {
			if comment {
				_, err := FStabFileNew.WriteString("#" + line + "\n")
				return err
			}
			return nil
		}

		_, err := FStabFileNew.WriteString(line + "\n")
		return err
	}); err != nil {
		return err
	}

	if err := copy(f.path, f.path+backupSuffix); err != nil {
		return err
	}

	return os.Rename(FStabPathNew, f.path)
}

// GetEntries returns every parsed entry in /etc/fstab, skipping
// comment lines + blank lines. Malformed rows return
// ErrInvalidFSTabEntry.
func (f *FStab) GetEntries() ([]*Entry, error) {
	entries := []*Entry{}

	if err := foreachLine(f.path, func(line string) error {
		entry, err := parseEntry(line)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return entries, nil
}

// GetEntryByMountPoint returns the entry whose MountPoint matches,
// or (nil, nil) if none match.
func (f *FStab) GetEntryByMountPoint(mountpoint string) (*Entry, error) {
	entries, err := f.GetEntries()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.MountPoint == mountpoint {
			return entry, nil
		}
	}

	return nil, nil
}

// GetEntryBySource returns the entry whose Source (device path /
// UUID / label) matches, or (nil, nil) if none match.
func (f *FStab) GetEntryBySource(source string) (*Entry, error) {
	entries, err := f.GetEntries()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Source == source {
			return entry, nil
		}
	}

	return nil, nil
}

// Get returns the package-level FStab singleton bound to
// DefaultPath (/etc/fstab). Lazy-initialised on first call.
func Get() *FStab {
	if _fstab == nil {
		_fstab = &FStab{
			path: DefaultPath,
		}
	}

	return _fstab
}

func parseEntry(line string) (*Entry, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, nil
	}

	fields := strings.Fields(line)
	if len(fields) < 4 {
		return nil, nil
	}

	entry := Entry{
		Dump: 0,
		Pass: PassDoNotCheck,
	}

	entry.Source = fields[0]
	entry.MountPoint = fields[1]
	entry.FSType = fields[2]
	entry.Options = fields[3]

	if len(fields) > 4 {
		dump, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, ErrInvalidFSTabEntry
		}
		entry.Dump = dump
	}

	if len(fields) > 5 {
		pass, err := strconv.Atoi(fields[5])
		if err != nil {
			return nil, ErrInvalidFSTabEntry
		}
		entry.Pass = pass
	}

	return &entry, nil
}

func foreachLine(path string, handle func(line string) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if err := handle(line); err != nil {
			return err
		}
	}

	return nil
}

func copy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
