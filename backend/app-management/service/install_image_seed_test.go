package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDockerRunner is a testable replacement for the dockerCLI runner
// the real SeedBindMountsFromImage uses. Records calls + replays
// canned output / errors.
type fakeDockerRunner struct {
	calls   []string
	stdout  map[string]string // keyed by joined args
	stderr  map[string]string
	failOn  map[string]error
	cpCalls []cpCall // record cp src -> dst (with content side-effect)
}

type cpCall struct {
	containerTarget string // e.g. "abc123:/var/www/html/storage"
	hostDest        string
}

func (f *fakeDockerRunner) run(args ...string) (stdout string, err error) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if e, ok := f.failOn[key]; ok {
		return f.stderr[key], e
	}
	// Record cp calls specifically for inspection
	if len(args) >= 3 && args[0] == "cp" {
		f.cpCalls = append(f.cpCalls, cpCall{
			containerTarget: args[1],
			hostDest:        args[2],
		})
	}
	if s, ok := f.stdout[key]; ok {
		return s, nil
	}
	return "", nil
}

func TestSeedBindMountsFromImage_EmptySourceGetsSeeded(t *testing.T) {
	tmp := t.TempDir()
	emptySrc := filepath.Join(tmp, "data")
	if err := os.MkdirAll(emptySrc, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlBytes := []byte(`services:
  app:
    image: solidtime/solidtime:0.12.1
    volumes:
      - ` + emptySrc + `:/var/www/html/storage
`)

	fake := &fakeDockerRunner{
		stdout: map[string]string{
			"create solidtime/solidtime:0.12.1": "abc123\n",
		},
	}

	if err := SeedBindMountsFromImageWithRunner(yamlBytes, fake.run); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Expect: docker create, docker cp <id>:<target> <host_src>/, docker rm
	gotCreate := false
	gotCp := false
	gotRm := false
	for _, c := range fake.calls {
		if strings.HasPrefix(c, "create ") {
			gotCreate = true
		}
		if strings.HasPrefix(c, "cp ") {
			gotCp = true
		}
		if strings.HasPrefix(c, "rm ") {
			gotRm = true
		}
	}
	if !gotCreate || !gotCp || !gotRm {
		t.Errorf("expected docker create+cp+rm sequence, got calls:\n%s", strings.Join(fake.calls, "\n"))
	}
	if len(fake.cpCalls) != 1 {
		t.Errorf("expected exactly 1 cp call, got %d", len(fake.cpCalls))
	}
	if len(fake.cpCalls) > 0 {
		// cp source should be `<container_id>:<container_target>`
		want := "abc123:/var/www/html/storage"
		if fake.cpCalls[0].containerTarget != want {
			t.Errorf("cp source: got %q, want %q", fake.cpCalls[0].containerTarget, want)
		}
		// cp dest should be the host bind-mount source dir
		if fake.cpCalls[0].hostDest != emptySrc+"/." {
			t.Errorf("cp dest: got %q, want %q (with /. for dir-content copy)", fake.cpCalls[0].hostDest, emptySrc+"/.")
		}
	}
}

func TestSeedBindMountsFromImage_NonEmptySourceIsLeftAlone(t *testing.T) {
	// Bind-mount source already has files (re-install, user data,
	// previous install). MUST NOT overwrite — would destroy data.
	tmp := t.TempDir()
	src := filepath.Join(tmp, "data")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "existing.txt"), []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlBytes := []byte(`services:
  app:
    image: solidtime/solidtime:0.12.1
    volumes:
      - ` + src + `:/var/www/html/storage
`)

	fake := &fakeDockerRunner{}
	if err := SeedBindMountsFromImageWithRunner(yamlBytes, fake.run); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if len(fake.calls) != 0 {
		t.Errorf("expected zero docker calls when source non-empty, got: %s", strings.Join(fake.calls, "\n"))
	}
}

func TestSeedBindMountsFromImage_NamedVolumesSkipped(t *testing.T) {
	yamlBytes := []byte(`services:
  app:
    image: postgres:14
    volumes:
      - pg_data:/var/lib/postgresql/data
volumes:
  pg_data:
`)
	fake := &fakeDockerRunner{}
	if err := SeedBindMountsFromImageWithRunner(yamlBytes, fake.run); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Errorf("expected zero docker calls for named volumes, got: %s", strings.Join(fake.calls, "\n"))
	}
}

