package service

import (
	"strings"

	"github.com/docker/docker/api/types"
)

// projectHasContainers reports whether any container in the given
// list belongs to the named compose project. Compose-go's standard
// lookup uses the `com.docker.compose.project` label, but catalog
// entries that set explicit `container_name:` (e.g. 2fauth, the
// `server` service in adventurelog) end up with containers that
// lose the label. The post-`service.Start()` health check needs a
// tolerant fallback so a project with running-but-unlabeled
// containers isn't reported as "no container found for project".
//
// Match precedence:
//  1. The compose project label is the source of truth — any hit
//     wins regardless of name.
//  2. Else, fall back to the container name (with Docker's leading
//     "/" stripped). Accepted forms:
//     - exact match — `name == project`           (container_name)
//     - hyphen form — `<project>-<svc>-<idx>`      (compose v2)
//     - underscore form — `<project>_<svc>_<idx>`  (compose v1, legacy)
//
// The fallback patterns anchor the project name as a whole token
// (followed by `-`, `_`, or end-of-string) so "blink" doesn't
// match "blinko-db-1".
func projectHasContainers(containers []types.Container, project string) bool {
	if project == "" {
		return false
	}
	for _, c := range containers {
		if c.Labels["com.docker.compose.project"] == project {
			return true
		}
	}
	for _, c := range containers {
		for _, raw := range c.Names {
			name := strings.TrimPrefix(raw, "/")
			if name == project {
				return true
			}
			if strings.HasPrefix(name, project+"-") || strings.HasPrefix(name, project+"_") {
				return true
			}
		}
	}
	return false
}
