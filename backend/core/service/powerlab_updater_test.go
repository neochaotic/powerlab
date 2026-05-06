package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixtureManifest produces a minimal-but-valid Manifest JSON with the
// right shape for the host's runtime arch. Tests pass a tweak callback
// to mutate fields they care about.
func fixtureManifest(version string, tweak func(*Manifest)) []byte {
	m := Manifest{
		Version:        version,
		ReleasedAt:     "2026-05-06T20:00:00Z",
		MinUpgradeFrom: "0.1.0",
		Summary:        "Test release",
		ChangelogURL:   "https://example.invalid/CHANGELOG.md",
		Tarball: map[string]TarballEntry{
			runtime.GOARCH: {
				URL:       "https://example.invalid/p.tar.gz",
				SHA256:    strings.Repeat("a", 64),
				SizeBytes: 100,
			},
		},
		BreakingChanges:  []map[string]any{},
		PreInstallChecks: []map[string]any{},
		DBMigrations:     []map[string]any{},
	}
	if tweak != nil {
		tweak(&m)
	}
	b, _ := json.Marshal(m)
	return b
}

// manifestServer spins up a local HTTP server that serves a fixture
// manifest. Returns the server (caller must Close) and the URL to
// pass into PowerLabUpdater.ManifestURL.
func manifestServer(t *testing.T, body []byte, status int) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, srv.URL
}

func TestCheck_UpToDate(t *testing.T) {
	body := fixtureManifest("0.2.4", nil)
	_, url := manifestServer(t, body, 200)
	u := &PowerLabUpdater{
		CurrentVersion: "0.2.4",
		ManifestURL:    url,
		HTTPClient:     http.DefaultClient,
	}
	res, err := u.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Decision != "up_to_date" {
		t.Fatalf("decision = %q, want up_to_date", res.Decision)
	}
}

func TestCheck_UpdateOK(t *testing.T) {
	body := fixtureManifest("0.2.4", nil)
	_, url := manifestServer(t, body, 200)
	u := &PowerLabUpdater{
		CurrentVersion: "0.2.3",
		ManifestURL:    url,
		HTTPClient:     http.DefaultClient,
	}
	res, err := u.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Decision != "update_ok" {
		t.Fatalf("decision = %q, want update_ok", res.Decision)
	}
	if res.Available != "0.2.4" {
		t.Fatalf("available = %q", res.Available)
	}
}

func TestCheck_TooOld(t *testing.T) {
	body := fixtureManifest("0.5.0", func(m *Manifest) {
		m.MinUpgradeFrom = "0.3.0"
	})
	_, url := manifestServer(t, body, 200)
	u := &PowerLabUpdater{
		CurrentVersion: "0.2.4",
		ManifestURL:    url,
		HTTPClient:     http.DefaultClient,
	}
	res, err := u.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Decision != "too_old" {
		t.Fatalf("decision = %q, want too_old (current %q < min %q)",
			res.Decision, u.CurrentVersion, "0.3.0")
	}
}

func TestCheck_SkipRelease(t *testing.T) {
	body := fixtureManifest("0.2.4", func(m *Manifest) {
		m.SkipRelease = true
	})
	_, url := manifestServer(t, body, 200)
	u := &PowerLabUpdater{
		CurrentVersion: "0.2.3",
		ManifestURL:    url,
		HTTPClient:     http.DefaultClient,
	}
	res, _ := u.Check(context.Background())
	if res.Decision != "skipped" {
		t.Fatalf("decision = %q, want skipped", res.Decision)
	}
}

func TestCheck_NoArch(t *testing.T) {
	body := fixtureManifest("0.2.4", func(m *Manifest) {
		// Empty the tarball map — manifest exists but ships no arch
		// the host can use.
		m.Tarball = map[string]TarballEntry{
			"riscv64": {URL: "https://example.invalid/p.tar.gz", SHA256: strings.Repeat("a", 64), SizeBytes: 1},
		}
	})
	_, url := manifestServer(t, body, 200)
	u := &PowerLabUpdater{
		CurrentVersion: "0.2.3",
		ManifestURL:    url,
		HTTPClient:     http.DefaultClient,
	}
	res, _ := u.Check(context.Background())
	if res.Decision != "no_arch" {
		t.Fatalf("decision = %q, want no_arch", res.Decision)
	}
}

func TestCheck_HTTPError(t *testing.T) {
	_, url := manifestServer(t, []byte("not found"), 404)
	u := &PowerLabUpdater{
		CurrentVersion: "0.2.3",
		ManifestURL:    url,
		HTTPClient:     http.DefaultClient,
	}
	if _, err := u.Check(context.Background()); err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
}

