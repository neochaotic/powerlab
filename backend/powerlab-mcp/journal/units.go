package journal

import (
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// ListUnits returns the powerlab-* unit names found in systemdDir,
// without the .service suffix and without the "powerlab-" prefix. That
// shape matches what an agent would pass back as `unit` in
// journal://{unit} (canonicalUnit re-applies prefix/suffix).
//
// systemdDir is normally "/etc/systemd/system" (where package-linux.sh's
// installer drops the powerlab-*.service files). A missing directory
// returns (nil, nil) — a dev box without the PowerLab installation in
// place must not fail the resource; it just reports no units.
//
// Reads the directory entries directly rather than shelling to
// `systemctl list-units` so the resource works without a running
// systemd (e.g. inside a container test, on a Mac dev box) and stays
// testable with a t.TempDir() fixture.
func ListUnits(systemdDir string) ([]string, error) {
	entries, err := os.ReadDir(systemdDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var units []string
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		// Filter to powerlab-*.service. We do NOT pick up
		// powerlab-*.timer, .target, etc. — the journal:// resource
		// only reads .service unit logs today.
		if !strings.HasPrefix(name, "powerlab-") || !strings.HasSuffix(name, ".service") {
			continue
		}
		// Strip prefix + suffix so the result matches the unit shape
		// canonicalUnit accepts back from the agent. e.g.
		// "powerlab-gateway.service" → "gateway".
		stem := strings.TrimSuffix(strings.TrimPrefix(name, "powerlab-"), ".service")
		if stem == "" {
			continue
		}
		// Some installers emit both powerlab-foo.service and a
		// symlink — dedupe by the stem.
		if seen[stem] {
			continue
		}
		seen[stem] = true
		units = append(units, stem)
	}
	sort.Strings(units)
	return units, nil
}
