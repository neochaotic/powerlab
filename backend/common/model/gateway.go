package model

// Route is a gateway path → upstream target mapping registered by a
// backend service at startup via the management API.
type Route struct {
	Path   string `json:"path" binding:"required"`
	Target string `json:"target" binding:"required"`
}

// ChangePortRequest is the body of the gateway "change listening
// port" admin endpoint.
type ChangePortRequest struct {
	Port string `json:"port" binding:"required"`
}
