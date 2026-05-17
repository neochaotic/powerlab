package service

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

// SeedBindMountsFromImage pre-populates empty bind-mount source dirs
// with the image's own content at the same target path. Solves the
// "bind-mount overlay" class — Laravel apps need
// `storage/framework/{cache,views,sessions}` to exist, but an empty
// host bind-mount source overlays the image's pre-populated content
// and the app crashes. Same for any image that ships skeleton files
// at its data path (nextcloud, wordpress, jellyfin's /config, etc).
//
// Algorithm per bind-mount:
//
//  1. Skip non-bind volumes (named volumes, tmpfs, relative paths).
//  2. Skip if host source dir is NOT empty (re-install, user data).
//  3. `docker create <image>` → temporary container, no run
//  4. `docker cp <container>:<target> <host_source>/.` → copy
//     image content into host bind-mount source
//  5. `docker rm <container>` → cleanup, regardless of cp outcome
//
// Best-effort: any docker CLI failure is logged + swallowed. The
// install proceeds; docker compose up will surface its own error if
// the missing seed actually breaks the app. This is the same posture
// as Sprint 21's SubstituteHostPlaceholders + PrepareBindMountSources.
//
// What this is NOT:
//   - Not running any upstream hooks or exports.sh (per ADR-0038)
//   - Not requiring sandbox infrastructure (the docker commands run
//     in PowerLab's own privilege context, but they're PowerLab-
//     authored Go calling docker CLI — not upstream bash)
//   - Not overwriting existing data (empty-check guard)
type dockerRunFunc func(args ...string) (stdout string, err error)

// SeedBindMountsFromImage is the production entry point — wires the
// real docker CLI runner. Tests use SeedBindMountsFromImageWithRunner
// with a fake.
func SeedBindMountsFromImage(yamlBytes []byte) error {
	return SeedBindMountsFromImageWithRunner(yamlBytes, runDockerCLI)
}

// SeedBindMountsFromImageWithRunner is the testable variant — the
// caller injects the docker runner so unit tests can record + replay
// without a real docker daemon.
func SeedBindMountsFromImageWithRunner(yamlBytes []byte, run dockerRunFunc) error {
	var doc map[string]any
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		return fmt.Errorf("parse compose: %w", err)
	}
	services, ok := doc["services"].(map[string]any)
	if !ok {
		return nil
	}
	for _, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		image, _ := svc["image"].(string)
		if image == "" {
			continue
		}
		vols, ok := svc["volumes"].([]any)
		if !ok {
			continue
		}
		for _, v := range vols {
			src, target := extractBindSourceAndTarget(v)
			if src == "" || target == "" {
				continue
			}
			if !isDirEmpty(src) {
				continue
			}
			seedOne(run, image, src, target)
		}
	}
	return nil
}

// extractBindSourceAndTarget returns the host source + container
// target of a bind-mount volume entry. Empty/empty when the entry
// is not a bind (named volume, tmpfs, relative-path, single-file).
//
// Short-form: `/host/path:/container/path[:mode]`
// Long-form:  `{type: bind, source: /host, target: /container}`
func extractBindSourceAndTarget(v any) (src, target string) {
	switch vv := v.(type) {
	case string:
		parts := strings.SplitN(vv, ":", 3)
		if len(parts) < 2 {
			return "", ""
		}
		if !strings.HasPrefix(parts[0], "/") {
			return "", ""
		}
		return parts[0], parts[1]
	case map[string]any:
		t, _ := vv["type"].(string)
		if t != "" && t != "bind" {
			return "", ""
		}
		s, _ := vv["source"].(string)
		tg, _ := vv["target"].(string)
		if !strings.HasPrefix(s, "/") || tg == "" {
			return "", ""
		}
		return s, tg
	}
	return "", ""
}

// isDirEmpty returns true when path doesn't exist OR is a dir with
// zero entries. Any other case (file, dir-with-content, permission
// error) returns false — caller treats false as "skip seeding".
func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		// dir missing → counts as empty (caller will create via mkdir
		// before docker cp; but we're called after PrepareBindMountSources
		// so the dir already exists with mode 0o777)
		if os.IsNotExist(err) {
			return true
		}
		return false
	}
	return len(entries) == 0
}

// seedOne runs the docker create → cp → rm sequence for one bind-
// mount. Best-effort: errors are silently swallowed. The temp
// container is always rm'd (deferred) even when cp fails — avoids
// leaking stopped containers on the host.
func seedOne(run dockerRunFunc, image, hostSrc, containerTarget string) {
	out, err := run("create", image)
	if err != nil {
		// Image not pullable / daemon down / etc — best-effort skip.
		return
	}
	cid := strings.TrimSpace(out)
	if cid == "" {
		return
	}
	defer func() {
		_, _ = run("rm", cid)
	}()
	// `docker cp <cid>:<target> <host>/.` — the `/.` suffix copies
	// dir CONTENT (not the dir itself) into the host source. Without
	// it docker would create a sub-dir named after the source basename.
	_, _ = run("cp", cid+":"+containerTarget, hostSrc+"/.")
}

// runDockerCLI is the real-world dockerRunFunc. Shells out to
// `docker <args>` and returns stdout + error.
func runDockerCLI(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	out, err := cmd.Output()
	return string(out), err
}
