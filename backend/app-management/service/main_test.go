package service_test

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Ignore goroutines that are already running before tests start (like those from library init functions)
	opt := goleak.IgnoreCurrent()

	goleak.VerifyTestMain(m, opt,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/IceWhaleTech/CasaOS-AppManagement/service_test.topFunc1"),
		goleak.IgnoreTopFunction("github.com/IceWhaleTech/CasaOS-AppManagement/service_test.pollFunc1"),
		goleak.IgnoreTopFunction("github.com/IceWhaleTech/CasaOS-AppManagement/service_test.httpFunc1"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("net.(*netFD).Read"),
		goleak.IgnoreTopFunction("crypto/tls.(*Conn).readRecordOrCCS"),
	)
}
