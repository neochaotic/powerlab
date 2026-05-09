package docker_test

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"gotest.tools/v3/assert"
)

func TestCurrentArchitecture(t *testing.T) {
	a, err := docker.CurrentArchitecture()
	if err != nil {
		t.Skip("Docker daemon incompatible or not running: " + err.Error())
	}
	assert.Assert(t, a != "")
}
