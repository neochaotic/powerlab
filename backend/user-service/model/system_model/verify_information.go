package system_model

// VerifyInformation is the JSON payload returned to a client after
// a successful authentication. AccessToken is the short-lived JWT
// (carried as Bearer in subsequent requests); RefreshToken is the
// longer-lived token the client trades in for a fresh AccessToken
// when ExpiresAt approaches.
//
// ExpiresAt is the Unix epoch (seconds) at which the AccessToken
// stops being valid — clients should refresh shortly before that.
type VerifyInformation struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	ExpiresAt    int64  `json:"expires_at"`
}
