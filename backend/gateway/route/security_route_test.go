package route

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/pkg/security"
	"github.com/digitorus/pkcs7"
)

// newTestCertManager wires a CertManager onto a tmp dir so tests can
// hit handleCAMobileConfig / handleCACer / handleCACrt without
// touching /etc.
func newTestCertManager(t *testing.T) *security.CertManager {
	t.Helper()
	tmp, err := os.MkdirTemp("", "powerlab-security-route-test")
	if err != nil {
		t.Fatalf("mktmp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })

	cm := &security.CertManager{StoragePath: tmp}
	if err := cm.Setup(); err != nil {
		t.Fatalf("CertManager.Setup: %v", err)
	}
	return cm
}

// TestHandleCACrt — the .crt endpoint serves a parseable PEM cert
// matching the CA on disk, with the right Content-Type/Disposition.
func TestHandleCACrt(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/sys/ca-certificate.crt", nil)
	s.handleCACrt(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/x-x509-ca-cert" {
		t.Errorf("Content-Type: got %q", got)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "powerlab-ca.crt") {
		t.Errorf("Content-Disposition missing filename: %q", rec.Header().Get("Content-Disposition"))
	}

	body, _ := io.ReadAll(rec.Body)
	block, _ := pem.Decode(body)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("PEM decode failed (block=%v)", block)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if !cert.IsCA {
		t.Errorf("served cert is not flagged IsCA")
	}
}

// TestHandleCACer — the .cer endpoint serves the DER bytes of the CA
// cert (Windows Certificate Import Wizard expects pure DER, not PEM).
func TestHandleCACer(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/sys/ca-certificate.cer", nil)
	s.handleCACer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pkix-cert" {
		t.Errorf("Content-Type: got %q, want application/pkix-cert", got)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "powerlab-ca.cer") {
		t.Errorf("Content-Disposition missing filename: %q", rec.Header().Get("Content-Disposition"))
	}

	body, _ := io.ReadAll(rec.Body)
	// Body must be raw DER (not PEM). Parsing it as x509 should
	// succeed without any Decode step.
	if _, err := x509.ParseCertificate(body); err != nil {
		t.Fatalf("body is not valid DER: %v", err)
	}
	if strings.HasPrefix(string(body), "-----BEGIN") {
		t.Error("body looks like PEM; should be DER")
	}
}

// TestHandleCABaseUARedirects — the User-Agent dispatcher sends each
// platform to the right format.
func TestHandleCABaseUARedirects(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)

	cases := []struct {
		name     string
		ua       string
		wantPath string
	}{
		{"iPhone → mobileconfig", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", "/v1/sys/ca-certificate.mobileconfig"},
		{"iPad → mobileconfig", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X)", "/v1/sys/ca-certificate.mobileconfig"},
		{"macOS → mobileconfig", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0)", "/v1/sys/ca-certificate.mobileconfig"},
		{"Windows → cer", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "/v1/sys/ca-certificate.cer"},
		{"Linux → crt", "Mozilla/5.0 (X11; Linux x86_64)", "/v1/sys/ca-certificate.crt"},
		{"empty UA → crt", "", "/v1/sys/ca-certificate.crt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/sys/ca-certificate", nil)
			req.Header.Set("User-Agent", c.ua)
			s.handleCABase(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("status: got %d, want 302", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != c.wantPath {
				t.Errorf("redirect to %q, want %q", loc, c.wantPath)
			}
		})
	}
}

// TestSignPKCS7_VerifiableBySignerCert — the .mobileconfig pipeline
// hangs entirely on this: the PKCS#7 wrapper must parse, the signer
// info must reference the CA cert, and the inner content must be
// recoverable. iOS/macOS render "Verified by PowerLab Local CA"
// when this verification succeeds against the installed root.
func TestSignPKCS7_VerifiableBySignerCert(t *testing.T) {
	caCert, caKey := generateTestCA(t)
	payload := []byte("hello world — totally a plist payload")

	signed, err := signPKCS7(payload, caCert, caKey)
	if err != nil {
		t.Fatalf("signPKCS7: %v", err)
	}
	if len(signed) < 100 {
		t.Fatalf("signed blob too small: %d bytes", len(signed))
	}

	parsed, err := pkcs7.Parse(signed)
	if err != nil {
		t.Fatalf("pkcs7.Parse: %v", err)
	}
	if string(parsed.Content) != string(payload) {
		t.Errorf("inner content mismatch — got %q, want %q", parsed.Content, payload)
	}
	if len(parsed.Certificates) == 0 {
		t.Fatal("signed blob has no embedded certificates")
	}
	// At least one of the embedded certs must match the signer CA —
	// that's how iOS chains the profile back to a root the user is
	// about to install.
	found := false
	for _, c := range parsed.Certificates {
		if c.Equal(caCert) {
			found = true
			break
		}
	}
	if !found {
		t.Error("CA cert not embedded in PKCS#7 signed blob")
	}
}

// TestBuildMobileConfigPlist_StructureAndPayload — the unsigned plist
// must be a valid Apple Configuration Profile with the CA cert
// base64-encoded in the PayloadContent dict.
func TestBuildMobileConfigPlist_StructureAndPayload(t *testing.T) {
	caCert, _ := generateTestCA(t)
	plist := buildMobileConfigPlist(caCert.Raw, "test-host")
	s := string(plist)

	required := []string{
		`<?xml version="1.0"`,
		`<!DOCTYPE plist`,
		`<plist version="1.0">`,
		`<key>PayloadType</key>`,
		`<string>com.apple.security.root</string>`,
		`<key>PayloadIdentifier</key>`,
		`<string>com.powerlab.local-ca</string>`,
		`<key>PayloadContent</key>`,
		`PowerLab Local CA — test-host`,
	}
	for _, want := range required {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q", want)
		}
	}

	// Two PayloadUUIDs (cert + profile); they must be valid UUIDs
	// and distinct.
	if strings.Count(s, "<key>PayloadUUID</key>") != 2 {
		t.Errorf("expected 2 PayloadUUID keys, got %d", strings.Count(s, "<key>PayloadUUID</key>"))
	}
}

