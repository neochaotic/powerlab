package service

import (
	"context"
	"regexp"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"go.uber.org/zap"
)

// dockerAutoRenamePrefix matches the `<12-hex>_` prefix Docker
// stamps onto a container when it auto-renames the existing one
// during a name conflict (recreate flow). Lowercase only — Docker
// generates these from the container ID's hex.
var dockerAutoRenamePrefix = regexp.MustCompile(`^[a-f0-9]{12}_`)

// isAutoRenamedOrphan reports whether containerName looks like a
// Docker auto-rename of a container that originally belonged to
// project. Pattern: `<12-lowercase-hex>_<project>` exactly.
//
// Why this exists (Sprint 17 C1, .142 incident with 2fauth):
// `compose-go`'s `RemoveOrphans: true` only catches containers
// whose service-name is missing from the current compose file.
// When Docker auto-renames a name-conflicting container, the
// renamed one keeps the project label but the service-name
// field still matches the active service — so RemoveOrphans
// thinks it's the legitimate container and leaves it alone.
// On the next install, compose-go races the orphan removal with
// the new create and references a now-dead container ID, surfacing
// `Error response from daemon: No such container: <sha>`.
//
// Pure function, no Docker dependency — testable as a string
// matcher. The cleanup loop in cleanupAutoRenamedOrphans does
// the actual Docker API calls.
func isAutoRenamedOrphan(containerName, project string) bool {
	if containerName == "" || project == "" {
		return false
	}
	if !dockerAutoRenamePrefix.MatchString(containerName) {
		return false
	}
	// After the 12-hex + underscore prefix (13 chars) the rest must
	// equal the project name exactly. No "double underscore" or
	// other suffix variants.
	const prefixLen = 13 // 12 hex + 1 underscore
	if len(containerName) < prefixLen+len(project) {
		return false
	}
	return containerName[prefixLen:] == project
}

// cleanupAutoRenamedOrphans force-removes any container that
// looks like a Docker auto-rename of this compose project. Best-
// effort: each remove failure is logged + skipped, never aborts
// the cleanup loop (one stuck container shouldn't block the
// removal of others).
//
// Called from Uninstall() AFTER `service.Down(...)` so we catch
// the orphans compose-go missed. Also safe to call from a future
// "force-cleanup" recovery endpoint — operations are idempotent
// (containers that don't exist cause a NotFound which we ignore).
func cleanupAutoRenamedOrphans(ctx context.Context, dockerClient client.APIClient, project string) (int, error) {
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.docker.compose.project="+project),
		),
	})
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, c := range containers {
		for _, name := range c.Names {
			// Docker prefixes container names with "/" in the API.
			trimmed := name
			if len(trimmed) > 0 && trimmed[0] == '/' {
				trimmed = trimmed[1:]
			}
			if !isAutoRenamedOrphan(trimmed, project) {
				continue
			}
			logger.Info("removing auto-renamed orphan",
				zap.String("project", project),
				zap.String("name", trimmed),
				zap.String("id", c.ID),
			)
			if err := dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{
				Force: true,
			}); err != nil {
				// Best-effort: log via Error (the logger pkg has no
				// Warn level — Error is the closest available without
				// adding a new helper just for this site).
				logger.Error("failed to remove orphan; continuing",
					zap.String("project", project),
					zap.String("name", trimmed),
					zap.Error(err),
				)
				continue
			}
			removed++
			break // one matching name per container is enough
		}
	}
	return removed, nil
}
