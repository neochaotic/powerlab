// Package jwt is the shared ES256 JWT helper used by every PowerLab
// backend service. user-service is the only signer (it owns the
// keypair, persisted per ADR-0020); every other service is a
// verifier and pulls the public key via JWKS.
package jwt

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

// Claims is PowerLab's JWT payload. Username + ID identify the
// authenticated user; the embedded RegisteredClaims carry the
// standard exp/iat/nbf/iss fields. Kept stable — changing the
// shape requires re-issuing every active token.
type Claims struct {
	Username string `json:"username"`
	ID       int    `json:"id"`
	jwt.RegisteredClaims
}

// GenerateToken signs a Claims-shaped JWT with the ES256 algorithm.
// Used directly only by user-service; other services hit the
// GetAccessToken / GetRefreshToken wrappers which fix the issuer
// and lifetime.
func GenerateToken(username string, privateKey *ecdsa.PrivateKey, id int, issuer string, t time.Duration) (string, error) {
	claims := Claims{
		username,
		id,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(t)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	signedToken, err := token.SignedString(privateKey)
	return signedToken, err
}

// AcceptedAccessIssuers is the set of issuer-claim values that
// Validate accepts on access tokens. "powerlab" is the new issuer
// (post-#246, Sprint 9). "casaos" is the legacy CasaOS-era issuer
// kept ONLY during the bridging release so existing sessions don't
// get logged out on upgrade. Drop "casaos" in v0.7 (tracked in
// CHANGELOG + ADR).
//
// "refresh" is the issuer reserved for refresh tokens, exchanged
// only by the dedicated refresh endpoint — it is intentionally
// NOT in this set.
var AcceptedAccessIssuers = map[string]struct{}{
	"powerlab": {},
	"casaos":   {},
}

// ParseToken verifies signedToken with the publicKeyFunc-provided
// key. publicKeyFunc is a callback (rather than a fixed key) so
// callers can fetch lazily + cache via external.GetPublicKey.
// Rejects anything not signed with ECDSA.
//
// ParseToken does NOT enforce the issuer allowlist — the refresh
// endpoint needs to read `iss=refresh` tokens via ParseToken. Use
// Validate when you want the access-token issuer guard (#246).
func ParseToken(signedToken string, publicKeyFunc func() (*ecdsa.PublicKey, error)) (*Claims, error) {
	token, err := jwt.ParseWithClaims(signedToken, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return publicKeyFunc()
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GetAccessToken issues a short-lived access JWT (3 hours, issuer
// "powerlab"). The legacy "casaos" issuer string was dropped in
// #246 — see AcceptedAccessIssuers for the bridging-release accept
// set.
func GetAccessToken(username string, privateKey *ecdsa.PrivateKey, id int) (string, error) {
	return GenerateToken(username, privateKey, id, "powerlab", 3*time.Hour)
}

// GetRefreshToken issues a long-lived refresh JWT (7 days, issuer
// "refresh"). Refresh tokens MUST only be accepted by the refresh
// endpoint — verify the issuer claim before exchanging.
func GetRefreshToken(username string, private *ecdsa.PrivateKey, id int) (string, error) {
	return GenerateToken(username, private, id, "refresh", 7*24*time.Hour)
}

// Validate is the (bool, *Claims, error) wrapper around ParseToken
// used by access-token middleware. Beyond signature verification
// (handled by ParseToken) it enforces the AcceptedAccessIssuers
// allowlist — a signature-valid token from an unknown issuer is
// rejected. Refresh tokens (iss="refresh") deliberately fail this
// check; they go through the dedicated refresh endpoint which uses
// ParseToken directly.
//
// The bool restates "no error" — kept for the existing call sites
// that key on it.
func Validate(token string, publicKeyFunc func() (*ecdsa.PublicKey, error)) (bool, *Claims, error) {
	claims, err := ParseToken(token, publicKeyFunc)
	if err != nil {
		return false, nil, err
	}

	if claims == nil {
		return false, nil, errors.New("invalid token")
	}

	if _, ok := AcceptedAccessIssuers[claims.Issuer]; !ok {
		return false, nil, fmt.Errorf("token has unrecognized issuer %q (#246)", claims.Issuer)
	}

	return true, claims, nil
}