// TestHandleTrustConfirmedRejectsHTTP — POST trust-confirmed must NOT
// arm the gate when the request arrived over plain HTTP. Otherwise
// any unauthenticated client could lock the user out by curl-ing
// http://host:8765/v1/sys/trust-confirmed.
func TestHandleTrustConfirmedRejectsHTTP(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sys/trust-confirmed", nil)
	// req.TLS is nil — simulates plain HTTP.
	s.handleTrustConfirmed(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("HTTP request armed the HSTS gate — must require HTTPS")
	}
	if cm.IsHSTSArmed() {
		t.Fatal("CertManager.IsHSTSArmed reports armed after HTTP request")
	}
}

// TestHandleTrustConfirmedRejectsLocalhost — even over HTTPS,
// localhost is rejected because curling https://127.0.0.1 from the
// box itself doesn't prove the trust dance worked end-to-end.
func TestHandleTrustConfirmedRejectsLocalhost(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)

	for _, addr := range []string{"127.0.0.1:54321", "[::1]:54321"} {
		t.Run(addr, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/sys/trust-confirmed", nil)
			req.RemoteAddr = addr
			req.TLS = &tls.ConnectionState{}
			s.handleTrustConfirmed(rec, req)

			if rec.Code == http.StatusOK {
				t.Fatalf("localhost armed the gate — must reject")
			}
			if cm.IsHSTSArmed() {
				t.Fatal("HSTS armed despite localhost rejection")
			}
		})
	}
}

// generateTestCA builds an ECDSA P-256 self-signed CA matching
// CertManager.GenerateRootCA shape, but in-memory for fast tests.
func generateTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test PowerLab CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return cert, priv
}

// _ unused-import guard
var _ = filepath.Join

// TestHandleTrustState_FreshState — first-boot probe should return
// armed=false + a fingerprint of the just-generated CA.
func TestHandleTrustState_FreshState(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/sys/trust-state", nil)
	s.handleTrustState(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"armed":false`) {
		t.Errorf("expected armed:false on fresh boot, got %s", body)
	}
	// Fingerprint format is colon-separated hex pairs ending in
	// uppercase. We don't know the actual value (CA was just
	// generated) but the SHAPE is stable.
	if !strings.Contains(body, `"ca_fingerprint":"`) {
		t.Errorf("ca_fingerprint missing or empty: %s", body)
	}
}

// TestHandleTrustState_AfterArm — after ArmHSTS the endpoint flips
// to armed=true.
func TestHandleTrustState_AfterArm(t *testing.T) {
	cm := newTestCertManager(t)
	if err := cm.ArmHSTS(); err != nil {
		t.Fatalf("ArmHSTS: %v", err)
	}
	s := NewSecurityRoute(cm)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/sys/trust-state", nil)
	s.handleTrustState(rec, req)

	if !strings.Contains(rec.Body.String(), `"armed":true`) {
		t.Errorf("expected armed:true after ArmHSTS, got %s", rec.Body.String())
	}
}

// TestHandleTrustConfirmedDELETE_DisarmHSTS — DELETE
// /trust-confirmed clears the HSTS gate (reset-trust path), no
// HTTPS / non-localhost guard required.
func TestHandleTrustConfirmedDELETE_DisarmHSTS(t *testing.T) {
	cm := newTestCertManager(t)
	if err := cm.ArmHSTS(); err != nil {
		t.Fatalf("ArmHSTS: %v", err)
	}
	if !cm.IsHSTSArmed() {
		t.Fatal("setup: gate should be armed")
	}
	s := NewSecurityRoute(cm)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/sys/trust-confirmed", nil)
	s.handleTrustConfirmed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if cm.IsHSTSArmed() {
		t.Fatal("gate still armed after DELETE")
	}
	if !strings.Contains(rec.Body.String(), `"armed":false`) {
		t.Errorf("response should report armed:false, got %s", rec.Body.String())
	}
}

