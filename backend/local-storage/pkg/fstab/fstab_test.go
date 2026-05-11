package fstab

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

const fstabContent = `
	# UNCONFIGURED FSTAB FOR BASE SYSTEM
	LABEL=UEFI      /boot/efi       vfat    umask=0077      0 1
	/mnt/sdb:/mnt/sdc       /media  mergerfs        defaults,allow_other,category.create=mfs,moveonenospc=true,minfreespace=1M 0 0
	LABEL=desktop-rootfs    /               ext4    defaults        0 1
`

func TestFSTab(t *testing.T) {
	fstab := &FStab{path: "/tmp/fstab"}

	err := os.WriteFile(fstab.path, []byte(fstabContent), 0o600)
	assert.NilError(t, err)

	entries, err := fstab.GetEntries()
	assert.NilError(t, err)

	assert.Equal(t, len(entries), 3)

	entry, err := fstab.GetEntryByMountPoint("/media")
	assert.NilError(t, err)

	assert.Equal(t, entry.Source, "/mnt/sdb:/mnt/sdc")
	assert.Equal(t, entry.MountPoint, "/media")
	assert.Equal(t, entry.FSType, "mergerfs")
	assert.Equal(t, entry.Options, "defaults,allow_other,category.create=mfs,moveonenospc=true,minfreespace=1M")
	assert.Equal(t, entry.Dump, 0)
	assert.Equal(t, entry.Pass, PassDoNotCheck)

	err = fstab.RemoveByMountPoint(entry.MountPoint, false)
	assert.NilError(t, err)

	nonExistingEntry, err := fstab.GetEntryByMountPoint(entry.MountPoint)
	assert.NilError(t, err)
	assert.Equal(t, nonExistingEntry, (*Entry)(nil))

	err = fstab.Add(*entry, true)
	assert.NilError(t, err)

	entry, err = fstab.GetEntryByMountPoint(entry.MountPoint)
	assert.NilError(t, err)

	assert.Equal(t, entry.Source, "/mnt/sdb:/mnt/sdc")
	assert.Equal(t, entry.MountPoint, "/media")
	assert.Equal(t, entry.FSType, "mergerfs")
	assert.Equal(t, entry.Options, "defaults,allow_other,category.create=mfs,moveonenospc=true,minfreespace=1M")
	assert.Equal(t, entry.Dump, 0)
	assert.Equal(t, entry.Pass, PassDoNotCheck)
}

// Closes #248 — fstab writes used to scatter ".casaos.bak" /
// ".casaos.new" backup files into the operator's /etc/ tree on
// every volume add, plus a "# Added by the CasaOS" marker line.
// Surprises operators (especially co-resident installs migrating
// from CasaOS where these names overlap real CasaOS-written
// files). Now uses powerlab-branded names + marker.
func TestFSTab_BackupFilesAndMarkerAreBranded(t *testing.T) {
	fstabPath := "/tmp/fstab-brand-test"
	bakPath := fstabPath + ".powerlab.bak"
	casaosBakPath := fstabPath + ".casaos.bak"

	// Clean slate.
	_ = os.Remove(bakPath)
	_ = os.Remove(casaosBakPath)

	err := os.WriteFile(fstabPath, []byte(fstabContent), 0o600)
	assert.NilError(t, err)
	defer os.Remove(fstabPath)
	defer os.Remove(bakPath)

	fstab := &FStab{path: fstabPath}

	entry := Entry{
		Source:     "/dev/sdz1",
		MountPoint: "/mnt/brand-test",
		FSType:     "ext4",
		Options:    "defaults",
	}
	err = fstab.Add(entry, false)
	assert.NilError(t, err)

	// Backup file must be powerlab-branded, NOT casaos-branded.
	_, errPowerlab := os.Stat(bakPath)
	assert.NilError(t, errPowerlab, ".powerlab.bak backup must exist after Add")
	_, errCasaos := os.Stat(casaosBakPath)
	assert.Assert(t, errCasaos != nil, ".casaos.bak must NOT be written — #248 regression check")

	// The marker comment on the appended line is powerlab-branded.
	contents, err := os.ReadFile(fstabPath)
	assert.NilError(t, err)
	body := string(contents)
	assert.Assert(t, contains(body, "# Added by PowerLab"),
		"marker comment must be 'Added by PowerLab', got: %q", body)
	assert.Assert(t, !contains(body, "Added by the CasaOS"),
		"legacy 'Added by the CasaOS' marker must not regress")
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
