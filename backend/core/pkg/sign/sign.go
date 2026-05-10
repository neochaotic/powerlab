// Package sign provides the time-bound HMAC signature scheme used to
// authenticate file-download URLs. Same shape as local-storage's
// pkg/sign — kept duplicated so the core service stays
// independently buildable.
package sign

import "errors"

// Sign is the time-bound signature contract.
type Sign interface {
	Sign(data string, expire int64) string
	Verify(data, sign string) error
}

// Sign error sentinels — returned by Verify when validation fails.
var (
	ErrSignExpired   = errors.New("sign expired")
	ErrSignInvalid   = errors.New("sign invalid")
	ErrExpireInvalid = errors.New("expire invalid")
	ErrExpireMissing = errors.New("expire missing")
)
