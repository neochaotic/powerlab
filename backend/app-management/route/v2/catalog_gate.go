package v2

import (
	"github.com/neochaotic/powerlab/backend/app-management/service"
)

// catalogIfEnabled gates the store-facing catalog on the operator's
// opt-in flag (config.ServerInfo.CatalogEnabled). Disabled by default:
// the store ships dark and returns an empty catalog until the operator
// enables it (first-run prompt / Settings → Catalog). The catalog data
// stays loaded in memory regardless — only the UI-facing listing is
// gated — so enabling is instant and internal lookups are unaffected.
func catalogIfEnabled(catalog map[string]*service.ComposeApp, enabled bool) map[string]*service.ComposeApp {
	if !enabled {
		return map[string]*service.ComposeApp{}
	}
	return catalog
}
