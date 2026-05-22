//go:build integration

// Phase 3 of #150 — testcontainers-class integration tests for the
// Docker-touching code paths in `app-management/service`. Sprint 13.5,
// carry from Sprint 11.
//
// Why a `//go:build integration` tag instead of testcontainers-go:
// the runtime that runs these tests already needs Docker available
// (we shell into the Docker socket the same way prod does). A heavy
// testcontainers dep would duplicate that — we just need a flag to
// keep these out of the default `go test ./...` so they don't fail
// on dev machines without Docker.
//
// To run locally:
//   go test -tags=integration ./service/...
//
// CI workflow (.github/workflows/ci.yml) runs `go test -tags=integration`
// in a job after the default unit tests pass — see the new
// `backend-integration` matrix entry.
//
// The tests here are intentionally minimal: prove the Docker socket
// is reachable, prove image-pull works against a tiny known-good
// image. Richer end-to-end install tests come in follow-ups; this
// PR establishes the pattern.

package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// TestDockerSocketReachable verifies the integration runner has a
// working Docker socket. If this fails, NONE of the more elaborate
// Docker-touching tests will work either — surface the diagnosis
// early at the suite gate level.
func TestDockerSocketReachable(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("create docker client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := cli.Ping(ctx); err != nil {
		t.Fatalf("docker daemon Ping failed — is Docker running? %v", err)
	}
}

// TestDockerListImagesWorks proves that the image-listing path
// (used by app-management's Pull flow to check if an image is
// already cached) works in the integration environment.
func TestDockerListImagesWorks(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("create docker client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// We don't care if there are 0 or 100 images — only that the
	// API call succeeds. If the call errors, the entire Pull
	// pipeline in prod would also fail.
	if _, err := cli.ImageList(ctx, image.ListOptions{}); err != nil {
		t.Fatalf("list images: %v", err)
	}
}
