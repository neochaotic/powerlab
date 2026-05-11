package jwt_test

import (
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJwtFlow(t *testing.T) {
	// Generate a key pair
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)

	// Generate access and refresh tokens
	username := "testuser"
	id := 1

	accessToken, err := jwt.GetAccessToken(username, privateKey, id)
	require.NoError(t, err)

	refreshToken, err := jwt.GetRefreshToken(username, privateKey, id)
	require.NoError(t, err)

	// Generate JWKS JSON
	jwksJSON, err := jwt.GenerateJwksJSON(publicKey)
	require.NoError(t, err)

	// Serve the JWKS JSON
	server := httptest.NewServer(jwt.JWKSHandler(jwksJSON))
	defer server.Close()

	// Consume the JWKS JSON
	response, err := http.Get(server.URL + "/" + jwt.JWKSPath)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)

	var jwks jwt.JWKS
	err = json.NewDecoder(response.Body).Decode(&jwks)
	require.NoError(t, err)
	require.Len(t, jwks.Keys, 1)

	// Extract the public key from the JWKS JSON
	consumedPublicKey, err := jwt.PublicKeyFromJwksJSON(jwksJSON)
	require.NoError(t, err)

	// Validate the access token via the access path — must pass
	// the AcceptedAccessIssuers gate added in #246.
	valid, claims, err := jwt.Validate(accessToken, func() (*ecdsa.PublicKey, error) { return consumedPublicKey, nil })
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, username, claims.Username)
	assert.Equal(t, id, claims.ID)

	// Refresh tokens go through ParseToken (used by the dedicated
	// refresh endpoint), NOT Validate. Validate is the access-token
	// gate and must REJECT refresh tokens — a refresh token used as
	// an Authorization header would be a security bug.
	claims, err = jwt.ParseToken(refreshToken, func() (*ecdsa.PublicKey, error) { return consumedPublicKey, nil })
	require.NoError(t, err)
	assert.Equal(t, username, claims.Username)
	assert.Equal(t, id, claims.ID)
	assert.Equal(t, "refresh", claims.Issuer)

	// Refresh-token-as-access regression guard.
	rejectValid, _, rejectErr := jwt.Validate(refreshToken, func() (*ecdsa.PublicKey, error) { return consumedPublicKey, nil })
	assert.Error(t, rejectErr, "refresh token must NOT pass the access-token gate")
	assert.False(t, rejectValid)
}

func TestInvalidToken(t *testing.T) {
	// Generate a key pair
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)

	// Generate access token
	username := "testuser"
	id := 1

	accessToken, err := jwt.GetAccessToken(username, privateKey, id)
	require.NoError(t, err)

	// Modify the token to make it invalid
	invalidToken := accessToken[:len(accessToken)-5] + "abcde"

	// Validate the invalid token
	valid, claims, err := jwt.Validate(invalidToken, func() (*ecdsa.PublicKey, error) { return publicKey, nil })
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Nil(t, claims)
}

func TestJWTMiddlewareWithValidToken(t *testing.T) {
	// Generate a key pair
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)

	// Generate access token
	username := "testuser"
	id := 1

	accessToken, err := jwt.GetAccessToken(username, privateKey, id)
	require.NoError(t, err)

	// Mock publicKeyFunc to return a public key.
	mockPublicKeyFunc := func() (*ecdsa.PublicKey, error) {
		// You can use a pre-generated public key here or generate a new key pair for testing.
		return publicKey, nil
	}

	// Create a Gin test context and a response recorder.
	router := echo.New()
	router.Use(jwt.JWT(mockPublicKeyFunc))
	router.GET("/test", func(c echo.Context) error {
		return c.JSON(http.StatusOK, model.Result{
			Success: common_err.SUCCESS,
			Message: "success",
		})
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", accessToken)
	respRecorder := httptest.NewRecorder()

	router.ServeHTTP(respRecorder, req)

	// Assert the response status code and content.
	assert.Equal(t, http.StatusOK, respRecorder.Code)

	result := model.Result{}
	err = json.Unmarshal(respRecorder.Body.Bytes(), &result)

	assert.Equal(t, result.Success, common_err.SUCCESS)
	require.NoError(t, err)
}

// Closes #246 — JWT issuer was unconditionally "casaos" identifying
// every PowerLab token as CasaOS. The bridging release issues NEW
// tokens with iss="powerlab" while CONTINUING to accept legacy
// "casaos"-issued tokens so existing sessions don't get logged out
// on upgrade. Drop "casaos" from the accepted set after the
// bridging window (tracked in CHANGELOG).
func TestGetAccessToken_IssuesWithPowerLabIssuer(t *testing.T) {
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)

	token, err := jwt.GetAccessToken("alice", privateKey, 42)
	require.NoError(t, err)

	claims, err := jwt.ParseToken(token, func() (*ecdsa.PublicKey, error) { return publicKey, nil })
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "powerlab", claims.Issuer,
		"new tokens MUST be issued with iss=powerlab — #246")
	assert.NotEqual(t, "casaos", claims.Issuer,
		"iss=casaos branding leak must not regress")
}