func TestSeedBindMountsFromImage_DockerCreateFailureIsBestEffort(t *testing.T) {
	// docker create failure (image not pullable, network issue, etc.)
	// must not block the install — log + skip, let docker compose up
	// surface its own error.
	tmp := t.TempDir()
	src := filepath.Join(tmp, "data")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlBytes := []byte(`services:
  app:
    image: missing/image:1.0
    volumes:
      - ` + src + `:/data
`)

	fake := &fakeDockerRunner{
		failOn: map[string]error{
			"create missing/image:1.0": errors.New("docker: Error response from daemon: pull access denied"),
		},
	}

	// MUST NOT return error — best-effort transform
	if err := SeedBindMountsFromImageWithRunner(yamlBytes, fake.run); err != nil {
		t.Errorf("expected nil error on docker create failure (best-effort), got: %v", err)
	}
}

func TestSeedBindMountsFromImage_HandlesLongFormVolumes(t *testing.T) {
	// Docker compose long-form bind mount:
	//   - type: bind
	//     source: /path
	//     target: /container/path
	tmp := t.TempDir()
	src := filepath.Join(tmp, "data")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlBytes := []byte(`services:
  app:
    image: nginx:latest
    volumes:
      - type: bind
        source: ` + src + `
        target: /usr/share/nginx/html
`)

	fake := &fakeDockerRunner{
		stdout: map[string]string{
			"create nginx:latest": "xyz789\n",
		},
	}
	if err := SeedBindMountsFromImageWithRunner(yamlBytes, fake.run); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if len(fake.cpCalls) != 1 {
		t.Errorf("expected 1 cp call for long-form bind, got %d (calls: %v)", len(fake.cpCalls), fake.calls)
	}
	if len(fake.cpCalls) > 0 && fake.cpCalls[0].containerTarget != "xyz789:/usr/share/nginx/html" {
		t.Errorf("cp source: got %q, want xyz789:/usr/share/nginx/html", fake.cpCalls[0].containerTarget)
	}
}

func TestSeedBindMountsFromImage_NoopWhenNoBindMounts(t *testing.T) {
	yamlBytes := []byte(`services:
  app:
    image: nginx:latest
`)
	fake := &fakeDockerRunner{}
	if err := SeedBindMountsFromImageWithRunner(yamlBytes, fake.run); err != nil {
		t.Errorf("expected nil for no-volume case, got %v", err)
	}
	if len(fake.calls) != 0 {
		t.Errorf("expected zero docker calls, got %d", len(fake.calls))
	}
}

func TestSeedBindMountsFromImage_MalformedYAMLReturnsError(t *testing.T) {
	fake := &fakeDockerRunner{}
	if err := SeedBindMountsFromImageWithRunner([]byte("services:\n  app: ["), fake.run); err == nil {
		t.Errorf("expected error on malformed YAML")
	}
}

func TestSeedBindMountsFromImage_RmContainerEvenOnCpFailure(t *testing.T) {
	// If docker cp fails (target missing in image, permission issue),
	// the temp container must still be `docker rm`'d — otherwise we
	// leak stopped containers on the host.
	tmp := t.TempDir()
	src := filepath.Join(tmp, "data")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlBytes := []byte(`services:
  app:
    image: scratch:latest
    volumes:
      - ` + src + `:/nonexistent/target
`)

	fake := &fakeDockerRunner{
		stdout: map[string]string{
			"create scratch:latest": "leak123\n",
		},
		failOn: map[string]error{
			"cp leak123:/nonexistent/target " + src + "/.": errors.New("path /nonexistent/target not found"),
		},
	}

	_ = SeedBindMountsFromImageWithRunner(yamlBytes, fake.run)

	gotRm := false
	for _, c := range fake.calls {
		if strings.HasPrefix(c, "rm ") {
			gotRm = true
		}
	}
	if !gotRm {
		t.Errorf("expected docker rm to fire even after cp failure; got calls:\n%s", strings.Join(fake.calls, "\n"))
	}
}
