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

// ParseToken verifies signedToken with the publicKeyFunc-provided
// key. publicKeyFunc is a callback (rather than a fixed key) so
// callers can fetch lazily + cache via external.GetPublicKey.
// Rejects anything not signed with ECDSA.
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
// "casaos" — kept for backwards-compat with CasaOS-era clients).
func GetAccessToken(username string, privateKey *ecdsa.PrivateKey, id int) (string, error) {
	return GenerateToken(username, privateKey, id, "casaos", 3*time.Hour)
}

// GetRefreshToken issues a long-lived refresh JWT (7 days, issuer
// "refresh"). Refresh tokens MUST only be accepted by the refresh
// endpoint — verify the issuer claim before exchanging.
func GetRefreshToken(username string, private *ecdsa.PrivateKey, id int) (string, error) {
	return GenerateToken(username, private, id, "refresh", 7*24*time.Hour)
}

// Validate is the (bool, *Claims, error) wrapper around ParseToken
// preferred by middleware code paths. The bool restates "no error"
// — kept for the existing call sites that key on it.
func Validate(token string, publicKeyFunc func() (*ecdsa.PublicKey, error)) (bool, *Claims, error) {
	claims, err := ParseToken(token, publicKeyFunc)
	if err != nil {
		return false, nil, err
	}

	if claims != nil {
		return true, claims, nil
	}

	return false, nil, errors.New("invalid token")
}
