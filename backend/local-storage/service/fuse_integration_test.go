//go:build linux && fuse

// Phase 4 of #150 — fuse build-tag tests for local-storage. Sprint 13.5
// (carry from Sprint 11).
//
// Why a `//go:build linux && fuse` tag:
//   - The fuse mount syscall (`mount()` with `fs.type=fuse`) requires
//     a kernel with FUSE support — Linux-only. macOS dev machines
//     have macFUSE but the syscall surface differs; sharing test
//     code isn't worth the wrapper effort.
//   - GitHub-hosted Linux runners DO have FUSE kernel support, BUT
//     running an actual mount needs CAP_SYS_ADMIN — fine on direct
//     runners, broken inside unprivileged containers. The `fuse`
//     tag gates the deeper mount tests so a CI step running this
//     can also do the priv setup (apt install fuse + setuid).
//   - Default `go test ./...` does NOT pick these up — they only run
//     with `go test -tags=fuse`.
//
// This scaffold is intentionally MINIMAL: it imports bazil.org/fuse,
// instantiates the basic types, and confirms a fuse mount option
// surface compiles. The deeper tests (actual mount → write → read →
// unmount) follow in Sprint 14 once the CI privilege setup is in
// place. The wiring established here ensures the build tag works
// + the fuse dep stays a tested seam.
//
// To run locally on Linux:
//   sudo apt install fuse libfuse-dev
//   go test -tags=fuse ./service/...

package service_test

import (
	"testing"

	"bazil.org/fuse"
)

// TestFuseAPISurfaceCompiles confirms the local-storage's bazil.org/fuse
// dependency is at a version where the mount-option types we rely on
// are present. If bazil.org/fuse changes API, this fails to compile
// before any subtle runtime bug surfaces.
func TestFuseAPISurfaceCompiles(t *testing.T) {
	// MountOption is the dominant config type — every mount goes
	// through this. Instantiating one without actually mounting
	// proves the package is usable in this build environment.
	opt := fuse.AllowOther()
	if opt == nil {
		t.Fatal("fuse.AllowOther() returned nil — bazil.org/fuse API broke")
	}

	// FSName + Subtype are stable mount-option helpers we use in
	// the mergerfs equivalent code. Lock the function shape.
	if fsName := fuse.FSName("powerlab-test"); fsName == nil {
		t.Fatal("fuse.FSName returned nil")
	}
}
