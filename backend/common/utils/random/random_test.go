package random_test

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/random"
)

func TestString(t *testing.T) {
	t.Log(random.String(6, true))
}

func TestName(t *testing.T) {
	t.Log(random.Name(nil))

	suffix := "whatever"
	t.Log(random.Name(&suffix))
}
