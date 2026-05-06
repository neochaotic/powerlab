//go:build linux || darwin

package service

import "syscall"

// detachSysProcAttr returns the SysProcAttr that places the spawned
// install.sh in its own session, so SIGTERM aimed at the parent
// (`systemctl stop powerlab-core` from inside install.sh) does NOT
// propagate to the upgrade script.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
