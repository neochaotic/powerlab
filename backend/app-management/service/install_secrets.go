package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SubstituteSecrets resolves Umbrel-runtime per-app secret placeholders
// inside an install-time compose YAML by generating + persisting a
// random secret per (app, placeholder name) tuple. The first install
// of a given app writes secrets to `<secretsDir>/<appID>.env`; later
// installs (re-install, upgrade) read the same file, so the secret
// stays STABLE across the app's lifetime — encrypted user data and
// JWT-signed sessions survive upgrades.
//
// Why install-time vs sync-time: cryptographic invariants (JWT
// signing keys, NextAuth secrets) MUST stay stable for the lifetime
// of a given app on a given install. Sync-time substitution would
// regenerate the secret on every weekly catalog refresh → user
// sessions die, encrypted data unrecoverable. Install-time per-app
// random + file persistence gives us both security (per-install
// unique, not catalog-wide shared) and stability (file survives
// re-installs).
//
// Placeholders handled (Sprint 13.4.x audit findings):
//   - ${APP_SEED}                  (49 apps — NextAuth secrets, salts, …)
//   - ${APP_PASSWORD}              (39 apps — admin/db passwords)
//   - ${APP_<NAME>_DB_PASSWORD}    (sibling-app DB cred, e.g. ghostfolio)
//   - ${APP_<NAME>_DB_USERNAME}    (same)
//   - ${APP_<NAME>_DB_DATABASE_NAME} (same)
//
// Placeholders NOT handled (left to the upstream defaults or future
// install-time UI flows): ${APP_DOMAIN} (needs the user's actual
// hostname), ${TOR_PROXY_*} (Tor isn't a PowerLab service yet).
//
// The function is pure-input/pure-output relative to the secrets
// file: read or generate-then-write, substitute the YAML bytes,
// return. No SSE log lines, no caller-state mutation.
var secretPlaceholderRE = regexp.MustCompile(`\$\{(APP_SEED|APP_PASSWORD|APP_[A-Z0-9_]+_DB_PASSWORD|APP_[A-Z0-9_]+_DB_USERNAME|APP_[A-Z0-9_]+_DB_DATABASE_NAME)\}`)

// SubstituteSecrets is the public entry point. See file-level doc.
func SubstituteSecrets(yaml []byte, appID, secretsDir string) ([]byte, error) {
	if !secretPlaceholderRE.Match(yaml) {
		return yaml, nil // fast path: nothing to substitute
	}

	// 1. Collect every unique placeholder var name in the YAML.
	matches := secretPlaceholderRE.FindAllSubmatch(yaml, -1)
	wanted := map[string]struct{}{}
	for _, m := range matches {
		wanted[string(m[1])] = struct{}{}
	}

	// 2. Load existing secrets (if any) — idempotent re-install.
	secretsPath := filepath.Join(secretsDir, appID+".env")
	existing, err := loadSecretsFile(secretsPath)
	if err != nil {
		return nil, fmt.Errorf("read secrets file %s: %w", secretsPath, err)
	}

	// 3. For each wanted placeholder, ensure we have a value.
	for name := range wanted {
		if _, ok := existing[name]; ok {
			continue
		}
		val, err := generateSecret(name)
		if err != nil {
			return nil, fmt.Errorf("generate secret for %s: %w", name, err)
		}
		existing[name] = val
	}

	// 4. Persist back. Write atomically so a crash mid-install
	// doesn't leave a half-written secrets file.
	if err := writeSecretsFile(secretsPath, existing); err != nil {
		return nil, fmt.Errorf("write secrets file %s: %w", secretsPath, err)
	}

	// 5. Substitute every reference in the YAML. Multi-occurrence
	// of the same placeholder gets the same value.
	out := secretPlaceholderRE.ReplaceAllFunc(yaml, func(match []byte) []byte {
		// extract the inner var name (re-parse — cheap enough)
		sub := secretPlaceholderRE.FindSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		name := string(sub[1])
		val, ok := existing[name]
		if !ok {
			return match // shouldn't happen given step 3
		}
		return []byte(val)
	})
	return out, nil
}

// generateSecret picks an appropriate random format for the
// placeholder kind. Long hex strings (32 bytes = 64 hex chars) for
// SEED, shorter (16 bytes = 32 hex) for PASSWORD-class. DB usernames
// get a short alphabetic ID. The format choices match what Umbrel's
// runtime uses, so apps reading the env work without further
// adjustments.
func generateSecret(name string) (string, error) {
	switch {
	case strings.HasSuffix(name, "_DB_USERNAME"):
		// Alphabetic-only, 12 chars — Postgres accepts but doesn't
		// quote. Avoids needing quote handling in the YAML.
		return randomAlpha(12)
	case strings.HasSuffix(name, "_DB_DATABASE_NAME"):
		return randomAlpha(12)
	case strings.HasSuffix(name, "_DB_PASSWORD"), name == "APP_PASSWORD":
		// 16-byte hex = 32 chars. Easily typeable if the user wants
		// to copy from the secrets file.
		return randomHex(16)
	case name == "APP_SEED":
		// 32-byte hex = 64 chars. Matches Umbrel's APP_SEED format.
		return randomHex(32)
	default:
		return randomHex(16)
	}
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randomAlpha(n int) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b), nil
}

// loadSecretsFile parses a `KEY=VALUE\n`-line file into a map. Returns
// an empty map (NOT an error) when the file doesn't exist — first
// install. Other I/O errors propagate.
func loadSecretsFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		out[line[:eq]] = line[eq+1:]
	}
	return out, nil
}

// writeSecretsFile writes the map back atomically (tempfile + rename).
// Directory is created with 0700 perms so other users on the host
// can't read app secrets; file is 0600 same rationale.
func writeSecretsFile(path string, secrets map[string]string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".secrets-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	fmt.Fprintf(tmp, "# PowerLab install-time secrets — DO NOT commit, DO NOT share.\n")
	fmt.Fprintf(tmp, "# Auto-generated; delete the file to rotate on next install.\n")
	for k, v := range secrets {
		fmt.Fprintf(tmp, "%s=%s\n", k, v)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
