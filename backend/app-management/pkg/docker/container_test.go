package docker_test

import (
	"context"
	"io"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/common/utils/random"
	"github.com/samber/lo"
	"go.uber.org/goleak"
	"gotest.tools/v3/assert"
)

// skipIfDockerIncompatible skips the test when the Docker daemon is not running
// or its API version is incompatible with the SDK (e.g. daemon requires ≥1.44 but SDK caps at 1.43).
func skipIfDockerIncompatible(t *testing.T) {
	t.Helper()
	_, err := docker.CurrentArchitecture()
	if err != nil {
		t.Skip("Docker daemon incompatible or not running: " + err.Error())
	}
}

func setupTestContainer(ctx context.Context, t *testing.T) *container.CreateResponse {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	assert.NilError(t, err)
	defer cli.Close()

	imageName := "alpine:latest"

	config := &container.Config{
		Image: imageName,
		Cmd:   []string{"tail", "-f", "/dev/null"},
		Env:   []string{"FOO=BAR"},
	}

	hostConfig := &container.HostConfig{}
	networkingConfig := &network.NetworkingConfig{}

	out, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	assert.NilError(t, err)

	_, err = io.ReadAll(out)
	assert.NilError(t, err)

	response, err := cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, "test-"+random.RandomString(4, false))
	assert.NilError(t, err)

	return &response
}

func TestCloneContainer(t *testing.T) {
	defer goleak.VerifyNone(t)

	defer func() {
		// workaround due to https://github.com/patrickmn/go-cache/issues/166
		docker.Cache = nil
		runtime.GC()
	}()

	skipIfDockerIncompatible(t)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	assert.NilError(t, err)
	defer cli.Close()

	ctx := context.Background()

	// setup
	response := setupTestContainer(ctx, t)

	defer func() {
		err = cli.ContainerRemove(ctx, response.ID, container.RemoveOptions{})
		assert.NilError(t, err)
	}()

	err = docker.StartContainer(ctx, response.ID)
	assert.NilError(t, err)

	defer func() {
		err = docker.StopContainer(ctx, response.ID)
		assert.NilError(t, err)
	}()

	newID, err := docker.CloneContainer(ctx, response.ID, "test-"+random.RandomString(4, false))
	assert.NilError(t, err)

	defer func() {
		err := docker.RemoveContainer(ctx, newID)
		assert.NilError(t, err)
	}()

	err = docker.StartContainer(ctx, newID)
	assert.NilError(t, err)

	defer func() {
		err := docker.StopContainer(ctx, newID)
		assert.NilError(t, err)
	}()

	containerInfo, err := docker.Container(ctx, newID)
	assert.NilError(t, err)
	assert.Assert(t, lo.Contains(containerInfo.Config.Env, "FOO=BAR"))
}

func TestNonExistingContainer(t *testing.T) {
	skipIfDockerIncompatible(t)
	containerInfo, err := docker.Container(context.Background(), "non-existing-container")
	assert.ErrorContains(t, err, "non-existing-container")
	assert.Assert(t, containerInfo == nil)
}
