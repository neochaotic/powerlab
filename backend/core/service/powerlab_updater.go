package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// PowerLabUpdater coordinates the in-UI update flow described in
// docs/UPDATE_MANIFEST.md. Phase 2 of issue #21 — the HTTP layer wires
// it up; Phase 4 will plug snapshot + rollback into the install path.
//
// The updater is intentionally a pile of small functions instead of a
// big interface: each step (fetch manifest, verify checksum, run a
// preflight check, swap binaries) maps to one function we can unit-test
// in isolation. The integration only happens at `RunInstall`, which is
// where rollback semantics matter and where we'll wire the snapshot
// machinery later.
type PowerLabUpdater struct {
	// CurrentVersion is the version stamp baked into THIS running
	// binary at compile time. Set by main on construction; tests
	// override.
	CurrentVersion string

	// ManifestURL is the canonical URL the updater fetches to learn
	// what's available. In production: GitHub's releases/latest
	// redirect. Tests override to a local fixture server.
	ManifestURL string

	// HTTPClient is exposed so tests can inject a fake. Defaults
	// to http.DefaultClient with a 30 s timeout.
	HTTPClient *http.Client

	// PowerLabRoot is the directory the updater treats as the install
	// target. /var/lib/powerlab in production. Tests override to a
	// temp dir so disk-free / app-state checks don't probe the real
	// system. Empty string falls back to /var/lib/powerlab.
	PowerLabRoot string
}

// NewPowerLabUpdater returns an updater wired to production defaults.
func NewPowerLabUpdater(currentVersion string) *PowerLabUpdater {
	return &PowerLabUpdater{
		CurrentVersion: currentVersion,
		ManifestURL:    "https://github.com/neochaotic/powerlab/releases/latest/download/manifest.json",
		HTTPClient:     &http.Client{Timeout: 30 * time.Second},
		PowerLabRoot:   "/var/lib/powerlab",
	}
}

// Manifest mirrors the structure documented in docs/UPDATE_MANIFEST.md.
// Unknown fields are preserved silently — see the forward-compatibility
// note at the end of that doc.
type Manifest struct {
	Version          string                  `json:"version"`
	ReleasedAt       string                  `json:"released_at"`
	MinUpgradeFrom   string                  `json:"min_upgrade_from"`
	SkipRelease      bool                    `json:"skip_release"`
	Summary          string                  `json:"summary"`
	ChangelogURL     string                  `json:"changelog_url"`
	Tarball          map[string]TarballEntry `json:"tarball"`
	BreakingChanges  []map[string]any        `json:"breaking_changes"`
	PreInstallChecks []map[string]any        `json:"pre_install_checks"`
	DBMigrations     []map[string]any        `json:"db_migrations"`
}

type TarballEntry struct {
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

// CheckResult is what `Check()` returns. Decision is one of:
//
//	up_to_date     — host already runs the manifest's version
//	update_ok      — newer version available, host can upgrade directly
//	too_old        — manifest's min_upgrade_from is greater than current
//	skipped        — manifest has skip_release: true
//	no_arch        — manifest does not ship a tarball for runtime.GOARCH
type CheckResult struct {
	Current        string    `json:"current"`
	Available      string    `json:"available,omitempty"`
	Decision       string    `json:"decision"`
	ReleaseSummary string    `json:"release_summary,omitempty"`
	ChangelogURL   string    `json:"changelog_url,omitempty"`
	Manifest       *Manifest `json:"manifest,omitempty"`
}

// Check fetches the latest release manifest and compares it to the
// running version. Returns a structured decision the caller can render
// directly in the UI.
func (u *PowerLabUpdater) Check(ctx context.Context) (*CheckResult, error) {
	m, err := u.fetchManifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}

	res := &CheckResult{
		Current:        u.CurrentVersion,
		Available:      m.Version,
		ReleaseSummary: m.Summary,
		ChangelogURL:   m.ChangelogURL,
		Manifest:       m,
	}

	switch {
	case m.SkipRelease:
		res.Decision = "skipped"
	case m.Version == u.CurrentVersion:
		res.Decision = "up_to_date"
	case compareSemver(u.CurrentVersion, m.MinUpgradeFrom) < 0:
		// Cannot upgrade directly — there's an intermediate version
		// the user has to pass through first. Surface it in the UI.
		res.Decision = "too_old"
	default:
		// Verify the host's arch is in the tarball list. If not, the
		// release just doesn't ship our build target and we can't
		// proceed — the maintainer would have to publish an arch
		// patch.
		if _, ok := m.Tarball[runtime.GOARCH]; !ok {
			res.Decision = "no_arch"
		} else {
			res.Decision = "update_ok"
		}
	}

	return res, nil
}

