package docker_test

import (
	"fmt"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
)

func TestGetDir(t *testing.T) {
	fmt.Println(docker.GetDir("", "config"))
}
