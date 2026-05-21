package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/service"
)

// SubstituteSecrets regression locks. v0.6.6 / Sprint 13.4.x audit
// surfaced 49 apps using ${APP_SEED} and 39 using ${APP_PASSWORD}
// that stayed literal in the install-time compose, breaking
// cryptographic invariants (NextAuth secrets, JWT signing,
// auto-init admin passwords). Substitute at install time with
// per-app random + persist so re-installs reuse the same value
// (encrypted data + sessions survive upgrades).

func TestSubstituteSecrets_NoPlaceholders_ReturnsInputUnchanged(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte("services:\n  app:\n    image: nginx:latest\n")
	out, err := service.SubstituteSecrets(yaml, "foo", dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(yaml) {
		t.Errorf("expected unchanged YAML, got: %s", out)
	}
}

func TestSubstituteSecrets_AppSeed_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`services:
  app:
    image: example:1
    environment:
      - NEXTAUTH_SECRET=${APP_SEED}
`)
	out, err := service.SubstituteSecrets(yaml, "blinko", dir)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "${APP_SEED}") {
		t.Errorf("APP_SEED placeholder should be substituted, got:\n%s", s)
	}
	// 64-char hex (32 bytes)
	if !strings.Contains(s, "NEXTAUTH_SECRET=") {
		t.Fatalf("expected NEXTAUTH_SECRET= in output, got:\n%s", s)
	}
	// secrets file should exist with 0600 mode
	secretsFile := filepath.Join(dir, "blinko.env")
	info, err := os.Stat(secretsFile)
	if err != nil {
		t.Fatalf("expected secrets file at %s: %v", secretsFile, err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("secrets file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestSubstituteSecrets_Idempotent_ReusesExistingSecret(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`services:
  app:
    image: example:1
    environment:
      - SECRET=${APP_SEED}
`)
	out1, err := service.SubstituteSecrets(yaml, "blinko", dir)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := service.SubstituteSecrets(yaml, "blinko", dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(out1) != string(out2) {
		t.Errorf("second call should produce identical output (encrypted data depends on stability):\nfirst:\n%s\nsecond:\n%s", out1, out2)
	}
}

func TestSubstituteSecrets_SameRefAcrossYAML_GetsSameValue(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`services:
  app:
    environment:
      - JWT=${APP_SEED}
      - SESSION=${APP_SEED}
`)
	out, err := service.SubstituteSecrets(yaml, "foo", dir)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Extract the substituted values for JWT and SESSION
	jwt := extractEnvValue(t, s, "JWT")
	session := extractEnvValue(t, s, "SESSION")
	if jwt == "" || session == "" {
		t.Fatalf("could not extract env values:\n%s", s)
	}
	if jwt != session {
		t.Errorf("multi-reference to ${APP_SEED} must produce same value (signed JWTs must verify):\nJWT=%s\nSESSION=%s", jwt, session)
	}
}

func TestSubstituteSecrets_DifferentApps_GetDifferentSecrets(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte("services:\n  app:\n    environment:\n      - S=${APP_SEED}\n")
	outA, _ := service.SubstituteSecrets(yaml, "appA", dir)
	outB, _ := service.SubstituteSecrets(yaml, "appB", dir)
	if string(outA) == string(outB) {
		t.Errorf("different apps must get different secrets (otherwise compromising one compromises all)")
	}
}

func TestSubstituteSecrets_AppPassword_GeneratesShorter(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte("services:\n  app:\n    environment:\n      - ADMIN_PASS=${APP_PASSWORD}\n")
	out, err := service.SubstituteSecrets(yaml, "foo", dir)
	if err != nil {
		t.Fatal(err)
	}
	v := extractEnvValue(t, string(out), "ADMIN_PASS")
	if len(v) != 32 {
		t.Errorf("APP_PASSWORD should produce 32-char hex (16 bytes), got %d chars: %q", len(v), v)
	}
}

func TestSubstituteSecrets_SiblingAppDBCredentials(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: ${APP_GHOSTFOLIO_DB_DATABASE_NAME}
      POSTGRES_USER: ${APP_GHOSTFOLIO_DB_USERNAME}
      POSTGRES_PASSWORD: ${APP_GHOSTFOLIO_DB_PASSWORD}
`)
	out, err := service.SubstituteSecrets(yaml, "ghostfolio", dir)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, k := range []string{"DATABASE_NAME", "USERNAME", "PASSWORD"} {
		if strings.Contains(s, "${APP_GHOSTFOLIO_DB_"+k+"}") {
			t.Errorf("sibling-app DB placeholder %s should be substituted, got:\n%s", k, s)
		}
	}
}

func TestSubstituteSecrets_PreservesUnrelatedPlaceholders(t *testing.T) {
	// ${APP_DATA_DIR}, ${APP_*_PORT}, ${DEVICE_*} are handled by
	// their own substitutors. SubstituteSecrets MUST leave them alone.
	dir := t.TempDir()
	yaml := []byte(`services:
  app:
    environment:
      - DATA=${APP_DATA_DIR}/x
      - PORT=${APP_FOO_PORT}
      - HOST=${DEVICE_DOMAIN_NAME}
      - SECRET=${APP_SEED}
`)
	out, err := service.SubstituteSecrets(yaml, "foo", dir)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, ph := range []string{"${APP_DATA_DIR}", "${APP_FOO_PORT}", "${DEVICE_DOMAIN_NAME}"} {
		if !strings.Contains(s, ph) {
			t.Errorf("unrelated placeholder %s should be preserved (other transforms own it), got:\n%s", ph, s)
		}
	}
	if strings.Contains(s, "${APP_SEED}") {
		t.Errorf("APP_SEED should be substituted, got:\n%s", s)
	}
}

func TestSubstituteSecrets_DeleteSecretsFile_RotatesOnNextCall(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte("services:\n  app:\n    environment:\n      - S=${APP_SEED}\n")
	out1, _ := service.SubstituteSecrets(yaml, "foo", dir)
	// Simulate operator-triggered rotation: delete the secrets file
	os.Remove(filepath.Join(dir, "foo.env"))
	out2, _ := service.SubstituteSecrets(yaml, "foo", dir)
	if string(out1) == string(out2) {
		t.Errorf("after secrets-file deletion, secret should rotate")
	}
}

// extractEnvValue is a small test-only helper that finds "K=V" in a
// YAML-ish blob and returns V (without surrounding quotes). It's
// loose on purpose — only needs to work for the test fixtures here.
func extractEnvValue(t *testing.T, blob, key string) string {
	t.Helper()
	for _, line := range strings.Split(blob, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		if !strings.HasPrefix(line, key+"=") {
			continue
		}
		return strings.TrimSpace(line[len(key)+1:])
	}
	return ""
}

// --- Adversarial / security locks -------------------------------------

// The secrets directory must be created 0700 so other users on the host
// can't enumerate which apps have secret files (the files themselves are
// 0600, but a world-readable dir leaks app inventory + invites races).
func TestSubstituteSecrets_SecretsDirIs0700(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "created", "by", "write") // does not exist yet
	yaml := []byte("services:\n  app:\n    environment:\n      - S=${APP_SEED}\n")
	if _, err := service.SubstituteSecrets(yaml, "appx", dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected secrets dir created: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("SECURITY: secrets dir mode = %v, want 0700", info.Mode().Perm())
	}
}