// PreflightCheck represents the result of one item in the manifest's
// pre_install_checks list.
type PreflightCheck struct {
	Kind     string `json:"kind"`     // canonical name, e.g. "disk_free_mb"
	Status   string `json:"status"`   // "pass" | "warn" | "fail"
	Message  string `json:"message"`  // short user-facing line
}

// RunPreflight evaluates each entry in `m.PreInstallChecks` and returns
// the result. Unknown kinds default to "warn" (safety bias — admin
// gets a yellow chevron rather than a green tick on something the
// updater doesn't understand).
func (u *PowerLabUpdater) RunPreflight(m *Manifest) []PreflightCheck {
	if m == nil || len(m.PreInstallChecks) == 0 {
		return nil
	}
	out := make([]PreflightCheck, 0, len(m.PreInstallChecks))
	for _, c := range m.PreInstallChecks {
		kind, _ := c["kind"].(string)
		switch kind {
		case "disk_free_mb":
			out = append(out, u.checkDiskFreeMB(c))
		case "docker_healthy":
			out = append(out, u.checkDockerHealthy())
		case "no_apps_unhealthy":
			out = append(out, u.checkNoAppsUnhealthy())
		case "no_active_install_task":
			out = append(out, u.checkNoActiveInstallTask())
		default:
			out = append(out, PreflightCheck{
				Kind:    kind,
				Status:  "warn",
				Message: fmt.Sprintf("Unknown check kind %q — your gateway may be older than the release manifest expects.", kind),
			})
		}
	}
	return out
}

func (u *PowerLabUpdater) checkDiskFreeMB(args map[string]any) PreflightCheck {
	path, _ := args["path"].(string)
	if path == "" {
		path = u.powerLabRoot()
	}
	minMB := 0
	switch v := args["min"].(type) {
	case float64:
		minMB = int(v)
	case int:
		minMB = v
	}

	free, err := freeDiskMB(path)
	if err != nil {
		return PreflightCheck{
			Kind:    "disk_free_mb",
			Status:  "warn",
			Message: fmt.Sprintf("Could not check disk space at %s (%v) — proceed at your own risk.", path, err),
		}
	}
	if free < int64(minMB) {
		return PreflightCheck{
			Kind:    "disk_free_mb",
			Status:  "fail",
			Message: fmt.Sprintf("Only %d MB free on %s; %d MB required. Free up space, then re-check.", free, path, minMB),
		}
	}
	return PreflightCheck{
		Kind:    "disk_free_mb",
		Status:  "pass",
		Message: fmt.Sprintf("%d MB free on %s (need %d MB).", free, path, minMB),
	}
}

func (u *PowerLabUpdater) checkDockerHealthy() PreflightCheck {
	// `docker info --format {{.ServerVersion}}` exits 0 on a
	// reachable daemon, non-zero otherwise. Cheap and standard.
	cmd := exec.Command("docker", "info", "--format", "{{.ServerVersion}}")
	out, err := cmd.Output()
	if err != nil {
		return PreflightCheck{
			Kind:    "docker_healthy",
			Status:  "fail",
			Message: "Docker daemon not responding. Apps will not run after upgrade — start Docker first.",
		}
	}
	return PreflightCheck{
		Kind:    "docker_healthy",
		Status:  "pass",
		Message: "Docker " + strings.TrimSpace(string(out)) + " responding.",
	}
}

func (u *PowerLabUpdater) checkNoAppsUnhealthy() PreflightCheck {
	// `docker ps --filter health=unhealthy --format {{.Names}}` lists
	// any container the daemon currently considers unhealthy. We do
	// not block on these — just warn, because the user might still
	// want to upgrade past a flapping app.
	cmd := exec.Command("docker", "ps", "--filter", "health=unhealthy", "--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		// Docker not installed / not running. The earlier
		// docker_healthy check already failed; this one degrades to a
		// warn so we don't double-complain.
		return PreflightCheck{
			Kind:    "no_apps_unhealthy",
			Status:  "warn",
			Message: "Could not query container health — Docker may be down.",
		}
	}
	names := strings.TrimSpace(string(out))
	if names == "" {
		return PreflightCheck{
			Kind:    "no_apps_unhealthy",
			Status:  "pass",
			Message: "No unhealthy containers.",
		}
	}
	return PreflightCheck{
		Kind:    "no_apps_unhealthy",
		Status:  "warn",
		Message: "Unhealthy containers: " + strings.ReplaceAll(names, "\n", ", ") + " — they will not be touched by the upgrade, but might be in a bad state.",
	}
}

