package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"gopkg.in/yaml.v3"
)

// TestPrepareBindMountSources_ChmodsExistingDirs locks the Sprint 21
// PR 10 fix for the bind-mount-perms class. Apps inherited from
// Umbrel / CasaOS write to bind-mounted `/DATA/PowerLabAppData/<app>`
// from inside the container, but the container's runtime user (e.g.,
// Laravel www-data uid 33, Node uid 1000) doesn't own that path —
// PowerLab creates it as root:root with default mode. Result: app
// crashes with "Permission denied" / "Please provide a valid cache
// path" / equivalent immediately after install.
//
// Fix: at install-time, after the compose YAML is rendered to disk,
// walk every service's `volumes:` list, find bind-mount sources, and
// chmod them 0o777. Non-bind-mount volumes (named volumes, tmpfs,
// non-existent paths) are skipped silently.
func TestPrepareBindMountSources_ChmodsExistingDirs(t *testing.T) {
	tmpRoot := t.TempDir()
	dataDir := filepath.Join(tmpRoot, "data")
	cfgDir := filepath.Join(tmpRoot, "config")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config.AppInfo.StoragePath = tmpRoot // containment root

	yamlBytes := []byte(`services:
  app:
    image: nginx:latest
    volumes:
      - ` + dataDir + `:/app/data
      - ` + cfgDir + `:/app/config:rw
      - some_named_volume:/x
`)

	if err := PrepareBindMountSources(yamlBytes); err != nil {
		t.Fatalf("PrepareBindMountSources: %v", err)
	}

	st, _ := os.Stat(dataDir)
	if st.Mode().Perm() != 0o777 {
		t.Errorf("expected dataDir 0o777, got %o", st.Mode().Perm())
	}
	st, _ = os.Stat(cfgDir)
	if st.Mode().Perm() != 0o777 {
		t.Errorf("expected cfgDir 0o777, got %o", st.Mode().Perm())
	}
}

func TestPrepareBindMountSources_CreatesMissingDirs(t *testing.T) {
	// Compose may reference a bind-mount source that doesn't exist
	// yet (typical Umbrel app: data dir created lazily by first app
	// run). Create + chmod so the first container start can write.
	tmpRoot := t.TempDir()
	missing := filepath.Join(tmpRoot, "will", "not", "exist", "yet")
	config.AppInfo.StoragePath = tmpRoot // containment root
	yamlBytes := []byte(`services:
  app:
    image: nginx:latest
    volumes:
      - ` + missing + `:/app/data
`)
	if err := PrepareBindMountSources(yamlBytes); err != nil {
		t.Fatalf("PrepareBindMountSources: %v", err)
	}
	st, err := os.Stat(missing)
	if err != nil {
		t.Fatalf("expected dir created, got %v", err)
	}
	if st.Mode().Perm() != 0o777 {
		t.Errorf("expected created dir 0o777, got %o", st.Mode().Perm())
	}
}

func TestPrepareBindMountSources_SkipsNamedVolumes(t *testing.T) {
	// Named volumes (no leading slash) are NOT filesystem paths —
	// docker manages them in /var/lib/docker/volumes. Touching them
	// here would be wrong.
	tmpRoot := t.TempDir()
	yamlBytes := []byte(`services:
  app:
    image: postgres:14
    volumes:
      - pg_data:/var/lib/postgresql/data
volumes:
  pg_data:
`)
	// MUST be a no-op (no panic, no error, no dirs created under
	// tmpRoot since we passed a named-volume reference).
	if err := PrepareBindMountSources(yamlBytes); err != nil {
		t.Fatalf("PrepareBindMountSources: %v", err)
	}
	entries, _ := os.ReadDir(tmpRoot)
	if len(entries) != 0 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected tmpRoot empty, got: %s", strings.Join(names, ","))
	}
}

func TestPrepareBindMountSources_SkipsRelativePathsAndFiles(t *testing.T) {
	// Defensive: only chmod paths that are absolute AND that we created/
	// own. Relative paths inside the container, single-file binds
	// (./config.yaml), and paths starting with `.` are skipped.
	tmpRoot := t.TempDir()
	yamlBytes := []byte(`services:
  app:
    image: nginx:latest
    volumes:
      - ./local-config:/cfg
      - relative/path:/x
`)
	if err := PrepareBindMountSources(yamlBytes); err != nil {
		t.Fatalf("PrepareBindMountSources: %v", err)
	}
	entries, _ := os.ReadDir(tmpRoot)
	if len(entries) != 0 {
		t.Errorf("expected tmpRoot empty for relative paths, got %d entries", len(entries))
	}
}

