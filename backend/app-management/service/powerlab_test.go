package service_test

import (
	"testing"

	"github.com/IceWhaleTech/CasaOS-AppManagement/service"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"gotest.tools/v3/assert"
)

// --- web: extension priority ---

func TestStoreInfoWithWebTag(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
web:
  title:
    en_us: Web Title
x-casaos:
  title:
    en_us: CasaOS Title
`
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, true)
	assert.NilError(t, err)

	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)

	// Should prioritize 'web' tag
	assert.Equal(t, "Web Title", storeInfo.Title["en_us"])
}

func TestStoreInfoWithOnlyWebTag(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
web:
  title:
    en_us: Only Web
`
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, true)
	assert.NilError(t, err)

	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)

	assert.Equal(t, "Only Web", storeInfo.Title["en_us"])
}

func TestStoreInfoWithOnlyXCasaOS(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
x-casaos:
  title:
    en_us: Legacy Title
`
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, true)
	assert.NilError(t, err)

	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)

	assert.Equal(t, "Legacy Title", storeInfo.Title["en_us"])
}

// --- port alias ---

func TestStoreInfoWithPortField(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
web:
  port: "9000"
  title:
    en_us: Port Test
`
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, true)
	assert.NilError(t, err)

	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)

	assert.Equal(t, "9000", storeInfo.PortMap)
}

func TestStoreInfoPortMapTakesPrecedenceOverPort(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
web:
  port: "9000"
  port_map: "8080"
  title:
    en_us: Precedence Test
`
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, true)
	assert.NilError(t, err)

	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)

	// port_map should win over port
	assert.Equal(t, "8080", storeInfo.PortMap)
}

// --- SetStoreAppID with web: ---

func TestSetStoreAppIDWithWebOnly(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
web:
  title:
    en_us: Web App
`
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, true)
	assert.NilError(t, err)

	id, ok := app.SetStoreAppID("test-app")
	assert.Assert(t, ok, "SetStoreAppID should succeed with web: extension")
	assert.Equal(t, "test-app", id)
}

// --- SetUncontrolled with web: ---

func TestSetUncontrolledWithWebOnly(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
web:
  title:
    en_us: Web App
`
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, true)
	assert.NilError(t, err)

	err = app.SetUncontrolled(true)
	assert.NilError(t, err, "SetUncontrolled should not panic or error with web: extension")

	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)
	assert.Assert(t, *storeInfo.IsUncontrolled)
}

// --- No extension at all ---

func TestStoreInfoWithoutAnyExtension(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
`
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, true)
	assert.NilError(t, err)

	// NewComposeAppFromYAML auto-creates a title from a random name, so StoreInfo should succeed
	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)
	// Title should be auto-generated (non-empty)
	assert.Assert(t, len(storeInfo.Title) > 0, "Should have auto-generated title")
}

// --- web: alias with validation enabled ---

func TestWebAliasWithValidationEnabled(t *testing.T) {
	logger.LogInitConsoleOnly()
	yaml := `
version: '3'
services:
  app:
    image: nginx
web:
  title:
    en_us: Validated Web
`
	// skipValidation=false is the real-world scenario
	app, err := service.NewComposeAppFromYAML([]byte(yaml), true, false)
	assert.NilError(t, err, "web: alias should be transparently converted to x-web: before validation")

	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)

	assert.Equal(t, "Validated Web", storeInfo.Title["en_us"])
}