func (u *PowerLabUpdater) checkNoActiveInstallTask() PreflightCheck {
	// Apps in mid-install live under $AppsPath/<id>/.installing or
	// have a docker-compose.yml referencing an image that isn't
	// pulled yet. The simplest signal is: any directory under
	// $AppsPath that has a `.installing` marker file. The
	// app-management service writes that during the install
	// goroutine.
	root := filepath.Join(u.powerLabRoot(), "apps")
	entries, err := os.ReadDir(root)
	if err != nil {
		// No apps directory yet — fresh install or the path is
		// elsewhere; either way, no active task.
		return PreflightCheck{
			Kind:    "no_active_install_task",
			Status:  "pass",
			Message: "No app installs in progress.",
		}
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), ".installing")); err == nil {
			return PreflightCheck{
				Kind:    "no_active_install_task",
				Status:  "fail",
				Message: "An app install is in progress (" + e.Name() + "). Wait for it to finish, then re-check.",
			}
		}
	}
	return PreflightCheck{
		Kind:    "no_active_install_task",
		Status:  "pass",
		Message: "No app installs in progress.",
	}
}

// fetchManifest downloads + parses the manifest JSON.
func (u *PowerLabUpdater) fetchManifest(ctx context.Context) (*Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.ManifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if !looksLikeSemver(m.Version) {
		return nil, fmt.Errorf("manifest version %q is not semver", m.Version)
	}
	return &m, nil
}

// VerifyTarballSHA256 streams a tarball through SHA-256 and compares
// to the expected hex digest. Used by the install path before extract.
func VerifyTarballSHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, expected)
	}
	return nil
}

// RunInstall is the install entrypoint — Phase 2 stub. Phase 4 lands
// the real snapshot + binary-swap + rollback flow. For now we refuse
// the request with a clear "not implemented" error so the route
// returns 501 instead of pretending to do something.
func (u *PowerLabUpdater) RunInstall(ctx context.Context, m *Manifest) error {
	return errors.New("install path is not implemented yet — Phase 4 of #21")
}

// ── Helpers ─────────────────────────────────────────────────────────────

func (u *PowerLabUpdater) client() *http.Client {
	if u.HTTPClient != nil {
		return u.HTTPClient
	}
	return http.DefaultClient
}

func (u *PowerLabUpdater) powerLabRoot() string {
	if u.PowerLabRoot != "" {
		return u.PowerLabRoot
	}
	return "/var/lib/powerlab"
}

// compareSemver returns -1 / 0 / 1 like strings.Compare. Tolerates a
// missing pre-release suffix (`0.2.4-rc.1` is treated as `0.2.4` for
// the purpose of compatibility floors — pre-releases share their
// MAJOR.MINOR.PATCH with the GA they precede). Empty strings sort
// before everything (so a missing min_upgrade_from never blocks).
func compareSemver(a, b string) int {
	if a == "" || b == "" {
		if a == b {
			return 0
		}
		if a == "" {
			return -1
		}
		return 1
	}
	pa := strings.Split(strings.SplitN(a, "-", 2)[0], ".")
	pb := strings.Split(strings.SplitN(b, "-", 2)[0], ".")
	for i := 0; i < 3; i++ {
		var ai, bi int
		if i < len(pa) {
			ai, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			bi, _ = strconv.Atoi(pb[i])
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

// looksLikeSemver mirrors the structural check in scripts/build-manifest.
func looksLikeSemver(v string) bool {
	if v == "" || strings.HasPrefix(v, "v") {
		return false
	}
	core := strings.Split(strings.SplitN(v, "-", 2)[0], ".")
	if len(core) != 3 {
		return false
	}
	for _, p := range core {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// freeDiskMB is split out so tests can stub the syscall. The
// implementation lives in powerlab_updater_disk_*.go (per-platform
// build-tagged) so we don't drag syscall.Statfs onto Darwin tests.
var freeDiskMB = freeDiskMBImpl
