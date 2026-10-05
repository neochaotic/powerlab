package v2_test

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/testutil"
)

func TestMain(m *testing.M) {
	// Goroutines started by library init() functions (e.g. ecache
	// background GC, opencensus worker) are ignored by VerifyTestMain.
	testutil.VerifyTestMain(m)
}
