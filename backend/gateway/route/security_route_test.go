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
