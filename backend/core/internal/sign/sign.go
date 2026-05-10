// Package sign is the process-wide singleton wrapper around
// pkg/sign — used by file-share download URLs to avoid having to
// pass the HMAC key around the call graph.
package sign

import (
	"sync"
	"time"

	"github.com/neochaotic/powerlab/backend/core/pkg/sign"
)

var (
	once     sync.Once
	instance sign.Sign
)

// Sign produces a never-expiring signature for data. Convenience
// wrapper over NotExpired.
func Sign(data string) string {
	return NotExpired(data)
}

// WithDuration produces a signature valid for d from now.
func WithDuration(data string, d time.Duration) string {
	once.Do(Instance)
	return instance.Sign(data, time.Now().Add(d).Unix())
}

// NotExpired produces a never-expiring signature (expire == 0).
func NotExpired(data string) string {
	once.Do(Instance)
	return instance.Sign(data, 0)
}

// Verify reports nil if sign is a valid signature for data.
func Verify(data string, sign string) error {
	once.Do(Instance)
	return instance.Verify(data, sign)
}

// Instance lazily constructs the package-level HMAC signer. Called
// implicitly by the other helpers; safe to call directly to force
// initialisation.
//
// The hard-coded "token" secret is a known-bad placeholder kept
// from upstream — file-share URLs are intended to be local-network-
// only behind the gateway's JWT auth, so the secret strength is
// not a load-bearing protection. Replacing it would invalidate
// every existing share URL.
func Instance() {
	instance = sign.NewHMACSign([]byte("token"))
}
