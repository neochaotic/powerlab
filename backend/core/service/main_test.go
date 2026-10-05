package service

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/testutil"
)

func TestMain(m *testing.M) {
	// Goroutines already running before tests start (library init
	// functions) are ignored by VerifyTestMain.
	testutil.VerifyTestMain(m, testutil.HTTPClientIgnores()...)
}
