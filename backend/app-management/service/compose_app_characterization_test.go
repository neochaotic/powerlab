package service_test

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
)

// Characterization lock for NewComposeAppFromYAML. This pins the observable
// output of compose parsing so the compose-go v1→v2 migration can be proven
// behavior-preserving: this test must be green BEFORE the bump (on main) and
// green AFTER. It does not assert new behavior — it freezes current behavior.
func TestComposeAppParsingCharacterization(t *testing.T) {
	logger.LogInitConsoleOnly()

	const y = `name: chartest
services:
  web:
    image: nginx:1.27
    ports:
      - "8080:80"
    environment:
      - FOO=bar
  db:
    image: postgres:16
x-powerlab:
  author: Alice
  developer: Alice
  store_app_id: chartest-id
  main: web
`

	app, err := service.NewComposeAppFromYAML([]byte(y), true, false)
	if err != nil {
		t.Fatalf("NewComposeAppFromYAML: %v", err)
	}

	// Service set + images (the field most affected by slice→map in v2).
	got := map[string]string{}
	for _, svc := range app.Services {
		got[svc.Name] = svc.Image
	}
	want := map[string]string{"web": "nginx:1.27", "db": "postgres:16"}
	if len(got) != len(want) {
		t.Fatalf("service count = %d, want %d (%v)", len(got), len(want), got)
	}
	for name, img := range want {
		if got[name] != img {
			t.Errorf("service %q image = %q, want %q", name, got[name], img)
		}
	}

	// x-powerlab extension survives parsing (store provenance + main service).
	if app.AuthorType() == "" {
		t.Error("AuthorType empty — x-powerlab extension lost in parse")
	}
	if id, ok := app.SetStoreAppID("override"); !ok || id != "chartest-id" {
		t.Errorf("store_app_id = (%q,%v), want (chartest-id,true)", id, ok)
	}
}
