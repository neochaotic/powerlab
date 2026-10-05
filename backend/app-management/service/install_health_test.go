package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// #440: PullAndInstall reported success while a service was crash
// looping, because compose's Wait only needs a container to be
// "running" once. These tests drive the post-start state check with
// canned `docker ps -a` snapshots.

func svc(project, service, state, status string) types.Container {
	return types.Container{
		ID:     project + "-" + service,
		Names:  []string{"/" + project + "-" + service + "-1"},
		State:  state,
		Status: status,
		Labels: map[string]string{
			api.ProjectLabel: project,
			api.ServiceLabel: service,
		},
	}
}

func TestEvaluateInstallHealth_AllRunningPasses(t *testing.T) {
	snap := []types.Container{
		svc("blinko", "app", "running", "Up 10 seconds"),
		svc("blinko", "db", "running", "Up 12 seconds (healthy)"),
	}
	if err := evaluateInstallHealth([][]types.Container{snap, snap, snap}, "blinko"); err != nil {
		t.Fatalf("healthy project reported failure: %v", err)
	}
}

func TestEvaluateInstallHealth_RestartingAtEndFails(t *testing.T) {
	ok := svc("blinko", "app", "running", "Up 2 seconds")
	looping := svc("blinko", "app", "restarting", "Restarting (1) 1 second ago")
	err := evaluateInstallHealth([][]types.Container{{ok}, {ok}, {looping}}, "blinko")
	if err == nil {
		t.Fatal("restarting container in final snapshot must fail the install")
	}
	want := `service "app" never became healthy (restarting)`
	if err.Error() != want {
		t.Fatalf("err=%q; want %q", err, want)
	}
}

// A crash loop sampled while "running" at the end is still caught
// when it was seen restarting in more than one snapshot.
func TestEvaluateInstallHealth_RepeatedRestartsFailEvenIfRunningAtEnd(t *testing.T) {
	looping := svc("app1", "web", "restarting", "Restarting (137) 2 seconds ago")
	running := svc("app1", "web", "running", "Up Less than a second")
	err := evaluateInstallHealth([][]types.Container{{looping}, {looping}, {running}}, "app1")
	if err == nil || !strings.Contains(err.Error(), `"web" never became healthy (restarting)`) {
		t.Fatalf("err=%v; want restarting failure for web", err)
	}
}

// One restart that recovers (app waited for its database) is fine.
func TestEvaluateInstallHealth_SingleRecoveredRestartPasses(t *testing.T) {
	restarting := svc("app1", "web", "restarting", "Restarting (1) 1 second ago")
	running := svc("app1", "web", "running", "Up 3 seconds")
	if err := evaluateInstallHealth([][]types.Container{{restarting}, {running}, {running}}, "app1"); err != nil {
		t.Fatalf("recovered single restart reported failure: %v", err)
	}
}

func TestEvaluateInstallHealth_ExitedNonZeroFails(t *testing.T) {
	snap := []types.Container{
		svc("app1", "db", "running", "Up 5 seconds"),
		svc("app1", "web", "exited", "Exited (1) 2 seconds ago"),
	}
	err := evaluateInstallHealth([][]types.Container{snap}, "app1")
	if err == nil || err.Error() != `service "web" never became healthy (exited)` {
		t.Fatalf("err=%v; want exited failure for web", err)
	}
}

// One-shot init/migration services exit 0 by design.
func TestEvaluateInstallHealth_ExitedZeroPasses(t *testing.T) {
	snap := []types.Container{
		svc("app1", "migrate", "exited", "Exited (0) 3 seconds ago"),
		svc("app1", "web", "running", "Up 3 seconds"),
	}
	if err := evaluateInstallHealth([][]types.Container{snap, snap}, "app1"); err != nil {
		t.Fatalf("exit-0 one-shot reported failure: %v", err)
	}
}

func TestEvaluateInstallHealth_UnhealthyFails(t *testing.T) {
	snap := []types.Container{svc("app1", "web", "running", "Up 40 seconds (unhealthy)")}
	err := evaluateInstallHealth([][]types.Container{snap}, "app1")
	if err == nil || err.Error() != `service "web" never became healthy (unhealthy)` {
		t.Fatalf("err=%v; want unhealthy failure", err)
	}
}

