package common

import (
	"os"
	"path/filepath"
	"testing"
)

// These tests lock the gateway config loader after dropping viper.
// viper v1.20 removed its built-in ini codec, so a v1.21 bump made the
// gateway panic at boot ("decoder not found for this format") on every
// fresh install — caught only on a real runtime, never by build/unit
// tests. The replacement reads the same `section.Key` ini directly.

func writeINI(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, GatewayName+"."+GatewayConfigType)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigFrom_ReadsSectionKeys(t *testing.T) {
	dir := t.TempDir()
	writeINI(t, dir, "[common]\nRuntimePath = /run/custom\n\n[gateway]\nPort = 9123\n")

	cfg, err := loadConfigFrom(filepath.Join(dir, "gateway.ini"))
	if err != nil {
		t.Fatalf("loadConfigFrom: %v", err)
	}
	if got := cfg.GetString(ConfigKeyGatewayPort); got != "9123" {
		t.Errorf("Port = %q, want 9123", got)
	}
	if got := cfg.GetString(ConfigKeyRuntimePath); got != "/run/custom" {
		t.Errorf("RuntimePath = %q, want /run/custom", got)
	}
}

func TestLoadConfigFrom_FallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	// Only Port present — RuntimePath/LogSaveName must come from defaults.
	writeINI(t, dir, "[gateway]\nPort = 80\n")

	cfg, err := loadConfigFrom(filepath.Join(dir, "gateway.ini"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetString(ConfigKeyLogSaveName); got != GatewayName {
		t.Errorf("LogSaveName = %q, want default %q", got, GatewayName)
	}
	// Absent key with no default → empty string, never a panic.
	if got := cfg.GetString("gateway.NoSuchKey"); got != "" {
		t.Errorf("missing key = %q, want empty", got)
	}
}

func TestConfig_SetThenGetRoundTrips(t *testing.T) {
	dir := t.TempDir()
	writeINI(t, dir, "[gateway]\nPort = 80\n")
	cfg, err := loadConfigFrom(filepath.Join(dir, "gateway.ini"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Set(ConfigKeyGatewayPort, "8765")
	if got := cfg.GetString(ConfigKeyGatewayPort); got != "8765" {
		t.Errorf("after Set, Port = %q, want 8765", got)
	}
}

func TestConfig_WriteConfigPersists(t *testing.T) {
	dir := t.TempDir()
	path := writeINI(t, dir, "[gateway]\nPort = 80\n")
	cfg, err := loadConfigFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Set(ConfigKeyGatewayPort, "8765")
	if err := cfg.WriteConfig(); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	// Re-read from disk via a fresh loader — the new value must persist.
	reloaded, err := loadConfigFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.GetString(ConfigKeyGatewayPort); got != "8765" {
		t.Errorf("persisted Port = %q, want 8765", got)
	}
}

func TestLoadConfig_ResolvesViaConfigPathEnv(t *testing.T) {
	dir := t.TempDir()
	writeINI(t, dir, "[gateway]\nPort = 7777\n")
	t.Setenv("CASAOS_CONFIG_PATH", dir)
	// Run from a temp cwd with no gateway.ini so the env path is the hit.
	other := t.TempDir()
	t.Chdir(other)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := cfg.GetString(ConfigKeyGatewayPort); got != "7777" {
		t.Errorf("Port = %q, want 7777", got)
	}
}
