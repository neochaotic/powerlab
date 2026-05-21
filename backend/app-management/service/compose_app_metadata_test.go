package service_test

import (
	"fmt"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
)

func appWithPowerLab(t *testing.T, name, xpowerlab string) *service.ComposeApp {
	t.Helper()
	logger.LogInitConsoleOnly()
	y := fmt.Sprintf("name: %s\nservices:\n  web:\n    image: nginx:latest\n", name)
	if xpowerlab != "" {
		y += "x-powerlab:\n" + xpowerlab
	}
	app, err := service.NewComposeAppFromYAML([]byte(y), true, false)
	if err != nil {
		t.Fatalf("NewComposeAppFromYAML: %v", err)
	}
	return app
}

// AuthorType drives the store badge. author=="CasaOS Team" → ByCasaos;
// author==developer → Official; otherwise Community. (NewComposeAppFromYAML
// always synthesizes an x-powerlab extension, so the StoreInfo-error→Unknown
// branch is defensive and unreachable through the public constructor.)
func TestAuthorType(t *testing.T) {
	cases := []struct {
		name string
		ext  string
		want codegen.StoreAppAuthorType
	}{
		{"official", "  author: Alice\n  developer: Alice\n  main: web\n", codegen.Official},
		{"bycasaos", "  author: CasaOS Team\n  developer: Someone\n  main: web\n", codegen.ByCasaos},
		{"community", "  author: Alice\n  developer: Bob\n  main: web\n", codegen.Community},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := appWithPowerLab(t, c.name, c.ext)
			if got := app.AuthorType(); got != c.want {
				t.Errorf("AuthorType() = %q, want %q", got, c.want)
			}
		})
	}
}

// SetStoreAppID: an existing id is returned unchanged (idempotent — install
// must not clobber it); absent an id, the supplied one is set and returned.
func TestSetStoreAppID(t *testing.T) {
	// existing store_app_id wins (idempotent — install must not clobber it)
	existing := appWithPowerLab(t, "hasid", "  author: A\n  store_app_id: keepme\n")
	if id, ok := existing.SetStoreAppID("override"); !ok || id != "keepme" {
		t.Errorf("existing id: got (%q,%v), want (\"keepme\",true)", id, ok)
	}

	// extension present, no id → sets the supplied one
	fresh := appWithPowerLab(t, "noid", "  author: A\n  main: web\n")
	if id, ok := fresh.SetStoreAppID("assigned"); !ok || id != "assigned" {
		t.Errorf("fresh: got (%q,%v), want (\"assigned\",true)", id, ok)
	}
}
