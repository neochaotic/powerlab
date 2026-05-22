package file

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrPathOutsideScope is returned by ResolveWithinScope when a requested
// path resolves outside the configured Files sandbox (#36).
var ErrPathOutsideScope = errors.New("path is outside the permitted file scope")

// ResolveWithinScope cleans req (collapsing `..`) and guarantees the
// result stays within scope. With an empty scope it returns the cleaned
// path unchanged (legacy whole-filesystem access, for configs that
// predate the [file] Scope setting). A request that — after resolving
// `..` — lands outside the scope root returns ErrPathOutsideScope.
//
//	ResolveWithinScope("/DATA", "/DATA/foo")           -> "/DATA/foo", nil
//	ResolveWithinScope("/DATA", "/etc/passwd")         -> "", ErrPathOutsideScope
//	ResolveWithinScope("/DATA", "/DATA/../etc/passwd") -> "", ErrPathOutsideScope
//	ResolveWithinScope("",      "/etc/passwd")         -> "/etc/passwd", nil (legacy)
func ResolveWithinScope(scope, req string) (string, error) {
	clean := filepath.Clean(req)
	if strings.TrimSpace(scope) == "" {
		return clean, nil
	}
	scope = filepath.Clean(scope)

	abs := clean
	if !filepath.IsAbs(abs) {
		abs = filepath.Clean(filepath.Join(scope, clean))
	}

	rel, err := filepath.Rel(scope, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathOutsideScope
	}
	return abs, nil
}
