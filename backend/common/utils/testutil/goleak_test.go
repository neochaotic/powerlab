package testutil

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	VerifyTestMain(m)
}

func TestIgnoreOptionsAreUsable(t *testing.T) {
	opts := append(HTTPClientIgnores(), OpenCensusIgnore(), SocketIOIgnore())
	if len(opts) != 7 {
		t.Fatalf("expected 7 ignore options, got %d", len(opts))
	}
	goleak.VerifyNone(t, opts...)
}

func TestVerifyNoneStillCatchesLeaks(t *testing.T) {
	stop := make(chan struct{})
	go func() { <-stop }()
	defer close(stop)

	opts := append(HTTPClientIgnores(), OpenCensusIgnore(), SocketIOIgnore())
	if err := goleak.Find(opts...); err == nil {
		t.Fatal("expected the shared ignores not to hide an unrelated leaked goroutine")
	}
}
