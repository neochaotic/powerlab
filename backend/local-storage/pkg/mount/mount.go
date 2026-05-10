// Package mount wraps the system `mount`/`umount` CLIs and exposes
// the FUSE filesystem implementation (rclone-vfs backed) used by
// remote-storage drivers. The CLI wrappers are thin — they exist so
// callers see Go errors instead of having to parse mount(8) exit
// codes.
package mount

import "github.com/neochaotic/powerlab/backend/local-storage/pkg/utils/command"

// Mount shells out to /bin/mount with the given source, mountpoint,
// optional fstype + options. The --verbose flag stays on so any
// error includes the kernel's reason.
func Mount(source string, mountpoint string, fstype *string, options *string) error {
	args := []string{"--verbose"}

	if fstype != nil && *fstype != "" {
		args = append(args, "-t", *fstype)
	}

	if options != nil && *options != "" {
		args = append(args, "-o", *options)
	}

	args = append(args, source, mountpoint)

	if _, err := command.ExecuteCommand("mount", args...); err != nil {
		return err
	}

	return nil
}

// UmountByMountPoint force-unmounts the filesystem at mountpoint.
// --force tolerates a busy fs; --quiet swallows lazy-unmount noise.
func UmountByMountPoint(mountpoint string) error {
	if _, err := command.ExecuteCommand("umount", "--force", "--verbose", "--quiet", mountpoint); err != nil {
		return err
	}

	return nil
}

// UmountByDevice unmounts every mount point backed by the given
// block device. --recursive walks the device's mount tree so a
// device with sub-mounts is fully detached.
func UmountByDevice(device string) error {
	if _, err := command.ExecuteCommand("umount", "--force", "--verbose", "--quiet", "--recursive", device); err != nil {
		return err
	}

	return nil
}
