package service_test

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/testutil"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Goroutines already running before tests start (library init
	// functions) are ignored by VerifyTestMain.
	opts := []goleak.Option{
		testutil.OpenCensusIgnore(),
		goleak.IgnoreTopFunction("github.com/neochaotic/powerlab/backend/app-management/service_test.topFunc1"),
		goleak.IgnoreTopFunction("github.com/neochaotic/powerlab/backend/app-management/service_test.pollFunc1"),
		goleak.IgnoreTopFunction("github.com/neochaotic/powerlab/backend/app-management/service_test.httpFunc1"),
	}
	testutil.VerifyTestMain(m, append(opts, testutil.HTTPClientIgnores()...)...)
}