func TestPrepareBindMountSources_LongFormVolumes(t *testing.T) {
	// Docker compose long-form volume entries:
	//   - type: bind
	//     source: /path/on/host
	//     target: /path/in/container
	tmpRoot := t.TempDir()
	src := filepath.Join(tmpRoot, "longform-data")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	config.AppInfo.StoragePath = tmpRoot // containment root
	yamlBytes := []byte(`services:
  app:
    image: nginx:latest
    volumes:
      - type: bind
        source: ` + src + `
        target: /app/data
`)
	if err := PrepareBindMountSources(yamlBytes); err != nil {
		t.Fatalf("PrepareBindMountSources: %v", err)
	}
	st, _ := os.Stat(src)
	if st.Mode().Perm() != 0o777 {
		t.Errorf("expected long-form bind source 0o777, got %o", st.Mode().Perm())
	}
}

func TestPrepareBindMountSources_NoopWhenNoVolumes(t *testing.T) {
	// Services without any volumes: function must do nothing and
	// return nil — no error from `volumes` key being absent.
	yamlBytes := []byte(`services:
  app:
    image: nginx:latest
`)
	if err := PrepareBindMountSources(yamlBytes); err != nil {
		t.Errorf("expected nil for no-volume case, got %v", err)
	}
}

func TestPrepareBindMountSources_MalformedYAMLReturnsError(t *testing.T) {
	if err := PrepareBindMountSources([]byte(`services:
  app: [`)); err == nil {
		t.Errorf("expected error on malformed YAML, got nil")
	}
}

// helper for tests that need to verify the YAML round-trips after
// PrepareBindMountSources (function MUST NOT mutate input bytes).
func mustParseYAML(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

func TestPrepareBindMountSources_DoesNotMutateInput(t *testing.T) {
	tmpRoot := t.TempDir()
	src := filepath.Join(tmpRoot, "data")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	yamlBytes := []byte(`services:
  app:
    image: nginx:latest
    volumes:
      - ` + src + `:/app/data
`)
	orig := string(yamlBytes)
	if err := PrepareBindMountSources(yamlBytes); err != nil {
		t.Fatalf("PrepareBindMountSources: %v", err)
	}
	if string(yamlBytes) != orig {
		t.Errorf("input bytes mutated; expected pass-through")
	}
	_ = mustParseYAML(t, yamlBytes)
}

// --- Adversarial / security: bind-source containment -------------------
//
// chmod 0o777 must NEVER touch a host path outside the app-data root.
// A malicious/compromised catalog entry (or future operator source)
// declaring a system path as a bind source must not weaken host perms.

func TestPrepareBindMountSources_RefusesPathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	// A victim dir in a DIFFERENT tree, mode 0o700 — outside the root.
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.MkdirAll(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	yamlBytes := []byte(`services:
  evil:
    image: nginx:latest
    volumes:
      - ` + victim + `:/x
`)
	if err := prepareBindMountSourcesWithinRoot(yamlBytes, root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st, _ := os.Stat(victim)
	if st.Mode().Perm() != 0o700 {
		t.Errorf("SECURITY: out-of-root path chmod'd to %o — containment failed", st.Mode().Perm())
	}
}

func TestPrepareBindMountSources_RejectsTraversalEscape(t *testing.T) {
	// `<root>/../escape` cleans to outside root → must be refused (not created).
	root := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	escape := filepath.Clean(filepath.Join(root, "..", "pwned"))
	yamlBytes := []byte(`services:
  evil:
    image: nginx:latest
    volumes:
      - ` + escape + `:/x
`)
	_ = prepareBindMountSourcesWithinRoot(yamlBytes, root)
	if _, err := os.Stat(escape); err == nil {
		t.Errorf("SECURITY: traversal-escape path %q created outside root", escape)
	}
}

func TestPathWithinRoot(t *testing.T) {
	cases := []struct {
		path, root string
		want       bool
	}{
		{"/DATA/PowerLabAppData/booklore/data", "/DATA", true},
		{"/DATA", "/DATA", true},
		{"/etc", "/DATA", false},
		{"/", "/DATA", false},
		{"/DATA/../etc/passwd", "/DATA", false}, // traversal
		{"/DATAhog/x", "/DATA", false},          // prefix-but-not-subdir boundary
		{"/anything", "", false},                // fail-closed: no root
	}
	for _, c := range cases {
		if got := pathWithinRoot(c.path, c.root); got != c.want {
			t.Errorf("pathWithinRoot(%q, %q) = %v, want %v", c.path, c.root, got, c.want)
		}
	}
}