func TestCheck_InvalidVersion(t *testing.T) {
	body := fixtureManifest("not-a-semver", nil)
	_, url := manifestServer(t, body, 200)
	u := &PowerLabUpdater{
		CurrentVersion: "0.2.3",
		ManifestURL:    url,
		HTTPClient:     http.DefaultClient,
	}
	if _, err := u.Check(context.Background()); err == nil {
		t.Fatal("expected error on invalid version, got nil")
	}
}

// compareSemver is the boundary that decides too_old. Pin the
// boundaries directly — we already had a regression where a poorly-
// implemented compare let v0.1.5 upgrade to v0.5.0 in spite of the
// floor.
func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.2.4", "0.2.4", 0},
		{"0.2.3", "0.2.4", -1},
		{"0.2.4", "0.2.3", 1},
		{"0.10.0", "0.9.99", 1}, // numeric, not lex
		{"1.0.0", "0.99.99", 1},
		{"0.2.4-rc.1", "0.2.4", 0}, // pre-release shares MMP for floor purposes
		{"", "0.2.0", -1},          // missing min_upgrade_from accepts everything
		{"0.2.0", "", 1},
	}
	for _, c := range cases {
		got := compareSemver(c.a, c.b)
		// Normalise to -1/0/1 ignoring magnitude.
		sign := func(n int) int {
			if n < 0 {
				return -1
			}
			if n > 0 {
				return 1
			}
			return 0
		}
		if sign(got) != c.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// VerifyTarballSHA256 underwrites the security model — a wrong digest
// means the host accepts a tampered tarball. Pin the happy path and
// the tampered path explicitly.
func TestVerifyTarballSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	const content = "powerlab-test"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte(content))
	good := hex.EncodeToString(h[:])

	if err := VerifyTarballSHA256(path, good); err != nil {
		t.Fatalf("good digest rejected: %v", err)
	}

	// One byte off should reject.
	bad := strings.Replace(good, good[:1], "0", 1)
	if good[:1] == "0" {
		bad = strings.Replace(good, good[:1], "1", 1)
	}
	if err := VerifyTarballSHA256(path, bad); err == nil {
		t.Fatal("tampered digest accepted — security regression!")
	}
}

// RunPreflight: unknown kinds default to warn (safety bias).
func TestRunPreflight_UnknownKind(t *testing.T) {
	u := &PowerLabUpdater{}
	m := &Manifest{
		PreInstallChecks: []map[string]any{
			{"kind": "made_up_kind", "extra": 42},
		},
	}
	out := u.RunPreflight(m)
	if len(out) != 1 {
		t.Fatalf("got %d checks, want 1", len(out))
	}
	if out[0].Status != "warn" {
		t.Fatalf("unknown kind got %q, want warn (safety bias)", out[0].Status)
	}
}

// disk_free_mb pass / fail boundaries. We swap the freeDiskMB var for
// a controlled stub.
func TestRunPreflight_DiskFreeMB(t *testing.T) {
	orig := freeDiskMB
	defer func() { freeDiskMB = orig }()

	freeDiskMB = func(_ string) (int64, error) { return 1000, nil }
	u := &PowerLabUpdater{}

	cases := []struct {
		minMB  int
		want   string
	}{
		{500, "pass"},  // 1000 free, 500 needed
		{1000, "pass"}, // exact match — still passes
		{1500, "fail"}, // 1000 free, 1500 needed
	}
	for _, c := range cases {
		m := &Manifest{
			PreInstallChecks: []map[string]any{
				{"kind": "disk_free_mb", "path": "/", "min": c.minMB},
			},
		}
		got := u.RunPreflight(m)[0].Status
		if got != c.want {
			t.Errorf("disk_free_mb min=%d got %q, want %q", c.minMB, got, c.want)
		}
	}
}

// no_active_install_task: the check looks for a `.installing` marker
// under <root>/apps/*. Set up a fake tree and verify pass / fail.
func TestRunPreflight_NoActiveInstallTask(t *testing.T) {
	root := t.TempDir()
	apps := filepath.Join(root, "apps")
	if err := os.MkdirAll(filepath.Join(apps, "syncthing"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := &PowerLabUpdater{PowerLabRoot: root}

	m := &Manifest{PreInstallChecks: []map[string]any{{"kind": "no_active_install_task"}}}
	if got := u.RunPreflight(m)[0].Status; got != "pass" {
		t.Fatalf("clean tree got %q, want pass", got)
	}

	// Drop a .installing marker — should now fail.
	if err := os.WriteFile(filepath.Join(apps, "syncthing", ".installing"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := u.RunPreflight(m)[0].Status; got != "fail" {
		t.Fatalf("with .installing got %q, want fail", got)
	}
}

func TestRunInstall_NotImplemented(t *testing.T) {
	u := &PowerLabUpdater{}
	if err := u.RunInstall(context.Background(), &Manifest{}); err == nil {
		t.Fatal("RunInstall should refuse with a clear error in Phase 2")
	}
}
