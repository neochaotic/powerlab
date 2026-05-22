package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
)

// ExtractTokenFromRequest reads the JWT from an incoming request,
// accepting both the legacy raw-token form (`Authorization: <jwt>`)
// and the standard RFC 6750 form (`Authorization: Bearer <jwt>`,
// case-insensitive on "Bearer"). Falls back to the `?token=` query
// parameter when the Authorization header is empty (the EventSource
// pattern — browser API can't set headers on SSE connections).
//
// This is the single source of truth for header → token extraction.
// All JWT middleware sites in the repo (gateway, app-management,
// message-bus, core, user-service) call this so a future "raw
// Bearer prefix" regression can't happen in one place while the
// others stay correct. See #342.
func ExtractTokenFromRequest(c echo.Context) string {
	auth := c.Request().Header.Get(echo.HeaderAuthorization)
	if auth != "" {
		// Case-insensitive "Bearer " prefix per RFC 6750 §2.1.
		const prefix = "Bearer "
		if len(auth) >= len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
			return auth[len(prefix):]
		}
		return auth
	}
	// HttpOnly cookie — preferred for browser-driven GETs (media
	// <video>/<img>, downloads) so the JWT never lands in the URL,
	// browser history, or access logs (#35). Checked before the legacy
	// ?token= query fallback (which SSE/WebSocket still rely on).
	if ck, err := c.Cookie("access_token"); err == nil && ck.Value != "" {
		return ck.Value
	}
	return c.QueryParam("token")
}

// JWK is a single ECDSA P-256 public key in the standard JWK
// envelope. PowerLab's JWKS only ever contains one key.
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// JWKS is the standard "keys" wrapper served at JWKSPath.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWKSPath is the URL path the user-service exposes its public key
// on. Other services fetch this to verify JWTs locally.
const JWKSPath = ".well-known/jwks.json"

// JWT returns an echo middleware that verifies Authorization
// headers (or ?token query param) against the publicKeyFunc-
// provided key. Skips loopback (::1, 127.0.0.1) so on-host
// admin tools don't need a token. Sets X-User-Id on the
// inbound request from the verified claims so downstream
// handlers can read it without re-parsing.
func JWT(publicKeyFunc func() (*ecdsa.PublicKey, error)) echo.MiddlewareFunc {
	return echojwt.WithConfig(
		echojwt.Config{
			Skipper: func(c echo.Context) bool {
				return c.RealIP() == "::1" || c.RealIP() == "127.0.0.1"
			},
			ParseTokenFunc: func(c echo.Context, token string) (interface{}, error) {
				valid, claims, err := Validate(token, publicKeyFunc)
				if err != nil || !valid {
					message := "token is invalid"
					c.JSON(http.StatusUnauthorized, model.Result{Success: common_err.ERROR_AUTH_TOKEN, Message: message})
					return nil, echo.ErrUnauthorized
				}
				c.Request().Header.Set("user_id", strconv.Itoa(claims.ID))

				return claims, nil
			},
			TokenLookupFuncs: []echo_middleware.ValuesExtractor{
				func(c echo.Context) ([]string, error) {
					return []string{ExtractTokenFromRequest(c)}, nil
				},
			},
		},
	)
}

// GenerateKeyPair returns a fresh ECDSA P-256 keypair. user-service
// calls this on first boot to seed the persistent keypair (ADR-0020).
func GenerateKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating key pair: %w", err)
	}

	publicKey := &privateKey.PublicKey

	return privateKey, publicKey, nil
}

// GenerateJwksJSON serialises publicKey as a JWKS document for the
// .well-known endpoint. P-256 only.
func GenerateJwksJSON(publicKey *ecdsa.PublicKey) ([]byte, error) {
	jwk := JWK{
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(publicKey.X.Bytes()),
		Y:   base64.RawURLEncoding.EncodeToString(publicKey.Y.Bytes()),
	}

	jwks := JWKS{
		Keys: []JWK{jwk},
	}

	return json.Marshal(jwks)
}

// PublicKeyFromJwksJSON inverts GenerateJwksJSON — reads the first
// key from a JWKS document into an ecdsa.PublicKey.
func PublicKeyFromJwksJSON(jwksJSON []byte) (*ecdsa.PublicKey, error) {
	var jwks JWKS
	err := json.Unmarshal(jwksJSON, &jwks)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling JWKS JSON: %w", err)
	}

	if len(jwks.Keys) == 0 {
		return nil, fmt.Errorf("no keys in JWKS")
	}

	jwk := jwks.Keys[0]

	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("error decoding X: %w", err)
	}

	y, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("error decoding Y: %w", err)
	}

	publicKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}

	return publicKey, nil
}

// JWKSHandler serves jwksJSON at the JWKSPath endpoint. user-service
// mounts this once at startup; the served bytes never change for the
// lifetime of the process (rotating the keypair is a restart event).
func JWKSHandler(jwksJSON []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(jwksJSON)
		if err != nil {
			http.Error(w, "Error writing JWKS JSON", http.StatusInternalServerError)
			return
		}
	})
}
