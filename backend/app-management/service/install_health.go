package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// Post-install health check (#440). `compose up` with Wait:true only
// guarantees each container reached running (or healthy, when a
// healthcheck is declared) once. A service with no healthcheck that
// crash-loops under `restart: unless-stopped` is momentarily
// "running" and passes compose's wait, so the install used to report
// success while the app was down. After Start returns, PullAndInstall
// samples the project's containers a few times and fails the install
// with a clear message when a service is restarting, has exited with
// a non-zero code or reports unhealthy.

var (
	// installWaitTimeout bounds compose's Wait during PullAndInstall
	// (api.StartOptions.WaitTimeout). Without it a never-healthy
	// service would hold the install task open indefinitely.
	installWaitTimeout = 90 * time.Second

	// installHealthSamples / installHealthInterval define the short
	// post-start observation window (~6s by default). Package vars so
	// tests can shrink them.
	installHealthSamples  = 3
	installHealthInterval = 3 * time.Second
)

// containerLister is the slice of the Docker client the post-start
// check needs. Hand-typed so tests can feed canned container lists.
type containerLister interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
}

// containerInProject reports whether c belongs to the compose
// project. The compose project label is authoritative when present;
// containers without it (explicit container_name:, see #397) fall back
// to the same name patterns projectHasContainers accepts.
func containerInProject(c types.Container, project string) bool {
	if project == "" {
		return false
	}
	if label, ok := c.Labels[api.ProjectLabel]; ok && label != "" {
		return label == project
	}
	for _, raw := range c.Names {
		name := strings.TrimPrefix(raw, "/")
		if name == project || strings.HasPrefix(name, project+"-") || strings.HasPrefix(name, project+"_") {
			return true
		}
	}
	return false
}

// serviceNameOf returns the compose service name of c, falling back
// to its container name when the service label is missing.
func serviceNameOf(c types.Container) string {
	if s := c.Labels[api.ServiceLabel]; s != "" {
		return s
	}
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	return c.ID
}

// containerProblem returns a short reason ("restarting", "exited",
// "unhealthy") when c is in a state an installed app must not be in,
// or "" when it is fine. A container that exited with code 0 is a
// completed one-shot (init/migration service) and is not a problem;
// "created" and "health: starting" are not flagged either, since
// compose's own Wait already covers services that never started.
func containerProblem(c types.Container) string {
	switch strings.ToLower(c.State) {
	case "restarting":
		return "restarting"
	case "dead":
		return "exited"
	case "exited":
		if strings.HasPrefix(c.Status, "Exited (0)") {
			return ""
		}
		return "exited"
	}
	if strings.Contains(c.Status, "(unhealthy)") {
		return "unhealthy"
	}
	return ""
}

// projectContainerProblems maps service name → problem for every
// container of the project that is in a bad state in this snapshot.
func projectContainerProblems(containers []types.Container, project string) map[string]string {
	problems := map[string]string{}
	for _, c := range containers {
		if !containerInProject(c, project) {
			continue
		}
		if p := containerProblem(c); p != "" {
			problems[serviceNameOf(c)] = p
		}
	}
	return problems
}

// evaluateInstallHealth decides the outcome from a series of
// snapshots, oldest first. A service fails when it is bad in the
// final snapshot (it did not recover within the window) or when it
// was bad in more than one snapshot (caught restarting repeatedly: a
// crash loop that happened to be "running" at the last sample). A
// single bad sample followed by recovery is tolerated, so an app that
// restarts once while its database finishes initialising still
// installs. Returns nil when every service is fine.
func evaluateInstallHealth(snapshots [][]types.Container, project string) error {
	if len(snapshots) == 0 {
		return nil
	}
	badCount := map[string]int{}
	lastReason := map[string]string{}
	var order []string
	for _, snap := range snapshots {
		for svc, reason := range projectContainerProblems(snap, project) {
			if badCount[svc] == 0 {
				order = append(order, svc)
			}
			badCount[svc]++
			lastReason[svc] = reason
		}
	}
	final := projectContainerProblems(snapshots[len(snapshots)-1], project)
	for _, svc := range order {
		reason, badAtEnd := final[svc]
		if !badAtEnd && badCount[svc] < 2 {
			continue
		}
		if !badAtEnd {
			reason = lastReason[svc]
		}
		return fmt.Errorf("service %q never became healthy (%s)", svc, reason)
	}
	return nil
}

// checkInstallHealth samples the project's containers
// installHealthSamples times, installHealthInterval apart, and
// returns evaluateInstallHealth's verdict. Best-effort: a
// ContainerList failure skips the check (returns nil) rather than
// failing an install that compose already reported as started.
func checkInstallHealth(ctx context.Context, lister containerLister, project string) error {
	snapshots := make([][]types.Container, 0, installHealthSamples)
	for i := 0; i < installHealthSamples; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(installHealthInterval):
			}
		}
		list, err := lister.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return nil
		}
		snapshots = append(snapshots, list)
	}
	return evaluateInstallHealth(snapshots, project)
}

// startErrIsMissingProjectLabel reports whether a compose Start error
// is the "no container found for project" case (#397), which compose
// wraps around api.ErrNotFound. Only that error may be downgraded by
// the container-name fallback; a Wait timeout or an unhealthy/exited
// container must still fail the install.
func startErrIsMissingProjectLabel(err error) bool {
	return errors.Is(err, api.ErrNotFound)
}
