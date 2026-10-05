package downloadHelper

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/testutil"
	"go.uber.org/goleak"
	"gotest.tools/v3/assert"
)

func TestDownload(t *testing.T) {
	defer goleak.VerifyNone(t, testutil.OpenCensusIgnore())

	src := "https://github.com/IceWhaleTech/get/archive/refs/heads/main.zip"

	dst := t.TempDir()

	err := Download(src, dst)
	assert.NilError(t, err)
}