// The placeholder regex must be exact. Over-broadening would substitute
// random secrets into unintended env vars (breaking apps) or leak
// generated values where they don't belong; under-matching reintroduces
// the literal-placeholder bug. Lock the boundary: near-miss tokens MUST
// be left untouched AND must not even trigger the substitution path
// (no secrets file written).
func TestSubstituteSecrets_RegexDoesNotOverMatch(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte(`services:
  app:
    environment:
      - A=${APP_SEEDX}
      - B=${app_seed}
      - C=${APP_SEED
      - D=${APP_TOKEN}
      - E=${SOMETHING_APP_SEED}
`)
	out, err := service.SubstituteSecrets(yaml, "appx", dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(yaml) {
		t.Errorf("near-miss placeholders should be untouched.\n got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "appx.env")); !os.IsNotExist(err) {
		t.Errorf("no secrets file should be written when nothing matched (err=%v)", err)
	}
}

// A persisted secret value containing regex-replacement metacharacters
// ($1, ${name}) must be substituted LITERALLY. This locks the use of
// ReplaceAllFunc over ReplaceAll — the latter would expand $1/${name} as
// capture-group references and silently corrupt the secret (and could
// inject attacker-influenced expansions if a value were ever externally
// sourced).
func TestSubstituteSecrets_DollarInValueSubstitutedLiterally(t *testing.T) {
	dir := t.TempDir()
	// Pre-seed the secrets file with a value full of $ metachars.
	const tricky = "ab$1cd${APP_SEED}ef$0"
	if err := os.WriteFile(filepath.Join(dir, "appx.env"),
		[]byte("APP_PASSWORD="+tricky+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	yaml := []byte("services:\n  app:\n    environment:\n      - PW=${APP_PASSWORD}\n")
	out, err := service.SubstituteSecrets(yaml, "appx", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "PW="+tricky) {
		t.Errorf("expected literal %q in output (no $-expansion), got:\n%s", tricky, out)
	}
}
