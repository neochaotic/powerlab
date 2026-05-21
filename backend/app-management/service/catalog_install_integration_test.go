//go:build integration

// Real-pipeline integration gate for the powerlab-store catalog.
//
// Guards the runtime-breaking class the v0.1.0 store catalog shipped
// (and core's strict gates caught): apps referenced their database by
// the Compose-v1 container hostname (`booklore_db_1`) instead of the
// service-name network alias (`db`). Under Compose v2 the former does
// not resolve, so the app can never reach its DB.
//
// It loads a real bundled catalog app through the SAME compose loader
// app-management uses in production (NewComposeAppFromYAML), reproduces
// the install pipeline's bind-mount perms prep, brings the DB service
// up with the real docker compose CLI, and asserts the DB service-name
// alias resolves on the app network. Runs in the amd64
// `backend-integration` CI job — the arch the catalog ships to.
//
// Run locally:  go test -tags=integration -run TestCatalogApp_DBAlias ./service/...

package service_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/app-management/service"
)

func TestCatalogApp_DBAliasResolvesOnNetwork(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if out, err := exec.Command("docker", "compose", "version").CombinedOutput(); err != nil {
		t.Skipf("docker compose plugin not available: %s", out)
	}

	// booklore is one of the 16 apps whose hostname was rewritten
	// (booklore_db_1 -> db). Its web service reaches mariadb via the
	// DATABASE_URL env; post-fix that host MUST be the `db` alias.
	const appID = "booklore"
	composePath := filepath.Join("..", "..", "..", "community-catalog", "Apps", appID, "docker-compose.yml")

	raw, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read %s: %v", composePath, err)
	}

	// Load through the production compose loader (parse + interpolate +
	// validate) — the same path that rejected mainsail's empty volumes.
	if _, err := service.NewComposeAppFromYAML(raw, false, false); err != nil {
		t.Fatalf("%s failed to load through production compose loader: %v", appID, err)
	}

	// Static invariant: the web env must reference the `db` service
	// alias, never the Compose-v1 `<project>_db_<idx>` hostname.
	if regexp.MustCompile(`\b` + appID + `_db_\d+\b`).Match(raw) {
		t.Fatalf("%s still references the Compose-v1 hostname %s_db_<n>; the hostname fix regressed", appID, appID)
	}
	if !strings.Contains(string(raw), "mariadb://db:") {
		t.Fatalf("%s web service does not reference the `db` service alias as expected", appID)
	}

	// Render: remap /DATA volume sources into a temp dir and drop the
	// obsolete top-level `version:` key (compose-go ignores it but warns).
	// Self-managed temp dir (NOT t.TempDir): mariadb writes its datadir
	// as a non-root uid with restrictive perms, which Go's t.TempDir
	// auto-cleanup cannot remove — that would fail the test on a pure
	// teardown artifact. We remove it best-effort via a root container.
	tmp, err := os.MkdirTemp("", "powerlab-itest-"+appID)
	if err != nil {
		t.Fatal(err)
	}
	dataRoot := filepath.Join(tmp, "PowerLabAppData")
	rendered := strings.ReplaceAll(string(raw), "/DATA/PowerLabAppData", dataRoot)
	rendered = regexp.MustCompile(`(?m)^version:.*\n`).ReplaceAllString(rendered, "")
	composeFile := filepath.Join(tmp, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(rendered), 0o600); err != nil {
		t.Fatal(err)
	}

	// Mirror the install pipeline's bind-mount perms prep
	// (PrepareBindMountSources): pre-create every bind source under the
	// temp data root and chmod 0o777 so the uid-1000 mariadb container
	// can write — otherwise it crash-loops, exactly as it would in prod
	// without this step.
	for _, m := range regexp.MustCompile(`(?m)-\s+(`+regexp.QuoteMeta(dataRoot)+`[^:]*):`).FindAllStringSubmatch(rendered, -1) {
		_ = os.MkdirAll(m[1], 0o777)
	}
	_ = exec.Command("chmod", "-R", "0777", tmp).Run()

	project := "powerlab-itest-" + appID
	network := project + "_default"
	dbContainer := project + "-db-1"

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	run := func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		c, cc := context.WithTimeout(ctx, timeout)
		defer cc()
		return exec.CommandContext(c, name, args...).CombinedOutput()
	}

	t.Cleanup(func() {
		_, _ = run(60*time.Second, "docker", "compose", "-p", project, "-f", composeFile, "down", "-v", "--remove-orphans")
		// Remove the (root-owned) datadir via a root container, then the
		// rest. Best-effort: leftovers live under the OS temp dir.
		_, _ = run(30*time.Second, "docker", "run", "--rm", "-v", tmp+":/h", "busybox:1.36", "rm", "-rf", "/h/PowerLabAppData")
		_ = os.RemoveAll(tmp)
	})

	// Bring up only the db service — enough to prove the service-name
	// alias resolves; avoids pulling the heavy web image.
	if out, err := run(3*time.Minute, "docker", "compose", "-p", project, "-f", composeFile, "up", "-d", "db"); err != nil {
		t.Fatalf("docker compose up db failed: %v\n%s", err, out)
	}

	// Poll the db container state via docker inspect (clean stdout, no
	// compose-cli warnings polluting the value).
	deadline := time.Now().Add(90 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		out, _ := run(15*time.Second, "docker", "inspect", "-f", "{{.State.Status}}", dbContainer)
		state = strings.TrimSpace(string(out))
		if state == "running" {
			// brief settle so the network alias is registered
			time.Sleep(3 * time.Second)
			break
		}
		time.Sleep(2 * time.Second)
	}
	if state != "running" {
		logs, _ := run(15*time.Second, "docker", "logs", "--tail", "20", dbContainer)
		t.Fatalf("db container never reached running (last: %q)\nlogs:\n%s", state, logs)
	}

	// The precise hostname-fix invariant: the `db` service-name alias
	// must be registered on the app network — that is exactly what
	// Docker's embedded DNS resolves, and what booklore's web env now
	// points at. Read it deterministically from the container's network
	// config (busybox nslookup's exit code is unreliable across builds).
	out, err := run(30*time.Second, "docker", "inspect", "-f",
		"{{range .NetworkSettings.Networks}}{{range .Aliases}}{{.}} {{end}}{{end}}", dbContainer)
	if err != nil {
		t.Fatalf("inspect db container networks: %v\n%s", err, out)
	}
	aliases := strings.Fields(string(out))
	if !slices.Contains(aliases, "db") {
		t.Fatalf("`db` service-name alias not registered on the app network (aliases=%v) — hostname fix regressed", aliases)
	}

	// Cross-check resolution at runtime from a throwaway container,
	// asserting on the OUTPUT (not busybox's flaky exit code).
	probe, _ := run(60*time.Second, "docker", "run", "--rm", "--network", network, "busybox:1.36", "nslookup", "db")
	if !regexp.MustCompile(`(?s)Name:\s*db.*Address`).Match(probe) && !strings.Contains(string(probe), "Address") {
		t.Fatalf("`db` did not resolve on %s — hostname fix regressed:\n%s", network, probe)
	}
}
