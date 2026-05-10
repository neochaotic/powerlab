// Package sign provides the time-bound signature scheme used to
// authenticate file-download URLs. The HMAC implementation in
// hmac.go is the only Sign currently used; the interface exists
// so a future asymmetric scheme can drop in without touching call
// sites.
package sign

import "errors"

// Sign is the time-bound signature contract. Sign produces a
// signature that's valid for `expire` seconds; Verify reports
// ErrSign* when the signature is bad / expired.
type Sign interface {
	Sign(data string, expire int64) string
	Verify(data, sign string) error
}

var (
	ErrSignExpired   = errors.New("sign expired")
	ErrSignInvalid   = errors.New("sign invalid")
	ErrExpireInvalid = errors.New("expire invalid")
	ErrExpireMissing = errors.New("expire missing")
)
