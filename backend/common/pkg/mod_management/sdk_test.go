package modmanagement_test

import (
	"os"
	"testing"

	modmanagement "github.com/neochaotic/powerlab/backend/common/pkg/mod_management"
	"github.com/stretchr/testify/assert"
)

const gatewayURLFile = "/var/run/casaos/management.url"

func skipIfGatewayNotRunning(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skip("skipped in CI")
	}
	if _, err := os.Stat(gatewayURLFile); err != nil {
		t.Skip("gateway not running (management.url not found)")
	}
}

func TestInstallableModules(t *testing.T) {
	skipIfGatewayNotRunning(t)
	client, err := modmanagement.NewClient(modmanagement.ModManagementClientOpts{})
	assert.NoError(t, err)
	modules, err := client.InstallableModules()
	assert.NoError(t, err)

	t.Log(modules)
}

func TestInstallModule(t *testing.T) {
	skipIfGatewayNotRunning(t)
	err := modmanagement.RequireModule("doconverter", "/var/run/casaos")
	assert.NoError(t, err)
}

func TestInstallNoExistModule(t *testing.T) {
	skipIfGatewayNotRunning(t)
	err := modmanagement.RequireModule("abc", "/var/run/casaos")
	assert.ErrorIs(t, err, modmanagement.ErrModuleNoInStore)
}
