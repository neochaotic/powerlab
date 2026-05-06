//go:build !linux && !darwin

package service

import "syscall"

// detachSysProcAttr no-op for platforms we don't ship.
func detachSysProcAttr() *syscall.SysProcAttr {
	return nil
}
