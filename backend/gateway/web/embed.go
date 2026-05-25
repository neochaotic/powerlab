// Package web embeds the built static UI bundle into the gateway
// binary (ADR-0043). The build/ directory holds a committed
// placeholder index.html so the package always compiles; the release
// build (scripts/package-linux.sh) overwrites build/ with the real
// ui/build output before `go build` of the gateway.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var assets embed.FS

// FS returns the embedded UI bundle rooted at the build directory,
// ready to hand to http.FS. In a checkout that has not had the real
// UI staged into build/, this contains only the placeholder
// index.html — enough to compile and boot, not the shipped UI.
func FS() fs.FS {
	sub, err := fs.Sub(assets, "build")
	if err != nil {
		// Unreachable: build/ is embedded at compile time, so the
		// sub-tree always exists. Panic rather than silently serve
		// an empty FS.
		panic("gateway/web: embedded build/ subtree missing: " + err.Error())
	}
	return sub
}
