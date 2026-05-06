package v2_test

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// IgnoreCurrent captures goroutines started by library init() functions
	// (e.g. ecache background GC, opencensus worker) before any test runs.
	opt := goleak.IgnoreCurrent()
	goleak.VerifyTestMain(m, opt)
}