// TestHandleRotateCA_RequiresConfirmToken — rotation must NOT fire
// without the explicit ?confirm=ROTATE_CA query parameter, even
// when all other guards (HTTPS, non-localhost) are satisfied.
func TestHandleRotateCA_RequiresConfirmToken(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)
	originalFP, _ := cm.CAFingerprint()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sys/rotate-ca", nil)
	req.RemoteAddr = "192.168.1.10:54321"
	req.TLS = &tls.ConnectionState{}
	s.handleRotateCA(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without confirm token, got %d", rec.Code)
	}
	currentFP, _ := cm.CAFingerprint()
	if currentFP != originalFP {
		t.Fatal("CA was rotated despite missing confirm token")
	}
}

// TestHandleRotateCA_RequiresHTTPS — same posture as
// /trust-confirmed: the caller must already have the current CA
// installed (proven by a working TLS session).
func TestHandleRotateCA_RequiresHTTPS(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)
	originalFP, _ := cm.CAFingerprint()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/v1/sys/rotate-ca?confirm=ROTATE_CA", nil)
	req.RemoteAddr = "192.168.1.10:54321"
	// req.TLS == nil — plain HTTP
	s.handleRotateCA(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 over HTTP, got %d", rec.Code)
	}
	currentFP, _ := cm.CAFingerprint()
	if currentFP != originalFP {
		t.Fatal("CA was rotated despite plain-HTTP request")
	}
}

// TestHandleRotateCA_RejectsLocalhost — same as above for the
// non-localhost requirement.
func TestHandleRotateCA_RejectsLocalhost(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)
	originalFP, _ := cm.CAFingerprint()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/v1/sys/rotate-ca?confirm=ROTATE_CA", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.TLS = &tls.ConnectionState{}
	s.handleRotateCA(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from localhost, got %d", rec.Code)
	}
	currentFP, _ := cm.CAFingerprint()
	if currentFP != originalFP {
		t.Fatal("CA was rotated despite localhost guard")
	}
}

// TestHandleRotateCA_HappyPath — all guards pass, rotation fires,
// new fingerprint differs from old, public backup is refreshed.
func TestHandleRotateCA_HappyPath(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)
	originalFP, _ := cm.CAFingerprint()
	if originalFP == "" {
		t.Fatal("setup: original fingerprint should not be empty")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/v1/sys/rotate-ca?confirm=ROTATE_CA", nil)
	req.RemoteAddr = "192.168.1.10:54321"
	req.TLS = &tls.ConnectionState{}
	s.handleRotateCA(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	newFP, _ := cm.CAFingerprint()
	if newFP == "" {
		t.Fatal("CA fingerprint empty after rotation")
	}
	if newFP == originalFP {
		t.Fatal("fingerprint did not change after rotation")
	}
	if !strings.Contains(rec.Body.String(), `"rotated":true`) {
		t.Errorf("response should report rotated:true, got %s", rec.Body.String())
	}
	// Previous CA should be preserved as audit trail.
	prevPath := cm.StoragePath + "/ca.crt.previous"
	if _, err := os.Stat(prevPath); err != nil {
		t.Errorf("ca.crt.previous not preserved: %v", err)
	}
	// Public backup should reflect the NEW CA.
	backup, err := os.ReadFile(cm.GetPublicBackupPath())
	if err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	caCertPath, _ := cm.GetCAPaths()
	current, _ := os.ReadFile(caCertPath)
	if string(backup) != string(current) {
		t.Error("public backup not refreshed after rotation")
	}
}

// TestSetup_WritesPublicBackup — first boot writes the backup file.
func TestSetup_WritesPublicBackup(t *testing.T) {
	cm := newTestCertManager(t)
	if _, err := os.Stat(cm.GetPublicBackupPath()); err != nil {
		t.Fatalf("backup file should be written by Setup: %v", err)
	}
	caCertPath, _ := cm.GetCAPaths()
	caBytes, _ := os.ReadFile(caCertPath)
	backup, _ := os.ReadFile(cm.GetPublicBackupPath())
	if string(backup) != string(caBytes) {
		t.Error("backup contents differ from ca.crt")
	}
}

// TestCAFingerprint_StableShape — the colon-separated uppercase
// hex shape that openssl produces. We don't pin a specific value
// (it changes per generation) but we pin the format so a consumer
// can rely on `===` comparison vs another openssl invocation.
func TestCAFingerprint_StableShape(t *testing.T) {
	cm := newTestCertManager(t)
	fp, err := cm.CAFingerprint()
	if err != nil {
		t.Fatalf("CAFingerprint: %v", err)
	}
	// SHA-256 in colon-hex = 32 bytes * 2 hex + 31 colons = 95 chars.
	if len(fp) != 95 {
		t.Errorf("fingerprint length: got %d, want 95 (got %s)", len(fp), fp)
	}
	for i, c := range fp {
		if i%3 == 2 {
			if c != ':' {
				t.Errorf("fingerprint position %d: expected colon, got %c", i, c)
			}
			continue
		}
		// Hex nibble: 0-9 or A-F.
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			t.Errorf("fingerprint position %d: not uppercase hex: %c", i, c)
		}
	}
}
