package jwt_test

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/stretchr/testify/require"
)

// TestTokenTTL checks the lifetime each issuer helper signs into the
// token (exp - iat), including the non-positive fallback (#484).
func TestTokenTTL(t *testing.T) {
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)
	keyFunc := func() (*ecdsa.PublicKey, error) { return publicKey, nil }

	tests := []struct {
		name       string
		mint       func() (string, error)
		wantIssuer string
		wantTTL    time.Duration
	}{
		{"access default", func() (string, error) { return jwt.GetAccessToken("a", privateKey, 1) }, "powerlab", 3 * time.Hour},
		{"access custom", func() (string, error) { return jwt.GetAccessTokenWithTTL("a", privateKey, 1, 8*time.Hour) }, "powerlab", 8 * time.Hour},
		{"access zero falls back", func() (string, error) { return jwt.GetAccessTokenWithTTL("a", privateKey, 1, 0) }, "powerlab", 3 * time.Hour},
		{"access negative falls back", func() (string, error) { return jwt.GetAccessTokenWithTTL("a", privateKey, 1, -time.Hour) }, "powerlab", 3 * time.Hour},
		{"refresh default", func() (string, error) { return jwt.GetRefreshToken("a", privateKey, 1) }, "refresh", 7 * 24 * time.Hour},
		{"refresh custom", func() (string, error) { return jwt.GetRefreshTokenWithTTL("a", privateKey, 1, 14*24*time.Hour) }, "refresh", 14 * 24 * time.Hour},
		{"refresh zero falls back", func() (string, error) { return jwt.GetRefreshTokenWithTTL("a", privateKey, 1, 0) }, "refresh", 7 * 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, err := tt.mint()
			require.NoError(t, err)
			claims, err := jwt.ParseToken(tok, keyFunc)
			require.NoError(t, err)
			require.Equal(t, tt.wantIssuer, claims.Issuer)
			require.Equal(t, tt.wantTTL, claims.ExpiresAt.Sub(claims.IssuedAt.Time))
		})
	}
}
