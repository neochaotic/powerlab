// Package docs embeds the per-service OpenAPI specifications and the
// Scalar host page so the gateway binary can serve the API portal at
// runtime without relying on disk paths. Specs are refreshed on every
// build by start.sh from the canonical per-service openapi.yaml.
package docs

import "embed"

//go:embed *.yaml portal.html
var EmbeddedFiles embed.FS

// Spec filenames embedded above. Defined as constants (rather than
// strings built at the call site) so a typo is a compile error.
const (
	GatewaySpecName       = "openapi_gateway.yaml"
	AppManagementSpecName = "openapi_app_management.yaml"
	MessageBusSpecName    = "openapi_message_bus.yaml"
	CoreSpecName          = "openapi_core.yaml"
	LocalStorageSpecName  = "openapi_local_storage.yaml"
	UserServiceSpecName   = "openapi_user_service.yaml"

	PortalTemplateName = "portal.html"
)

// Service is one of the documented backends. The slug doubles as the
// `?service=` query parameter on `/docs` and as the user-facing label
// in the service-switcher dropdown.
type Service struct {
	ID    string
	Label string
	Spec  string
}

// Services is the canonical, ordered list shown in the dropdown. The
// order maps to discoverability priority: gateway first because it
// is the public API surface, then the user-facing services, then
// internal-but-exposed services.
var Services = []Service{
	{ID: "gateway", Label: "Gateway", Spec: GatewaySpecName},
	{ID: "app-management", Label: "App Management", Spec: AppManagementSpecName},
	{ID: "user-service", Label: "User Service", Spec: UserServiceSpecName},
	{ID: "core", Label: "Core", Spec: CoreSpecName},
	{ID: "message-bus", Label: "Message Bus", Spec: MessageBusSpecName},
	{ID: "local-storage", Label: "Local Storage", Spec: LocalStorageSpecName},
}

// LookupService returns the Service with matching ID. If the id is
// unknown, the second return value is false; callers fall back to
// the Gateway service so a stale bookmark or typo still lands on a
// real page rather than a 404.
func LookupService(id string) (Service, bool) {
	for _, s := range Services {
		if s.ID == id {
			return s, true
		}
	}
	return Service{}, false
}
