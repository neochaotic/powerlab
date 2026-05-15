package service

import (
	"testing"

	"github.com/docker/docker/api/types"
)

// Regression test for #397 — 2fauth (and other catalog apps that
// use explicit `container_name:`) lose the compose project label
// on their containers. The compose-go library's Start() errors
// with "no container found for project X" because its filter
// `label=com.docker.compose.project=<X>` returns empty. But the
// containers ARE running — just without the expected label.
//
// projectHasContainers must report true when EITHER:
//   - any container carries the project label, OR
//   - any container's name matches the project (either exact
//     match, or `<project>-<svc>-<idx>` / `<project>_<svc>_<idx>`
//     compose-naming patterns).
//
// Failing test first, per TDD strict + bug-regression discipline.

func ctn(name, projectLabel string) types.Container {
	labels := map[string]string{}
	if projectLabel != "" {
		labels["com.docker.compose.project"] = projectLabel
	}
	return types.Container{
		Names:  []string{"/" + name},
		Labels: labels,
	}
}

func TestProjectHasContainers_LabelMatchWins(t *testing.T) {
	got := projectHasContainers([]types.Container{
		ctn("anything_3", "myproj"),
	}, "myproj")
	if !got {
		t.Error("label match should report true")
	}
}

func TestProjectHasContainers_LabelMismatchReportsFalse(t *testing.T) {
	got := projectHasContainers([]types.Container{
		ctn("foo", "other"),
		ctn("bar", "another"),
	}, "myproj")
	if got {
		t.Error("none of the labels match — should report false")
	}
}

func TestProjectHasContainers_ContainerNameExactMatchFallback(t *testing.T) {
	// The 2fauth case: container_name: 2fauth, NO project label.
	got := projectHasContainers([]types.Container{
		ctn("2fauth", ""),
	}, "2fauth")
	if !got {
		t.Errorf("#397: container_name exact match must trigger fallback (2fauth bug)")
	}
}

func TestProjectHasContainers_ComposeV2HyphenPatternFallback(t *testing.T) {
	// docker-compose v2 default naming: <project>-<service>-<index>.
	// AdventureLog case: `server` service has container_name:
	// adventurelogserver1, but `db` and `web` use defaults.
	got := projectHasContainers([]types.Container{
		ctn("adventurelog-db-1", ""), // no label, hyphen-form name
	}, "adventurelog")
	if !got {
		t.Errorf("compose v2 hyphen naming must trigger fallback")
	}
}

func TestProjectHasContainers_ComposeV1UnderscorePatternFallback(t *testing.T) {
	// docker-compose v1 (legacy) default naming: <project>_<service>_<index>.
	got := projectHasContainers([]types.Container{
		ctn("legacy_db_1", ""),
	}, "legacy")
	if !got {
		t.Errorf("compose v1 underscore naming must trigger fallback")
	}
}

func TestProjectHasContainers_PrefixOnlyDoesNotMatch(t *testing.T) {
	// Substring match would create false positives — e.g. project
	// "blink" must NOT match container "blinko-db-1". The pattern
	// must anchor the project name as a whole token.
	got := projectHasContainers([]types.Container{
		ctn("blinko-db-1", ""),
	}, "blink")
	if got {
		t.Errorf("prefix-only must NOT match — would be false positive")
	}
}

func TestProjectHasContainers_DockerNameSlashStripped(t *testing.T) {
	// Docker prepends "/" to container names (legacy moby quirk).
	// The fallback must strip it.
	got := projectHasContainers([]types.Container{
		{Names: []string{"/2fauth"}, Labels: nil},
	}, "2fauth")
	if !got {
		t.Errorf("the leading / must not block the match")
	}
}

func TestProjectHasContainers_EmptyListReportsFalse(t *testing.T) {
	got := projectHasContainers(nil, "anything")
	if got {
		t.Error("nil container list must report false")
	}
}
