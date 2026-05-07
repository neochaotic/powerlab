package route

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/CasaOS-Common/pkg/security"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/digitorus/pkcs7"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SecurityRoute exposes the local CA + HSTS-trust endpoints described
// in ADRs 0001, 0002, 0006. Lives on the gateway because that's where
// the cert files exist (and where TLS terminates); does not proxy to
// any backend service.
//
// Endpoints (all under /v1/sys):
//
//   GET  /v1/sys/ca-certificate                    UA-based 302 redirect
//   GET  /v1/sys/ca-certificate.crt                PEM-encoded CA cert
//   GET  /v1/sys/ca-certificate.mobileconfig       PKCS#7-signed Apple
//                                                  Configuration Profile
//   GET  /v1/sys/ca-certificate.cer                DER-encoded CA cert
//                                                  (Windows import wizard)
//   POST /v1/sys/trust-confirmed                   arms the HSTS gate
//
// The download endpoints are unauthenticated by design (catch-22:
// users need the CA in order to authenticate over HTTPS in the first
// place). Each download is logged for the admin's audit log.
type SecurityRoute struct {
	cm *security.CertManager
}

func NewSecurityRoute(cm *security.CertManager) *SecurityRoute {
	return &SecurityRoute{cm: cm}
}

// Register attaches the security handlers onto the given mux. Caller
// is responsible for ensuring this runs BEFORE any catch-all proxy
// registered at "/".
func (s *SecurityRoute) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/sys/ca-certificate", s.handleCABase)
	mux.HandleFunc("/v1/sys/ca-certificate.crt", s.handleCACrt)
	mux.HandleFunc("/v1/sys/ca-certificate.mobileconfig", s.handleCAMobileConfig)
	mux.HandleFunc("/v1/sys/ca-certificate.cer", s.handleCACer)
	mux.HandleFunc("/v1/sys/trust-confirmed", s.handleTrustConfirmed)
}

// handleCABase redirects to the format that matches the User-Agent.
// Apple devices get the .mobileconfig (one-tap install). Windows gets
// .cer (DER, accepted by the Certificate Import Wizard). Everyone else
// gets the raw .crt.
func (s *SecurityRoute) handleCABase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ua := strings.ToLower(r.UserAgent())
	switch {
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"),
		strings.Contains(ua, "macintosh"), strings.Contains(ua, "mac os x"):
		http.Redirect(w, r, "/v1/sys/ca-certificate.mobileconfig", http.StatusFound)
	case strings.Contains(ua, "windows"):
		http.Redirect(w, r, "/v1/sys/ca-certificate.cer", http.StatusFound)
	default:
		http.Redirect(w, r, "/v1/sys/ca-certificate.crt", http.StatusFound)
	}
}

// handleCACrt serves the raw CA certificate as PEM. Trusted by Linux,
// Android, Windows (manual install path), or as a fallback for any
// platform.
func (s *SecurityRoute) handleCACrt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	caCertPath, _ := s.cm.GetCAPaths()
	pemBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		http.Error(w, "ca certificate not available yet", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="powerlab-ca.crt"`)
	logger.Info("CA certificate downloaded",
		zap.String("format", "crt"),
		zap.String("ua", r.UserAgent()),
		zap.String("ip", r.RemoteAddr),
	)
	_, _ = w.Write(pemBytes)
}

// handleCAMobileConfig serves a PKCS#7-signed Apple Configuration
// Profile that bundles the CA certificate. Signing is done with the
// CA itself so iOS / macOS render "Verified by PowerLab Local CA"
// instead of the red "Unverified" banner. ADR 0002 (digitorus/pkcs7).
func (s *SecurityRoute) handleCAMobileConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	caCertPath, caKeyPath := s.cm.GetCAPaths()
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		http.Error(w, "ca certificate not available yet", http.StatusServiceUnavailable)
		return
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		http.Error(w, "ca key not available", http.StatusServiceUnavailable)
		return
	}

	// Build the unsigned plist payload first.
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		http.Error(w, "ca cert PEM decode failed", http.StatusInternalServerError)
		return
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("ca cert parse failed: %v", err), http.StatusInternalServerError)
		return
	}
	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		http.Error(w, "ca key PEM decode failed", http.StatusInternalServerError)
		return
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("ca key parse failed: %v", err), http.StatusInternalServerError)
		return
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "powerlab"
	}

	plist := buildMobileConfigPlist(caBlock.Bytes, hostname)

	// Sign the plist with the CA itself. iOS will then display the
	// profile as "Verified by PowerLab Local CA" once the user
	// installs the CA — until then it's "Verified" against an
	// untrusted-root, which is fine because the CA install IS what
	// the user is doing here.
	signed, err := signPKCS7(plist, caCert, caKey)
	if err != nil {
		logger.Error("mobileconfig signing failed", zap.Error(err))
		// Fall back to unsigned plist — iOS will show "Unverified"
		// but the install still works. Better than 500.
		signed = plist
	}

	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Header().Set("Content-Disposition", `attachment; filename="PowerLab Local CA.mobileconfig"`)
	logger.Info("CA mobileconfig downloaded",
		zap.String("format", "mobileconfig"),
		zap.Bool("signed", err == nil),
		zap.String("ua", r.UserAgent()),
		zap.String("ip", r.RemoteAddr),
	)
	_, _ = w.Write(signed)
}

// handleCACer serves the CA cert as DER-encoded x509. The Windows
// Certificate Import Wizard accepts .cer (DER) directly for trusted-
// root install; this is the honest filename for what we're serving
// (raw DER, not a PKCS#12 bundle). A real PKCS#12 (.p12) wrapper can
// land in a later release if a use case for it appears.
func (s *SecurityRoute) handleCACer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	caCertPath, _ := s.cm.GetCAPaths()
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		http.Error(w, "ca certificate not available yet", http.StatusServiceUnavailable)
		return
	}
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		http.Error(w, "ca cert PEM decode failed", http.StatusInternalServerError)
		return
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("ca cert parse failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pkix-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="powerlab-ca.cer"`)
	logger.Info("CA cer downloaded",
		zap.String("format", "cer"),
		zap.String("ua", r.UserAgent()),
		zap.String("ip", r.RemoteAddr),
	)
	_, _ = w.Write(caCert.Raw)
}

