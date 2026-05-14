package service

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/logger"
)

func init() {
	logger.LogInitConsoleOnly()
}

func TestParseUserGroup(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantUID int
		wantGID int
		wantErr bool
	}{
		{"empty string", "", 0, 0, true},
		{"uid only", "1000", 1000, 1000, false},
		{"uid:gid numeric", "1000:2000", 1000, 2000, false},
		{"uid:gid same", "999:999", 999, 999, false},
		{"non-numeric uid", "postgres", 0, 0, true},
		{"non-numeric gid", "1000:postgres", 0, 0, true},
		{"colon-prefixed", ":1000", 0, 0, true},
		{"trailing colon", "1000:", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, gid, err := parseUserGroup(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseUserGroup(%q): want error, got uid=%d gid=%d", tt.in, uid, gid)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseUserGroup(%q): unexpected error: %v", tt.in, err)
			}
			if uid != tt.wantUID || gid != tt.wantGID {
				t.Errorf("parseUserGroup(%q) = (%d,%d), want (%d,%d)", tt.in, uid, gid, tt.wantUID, tt.wantGID)
			}
		})
	}
}

func TestChownBindMountSource_EmptyUserField(t *testing.T) {
	// Empty user field is a silent no-op — catalog didn't specify, so we
	// don't change ownership. Returns nil without error.
	tmp := t.TempDir()
	target := filepath.Join(tmp, "data")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := chownBindMountSource(target, ""); err != nil {
		t.Errorf("chownBindMountSource(empty): unexpected error: %v", err)
	}
}

func TestChownBindMountSource_NonNumericUser(t *testing.T) {
	// "postgres" refers to /etc/passwd inside the container, not the host.
	// We can't resolve it on the host, so we warn-and-continue (nil error).
	tmp := t.TempDir()
	if err := chownBindMountSource(tmp, "postgres"); err != nil {
		t.Errorf("chownBindMountSource(non-numeric): want nil (warn-and-continue), got %v", err)
	}
}

func TestChownBindMountSource_NonExistentPath(t *testing.T) {
	// Chown failure (path missing) must NOT break the install — docker
	// will surface its own error if the mount is genuinely broken.
	if err := chownBindMountSource("/does/not/exist/powerlab", "1000:1000"); err != nil {
		t.Errorf("chownBindMountSource(missing path): want nil (warn-and-continue), got %v", err)
	}
}

func TestChownBindMountSource_AppliesOwnership(t *testing.T) {
	// Skip if not root — non-root processes can only chown to their own UID,
	// and we want to verify the chown actually applies the requested values.
	if os.Geteuid() != 0 {
		t.Skip("chown to arbitrary UID requires root")
	}
	tmp := t.TempDir()
	target := filepath.Join(tmp, "db")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := chownBindMountSource(target, "999:999"); err != nil {
		t.Fatalf("chownBindMountSource: %v", err)
	}
	st, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	sys := st.Sys().(*syscall.Stat_t)
	if sys.Uid != 999 || sys.Gid != 999 {
		t.Errorf("ownership = %d:%d, want 999:999", sys.Uid, sys.Gid)
	}
}

func TestChownBindMountSource_SelfOwnership(t *testing.T) {
	// Non-root path: chown to current UID:GID. Should succeed (no-op chown).
	tmp := t.TempDir()
	target := filepath.Join(tmp, "self")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	self := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	if err := chownBindMountSource(target, self); err != nil {
		t.Errorf("chownBindMountSource(self): %v", err)
	}
}
