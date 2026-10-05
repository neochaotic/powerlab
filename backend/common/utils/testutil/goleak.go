// Package testutil holds test helpers shared by the backend modules.
//
// It lives in backend/common because every service module (including
// app-management, which does not depend on backend/pkg) already requires it.
package testutil

import "go.uber.org/goleak"

// OpenCensusIgnore ignores the opencensus stats worker, which is started
// from a package init() and never stops.
// https://github.com/census-instrumentation/opencensus-go/issues/1191
func OpenCensusIgnore() goleak.Option {
	return goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
}

// SocketIOIgnore ignores the accept loop go-socket.io's engineio server
// leaves running after the server is closed.
func SocketIOIgnore() goleak.Option {
	return goleak.IgnoreTopFunction("github.com/CorrectRoadH/go-socket.io/engineio.(*Server).Accept")
}

// HTTPClientIgnores ignores the goroutines net/http keeps alive for pooled
// idle (keep-alive) client connections, including TLS ones.
func HTTPClientIgnores() []goleak.Option {
	return []goleak.Option{
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("net.(*netFD).Read"),
		goleak.IgnoreTopFunction("crypto/tls.(*Conn).readRecordOrCCS"),
	}
}

// VerifyTestMain runs the package's tests and fails if goroutines leak.
// Goroutines already running when it is called (those started by library
// init functions, e.g. the ecache janitor) are ignored, along with opts.
// Call it from TestMain.
func VerifyTestMain(m goleak.TestingM, opts ...goleak.Option) {
	goleak.VerifyTestMain(m, append([]goleak.Option{goleak.IgnoreCurrent()}, opts...)...)
}