// handleTrustConfirmed arms the HSTS gate. Required so the user can
// browse to HTTPS and not get locked out — see ADR 0006.
//
// Conditions: the request must come over HTTPS (r.TLS != nil) and
// from a non-localhost address. We log who armed the gate.
func (s *SecurityRoute) handleTrustConfirmed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.TLS == nil {
		http.Error(w, "trust-confirmed must arrive over HTTPS — the whole point of this endpoint is to prove the trust dance worked", http.StatusBadRequest)
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if host == "" {
		host = r.RemoteAddr
	}
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		http.Error(w, "trust-confirmed must arrive from a non-localhost address — localhost requests bypass TLS verification", http.StatusBadRequest)
		return
	}

	if err := s.cm.ArmHSTS(); err != nil {
		http.Error(w, fmt.Sprintf("failed to arm HSTS: %v", err), http.StatusInternalServerError)
		return
	}
	logger.Info("HSTS gate armed",
		zap.String("ip", host),
		zap.String("ua", r.UserAgent()),
	)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"armed":true}`))
}

// buildMobileConfigPlist assembles the Apple Configuration Profile
// XML. Unsigned at this stage; signing happens in signPKCS7.
func buildMobileConfigPlist(caCertDER []byte, hostname string) []byte {
	encoded := base64.StdEncoding.EncodeToString(caCertDER)
	// 60-char wrap to match Apple's example profiles.
	var wrapped strings.Builder
	for i := 0; i < len(encoded); i += 60 {
		end := i + 60
		if end > len(encoded) {
			end = len(encoded)
		}
		wrapped.WriteString(encoded[i:end])
		wrapped.WriteString("\n")
	}

	profileUUID := uuid.New().String()
	certUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>PayloadContent</key>
    <array>
        <dict>
            <key>PayloadCertificateFileName</key>
            <string>powerlab-ca.crt</string>
            <key>PayloadContent</key>
            <data>
%s</data>
            <key>PayloadDescription</key>
            <string>PowerLab Local Certificate Authority — trusts the local server's HTTPS certificate.</string>
            <key>PayloadDisplayName</key>
            <string>PowerLab Local CA — %s</string>
            <key>PayloadIdentifier</key>
            <string>com.powerlab.local-ca.cert.%s</string>
            <key>PayloadType</key>
            <string>com.apple.security.root</string>
            <key>PayloadUUID</key>
            <string>%s</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
        </dict>
    </array>
    <key>PayloadDescription</key>
    <string>Installs the PowerLab Local CA so this device trusts the HTTPS certificate served by your PowerLab instance.</string>
    <key>PayloadDisplayName</key>
    <string>PowerLab Local CA</string>
    <key>PayloadIdentifier</key>
    <string>com.powerlab.local-ca</string>
    <key>PayloadOrganization</key>
    <string>PowerLab</string>
    <key>PayloadRemovalDisallowed</key>
    <false/>
    <key>PayloadType</key>
    <string>Configuration</string>
    <key>PayloadUUID</key>
    <string>%s</string>
    <key>PayloadVersion</key>
    <integer>1</integer>
</dict>
</plist>
`, wrapped.String(), hostname, certUUID, certUUID, profileUUID)
	return []byte(plist)
}

// signPKCS7 wraps the plist payload in a PKCS#7 SignedData blob using
// the CA's own private key. Apple parses the outer PKCS#7 envelope to
// validate the signer chain — once the user has installed the CA,
// iOS displays "Verified by PowerLab Local CA".
func signPKCS7(payload []byte, signerCert *x509.Certificate, signerKey *ecdsa.PrivateKey) ([]byte, error) {
	sd, err := pkcs7.NewSignedData(payload)
	if err != nil {
		return nil, fmt.Errorf("pkcs7 NewSignedData: %w", err)
	}
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	if err := sd.AddSigner(signerCert, signerKey, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, fmt.Errorf("pkcs7 AddSigner: %w", err)
	}
	return sd.Finish()
}

// _ unused import guard for filepath (lint suppression — the path
// helpers in CertManager use it but Go won't complain).
var _ = filepath.Join