// Other projects' broken containers must not fail this install,
// including a project whose name merely shares a prefix.
func TestEvaluateInstallHealth_IgnoresOtherProjects(t *testing.T) {
	snap := []types.Container{
		svc("blink", "web", "running", "Up 5 seconds"),
		svc("blinko", "web", "restarting", "Restarting (1) 1 second ago"),
		svc("other", "db", "exited", "Exited (1) 1 minute ago"),
	}
	if err := evaluateInstallHealth([][]types.Container{snap, snap}, "blink"); err != nil {
		t.Fatalf("foreign project leaked into the check: %v", err)
	}
}

// #397 containers (explicit container_name, no project label) are
// matched by name and checked too.
func TestEvaluateInstallHealth_UnlabeledContainerMatchedByName(t *testing.T) {
	c := types.Container{Names: []string{"/2fauth"}, State: "restarting", Status: "Restarting (1) 1 second ago"}
	err := evaluateInstallHealth([][]types.Container{{c}}, "2fauth")
	if err == nil || err.Error() != `service "2fauth" never became healthy (restarting)` {
		t.Fatalf("err=%v; want restarting failure for 2fauth", err)
	}
}

type cannedLister struct {
	snapshots [][]types.Container
	err       error
	calls     int
}

func (l *cannedLister) ContainerList(_ context.Context, _ container.ListOptions) ([]types.Container, error) {
	if l.err != nil {
		return nil, l.err
	}
	i := l.calls
	if i >= len(l.snapshots) {
		i = len(l.snapshots) - 1
	}
	l.calls++
	return l.snapshots[i], nil
}

func withFastHealthSampling(t *testing.T) {
	t.Helper()
	origSamples, origInterval := installHealthSamples, installHealthInterval
	installHealthSamples, installHealthInterval = 3, time.Millisecond
	t.Cleanup(func() {
		installHealthSamples, installHealthInterval = origSamples, origInterval
	})
}

func TestCheckInstallHealth_SamplesAndFails(t *testing.T) {
	withFastHealthSampling(t)
	running := svc("app1", "web", "running", "Up 1 second")
	looping := svc("app1", "web", "restarting", "Restarting (1) 1 second ago")
	l := &cannedLister{snapshots: [][]types.Container{{running}, {looping}, {looping}}}

	err := checkInstallHealth(context.Background(), l, "app1")
	if err == nil || !strings.Contains(err.Error(), "never became healthy (restarting)") {
		t.Fatalf("err=%v; want restarting failure", err)
	}
	if l.calls != 3 {
		t.Fatalf("ContainerList called %d times; want 3 samples", l.calls)
	}
}

// Best-effort: a daemon list error must not fail an install compose
// already reported as started.
func TestCheckInstallHealth_ListErrorIsNotFatal(t *testing.T) {
	withFastHealthSampling(t)
	l := &cannedLister{err: errors.New("daemon hiccup")}
	if err := checkInstallHealth(context.Background(), l, "app1"); err != nil {
		t.Fatalf("list error surfaced as install failure: %v", err)
	}
}

func TestInstallWaitTimeoutDefault(t *testing.T) {
	if installWaitTimeout != 90*time.Second {
		t.Fatalf("installWaitTimeout=%s; want 90s default", installWaitTimeout)
	}
}

// Only compose's "no container found for project" error (wrapped
// api.ErrNotFound) may be downgraded by the #397 name fallback; a
// WaitTimeout expiry must still fail the install.
func TestStartErrIsMissingProjectLabel(t *testing.T) {
	notFound := fmt.Errorf("no container found for project %q: %w", "2fauth", api.ErrNotFound)
	if !startErrIsMissingProjectLabel(notFound) {
		t.Error("wrapped api.ErrNotFound must be treated as the #397 case")
	}
	timeout := fmt.Errorf("application not healthy after %s", 90*time.Second)
	if startErrIsMissingProjectLabel(timeout) {
		t.Error("wait timeout must not be downgraded by the name fallback")
	}
	if startErrIsMissingProjectLabel(errors.New("container app1-web-1 is unhealthy")) {
		t.Error("unhealthy error must not be downgraded by the name fallback")
	}
}