func TestValidate_AcceptsLegacyCasaosIssuer(t *testing.T) {
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)

	// Forge a legacy-style token by calling GenerateToken directly
	// with iss="casaos" — what older PowerLab binaries produced.
	legacy, err := jwt.GenerateToken("bob", privateKey, 7, "casaos", 3*time.Hour)
	require.NoError(t, err)

	valid, claims, err := jwt.Validate(legacy, func() (*ecdsa.PublicKey, error) { return publicKey, nil })
	assert.NoError(t, err)
	assert.True(t, valid, "legacy iss=casaos tokens must remain valid during the bridging release")
	require.NotNil(t, claims)
	assert.Equal(t, "bob", claims.Username)
}

func TestValidate_AcceptsCurrentPowerLabIssuer(t *testing.T) {
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)

	token, err := jwt.GetAccessToken("carol", privateKey, 99)
	require.NoError(t, err)

	valid, claims, err := jwt.Validate(token, func() (*ecdsa.PublicKey, error) { return publicKey, nil })
	assert.NoError(t, err)
	assert.True(t, valid)
	require.NotNil(t, claims)
	assert.Equal(t, "carol", claims.Username)
}

func TestValidate_RejectsUnknownIssuer(t *testing.T) {
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)

	// A signature-valid token from a foreign issuer must not be
	// accepted as an access token. Refresh tokens go through a
	// dedicated endpoint and don't hit the access-token path.
	hostile, err := jwt.GenerateToken("eve", privateKey, 1, "evil-corp", 3*time.Hour)
	require.NoError(t, err)

	valid, claims, err := jwt.Validate(hostile, func() (*ecdsa.PublicKey, error) { return publicKey, nil })
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Nil(t, claims)
}

func TestJWTMiddlewareWithInvalidToken(t *testing.T) {
	// Generate a key pair
	_, publicKey, err := jwt.GenerateKeyPair()
	require.NoError(t, err)

	// Mock publicKeyFunc to return a public key.
	mockPublicKeyFunc := func() (*ecdsa.PublicKey, error) {
		// You can use a pre-generated public key here or generate a new key pair for testing.
		return publicKey, nil
	}

	// Create a Gin test context and a response recorder.
	router := echo.New()
	router.Use(jwt.JWT(mockPublicKeyFunc))

	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.JSON(http.StatusOK, echo.Map{"message": "success"})
			return next(c)
		}
	})
	router.GET("/test", func(c echo.Context) error {
		assert.Fail(t, "this handler should not be called")
		return nil
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "invalid_token")
	respRecorder := httptest.NewRecorder()

	router.ServeHTTP(respRecorder, req)

	// Assert the response status code and content.
	assert.Equal(t, http.StatusUnauthorized, respRecorder.Code)

	result := model.Result{}
	err = json.Unmarshal(respRecorder.Body.Bytes(), &result)

	assert.Equal(t, result.Success, common_err.ERROR_AUTH_TOKEN)
	require.NoError(t, err)
}
