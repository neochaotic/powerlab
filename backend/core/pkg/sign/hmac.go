package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strconv"
	"strings"
	"time"
)

// HMACSign is an HMAC-SHA256 implementation of Sign. The secret is
// per-install (rotated when the user resets file-share signing).
type HMACSign struct {
	SecretKey []byte
}

// Sign returns the URL-safe base64 HMAC of data + expire, joined
// with the expire timestamp by ":".
func (s HMACSign) Sign(data string, expire int64) string {
	h := hmac.New(sha256.New, s.SecretKey)
	expireTimeStamp := strconv.FormatInt(expire, 10)
	_, err := io.WriteString(h, data+":"+expireTimeStamp)
	if err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(h.Sum(nil)) + ":" + expireTimeStamp
}

// Verify reports nil if sign is a valid HMAC of data + extracted
// expire and the expire timestamp is still in the future. expire
// == 0 is "never expires".
func (s HMACSign) Verify(data, sign string) error {
	signSlice := strings.Split(sign, ":")
	// check whether contains expire time
	if signSlice[len(signSlice)-1] == "" {
		return ErrExpireMissing
	}
	// check whether expire time is expired
	expires, err := strconv.ParseInt(signSlice[len(signSlice)-1], 10, 64)
	if err != nil {
		return ErrExpireInvalid
	}
	// if expire time is expired, return error
	if expires < time.Now().Unix() && expires != 0 {
		return ErrSignExpired
	}
	// verify sign
	if s.Sign(data, expires) != sign {
		return ErrSignInvalid
	}
	return nil
}

// NewHMACSign returns an HMACSign value behind the Sign interface.
// secret should be at least 32 bytes.
func NewHMACSign(secret []byte) Sign {
	return HMACSign{SecretKey: secret}
}
