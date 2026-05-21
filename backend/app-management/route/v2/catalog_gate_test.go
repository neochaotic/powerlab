package v2

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/service"
)

// TestCatalogIfEnabled locks the opt-in gate: the store returns an empty
// catalog when disabled (default), and the full catalog once enabled.
func TestCatalogIfEnabled(t *testing.T) {
	full := map[string]*service.ComposeApp{"booklore": {}, "ghost": {}, "mainsail": {}}

	if got := catalogIfEnabled(full, false); len(got) != 0 {
		t.Fatalf("catalog disabled → want empty store, got %d apps", len(got))
	}

	if got := catalogIfEnabled(full, true); len(got) != len(full) {
		t.Fatalf("catalog enabled → want %d apps, got %d", len(full), len(got))
	}
}
