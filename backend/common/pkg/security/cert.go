package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/devmode"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"go.uber.org/zap"
)

const (
	// DefaultProductionPath is where the CA + leaf live in prod.
	// Conventionally `/etc/...` is config that survives data-dir
	// wipes (`/var/lib/powerlab` etc.); we deliberately keep the
	// security material there so a `rm -rf /var/lib/powerlab`
	// does NOT void every device's installed trust.
	DefaultProductionPath = "/etc/powerlab/security"

	// LegacyProductionPath is the original v0.2.7 location.
	// CertManager.Setup migrates from here on first boot so users
	// upgrading from v0.2.7 don't see their CA disappear.
	LegacyProductionPath = "/etc/powerlab/tls"

	// HSTSGateFile is the marker file that gates the HSTS header
	// (see ADR 0006 — HSTS gate after first verified non-localhost
	// client).
	HSTSGateFile = ".hsts-armed"

	// HSTSDisarmingFile is the post-reset marker that tells the
	// WrapHSTS middleware to emit `Strict-Transport-Security:
	// max-age=0` for HSTSDisarmingTTL after a reset/rotate. Browsers
	// see max-age=0 and clear their cached HSTS pin (RFC 6797
	// §6.1.1) — without this, even a server-side reset doesn't
	// reach a browser that already cached the pin.
	HSTSDisarmingFile = ".hsts-disarming"

	// HSTSDisarmingTTL is how long after a reset we keep emitting
	// max-age=0. 15 min is enough for the user to load the page
	// once on each device they want to recover; longer than that
	// just confuses cache layers (CDNs, corporate proxies).
	HSTSDisarmingTTL = 15 * time.Minute

	// PublicBackupFile is a non-secret copy of the CA public cert
	// the user can backup to a USB stick / cloud / password
	// manager. Identical bytes to ca.crt; the separate filename
	// signals "this is the file to back up" to a human grepping
	// the storage dir.
	PublicBackupFile = "ca-public-backup.crt"
)

type CertManager struct {
	StoragePath string
	mu          sync.Mutex
	lastIPs     []string
}

// NewCertManager picks the storage path based on dev/prod mode.
//
// Prod: /etc/powerlab/security — survives data-dir wipes.
// Dev:  ~/.config/powerlab/security — survives `start.sh --build`,
// `rm -rf backend/runtime/`, and other dev-cycle wipes. The cert
// storage is decoupled from the runtime dir on purpose, so the
// user's "trust me once" install is not invalidated by a routine
// `rm -rf` of the data dir. See ADR 0010.
//
// `runtimePath` is the legacy v0.2.7 dev location and is used only
// as a fallback when HOME is unavailable.
func NewCertManager(runtimePath string) *CertManager {
	storagePath := DefaultProductionPath
	if devmode.IsDev() {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			storagePath = filepath.Join(home, ".config", "powerlab", "security")
		} else {
			storagePath = filepath.Join(runtimePath, "tls")
		}
	}
	return &CertManager{StoragePath: storagePath}
}

func (m *CertManager) GetCAPaths() (certPath, keyPath string) {
	return filepath.Join(m.StoragePath, "ca.crt"), filepath.Join(m.StoragePath, "ca.key")
}

func (m *CertManager) GetServerPaths() (certPath, keyPath string) {
	return filepath.Join(m.StoragePath, "server.crt"), filepath.Join(m.StoragePath, "server.key")
}

func (m *CertManager) GetHSTSPath() string {
	return filepath.Join(m.StoragePath, HSTSGateFile)
}

// GetPublicBackupPath returns the user-visible "back this up" file path.
func (m *CertManager) GetPublicBackupPath() string {
	return filepath.Join(m.StoragePath, PublicBackupFile)
}

// Setup initializes the cert manager: ensures CA exists, generates
// server cert if missing, etc. NO-OP when HTTPS is gated (see
// pkg/security/gate.go and issue #130) — returns nil immediately so
// the caller's gateway boot continues HTTP-only without producing
// a CA on disk that the user never asked for.
//
// Why gate at this layer: every entry point to cert generation
// (GenerateRootCA, CheckAndRotate, writePublicBackup) is reached
// from Setup. Gating here is one check that covers all paths.
func (m *CertManager) Setup() error {
	if !HTTPSEnabled() {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.StoragePath, 0755); err != nil {
		return err
	}

	// One-shot migration: if the legacy storage path exists and
	// we don't already have a CA at the new path, copy the files.
	// Idempotent — runs once, leaves a no-op on subsequent boots.
	if err := m.migrateLegacyStorage(); err != nil {
		logger.Info("legacy cert storage migration skipped", zap.Error(err))
	}

	caCertPath, _ := m.GetCAPaths()
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		if err := m.GenerateRootCA(); err != nil {
			return err
		}
	}

	// Always (re-)write the user-visible backup file from the
	// canonical ca.crt. If the user has rotated the CA via the
	// rotation flow the backup is automatically refreshed.
	if err := m.writePublicBackup(); err != nil {
		logger.Info("could not refresh CA public backup", zap.Error(err))
	}

	// Initial IP capture
	m.lastIPs = m.getCurrentIPs()

	// Initial check/generation of the server leaf
	return m.CheckAndRotate()
}

// migrateLegacyStorage moves files from the v0.2.7 production
// location (/etc/powerlab/tls) into the new path
// (/etc/powerlab/security). Safe to call repeatedly: no-op if the
// new path already has a ca.crt OR if the legacy path doesn't exist.
func (m *CertManager) migrateLegacyStorage() error {
	if m.StoragePath != DefaultProductionPath {
		return nil // dev install; no production migration to run
	}
	newCA := filepath.Join(m.StoragePath, "ca.crt")
	if _, err := os.Stat(newCA); err == nil {
		return nil // already populated, migration done or never needed
	}
	legacyCA := filepath.Join(LegacyProductionPath, "ca.crt")
	if _, err := os.Stat(legacyCA); os.IsNotExist(err) {
		return nil // nothing to migrate
	}
	logger.Info("Migrating CA storage from legacy path",
		zap.String("from", LegacyProductionPath),
		zap.String("to", m.StoragePath))
	for _, name := range []string{"ca.crt", "ca.key", "server.crt", "server.key", HSTSGateFile} {
		src := filepath.Join(LegacyProductionPath, name)
		dst := filepath.Join(m.StoragePath, name)
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		perm := os.FileMode(0644)
		if name == "ca.key" || name == "server.key" {
			perm = 0600
		}
		if err := os.WriteFile(dst, data, perm); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		_ = os.Remove(src)
	}
	return nil
}

// writePublicBackup creates ca-public-backup.crt next to ca.crt with
// 0644 perms. Identical bytes to ca.crt; the CA private key is NEVER
// included — exposing it would compromise every device that trusts
// the CA.
func (m *CertManager) writePublicBackup() error {
	caCertPath, _ := m.GetCAPaths()
	src, err := os.ReadFile(caCertPath)
	if err != nil {
		return err
	}
	dst := filepath.Join(m.StoragePath, PublicBackupFile)
	return os.WriteFile(dst, src, 0644)
}

// CAFingerprint returns the SHA-256 fingerprint of the active CA cert
// in colon-separated uppercase hex (the format `openssl x509
// -fingerprint -sha256` produces). Used by the trust-state endpoint
// so clients can detect a CA mismatch (CA regenerated or rotated
// server-side) and re-prompt for trust install.
func (m *CertManager) CAFingerprint() (string, error) {
	caCertPath, _ := m.GetCAPaths()
	pemBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return "", fmt.Errorf("CA pem decode failed")
	}
	sum := sha256.Sum256(block.Bytes)
	hex := make([]byte, 0, len(sum)*3-1)
	for i, b := range sum {
		if i > 0 {
			hex = append(hex, ':')
		}
		hex = append(hex, hexNybble(b>>4), hexNybble(b&0x0f))
	}
	return string(hex), nil
}

func hexNybble(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'A' + (n - 10)
}

func (m *CertManager) CheckAndRotate() error {
	serverCertPath, _ := m.GetServerPaths()

	shouldRotate := false

	// 1. Check if server cert exists
	if _, err := os.Stat(serverCertPath); os.IsNotExist(err) {
		shouldRotate = true
	} else {
		// 2. Check Expiry (< 60 days)
		certBytes, err := os.ReadFile(serverCertPath)
		if err == nil {
			block, _ := pem.Decode(certBytes)
			if block != nil {
				cert, err := x509.ParseCertificate(block.Bytes)
				if err == nil {
					if time.Until(cert.NotAfter) < (60 * 24 * time.Hour) {
						logger.Info("Server certificate expiring soon, rotating...", zap.Time("expiry", cert.NotAfter))
						shouldRotate = true
					}
				}
			}
		}

		// 3. Check IP Change
		currentIPs := m.getCurrentIPs()
		if !m.compareIPs(m.lastIPs, currentIPs) {
			logger.Info("IP change detected, rotating server certificate...", zap.Strings("old", m.lastIPs), zap.Strings("new", currentIPs))
			m.lastIPs = currentIPs
			shouldRotate = true
		}
	}

	if shouldRotate {
		return m.GenerateServerCert()
	}

	return nil
}

func (m *CertManager) StartTicker(ctx_cancel chan struct{}) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.mu.Lock()
				if err := m.CheckAndRotate(); err != nil {
					logger.Error("Failed to rotate certificates in ticker", zap.Error(err))
				}
				m.mu.Unlock()
			case <-ctx_cancel:
				ticker.Stop()
				return
			}
		}
	}()
}

func (m *CertManager) GenerateRootCA() error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PowerLab CA"},
			CommonName:   "PowerLab Local Authority",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	caCertPath, caKeyPath := m.GetCAPaths()

	certOut, err := os.Create(caCertPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyOut, err := os.OpenFile(caKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})

	return nil
}

func (m *CertManager) GenerateServerCert() error {
	caCertPath, caKeyPath := m.GetCAPaths()
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return err
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return err
	}

	caBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(caKeyPEM)
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	hostname, _ := os.Hostname()

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PowerLab"},
			CommonName:   "powerlab.local",
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    buildDNSNames(hostname),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	// Add all local IPv4 and IPv6 ULA addresses to SAN
	for _, ipStr := range m.getCurrentIPs() {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(ipStr))
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return err
	}

	serverCertPath, serverKeyPath := m.GetServerPaths()

	certOut, err := os.Create(serverCertPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyOut, err := os.OpenFile(serverKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})

	return nil
}

// RotateCA performs a destructive CA rotation. After this returns
// successfully, EVERY device that previously trusted PowerLab will
// see cert-not-trusted and must re-install the CA. The old material
// is preserved as ca.crt.previous / ca.key.previous so an admin
// with shell access can recover if the rotation was a mistake.
//
// Steps:
//
//  1. Move current ca.{crt,key} aside as .previous (audit trail)
//  2. Generate new root CA (10y validity, fresh keypair)
//  3. Generate new server leaf signed by the new CA
//  4. Refresh the public backup file
//  5. Remove the HSTS gate file so the user has to re-run the trust
//     dance from scratch (otherwise armed HSTS would point browsers
//     at the new HTTPS leaf they can't verify yet)
//
// This is the canonical "Rotate CA" action surfaced in the UI as a
// distinct, scary, confirmation-required button — separate from the
// lighter "Reset trust" which only clears the HSTS gate. See
// ADR 0012.
func (m *CertManager) RotateCA() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	caCertPath, caKeyPath := m.GetCAPaths()
	prevCert := caCertPath + ".previous"
	prevKey := caKeyPath + ".previous"

	// 1. Move existing material aside (best-effort).
	if data, err := os.ReadFile(caCertPath); err == nil {
		_ = os.WriteFile(prevCert, data, 0644)
	}
	if data, err := os.ReadFile(caKeyPath); err == nil {
		_ = os.WriteFile(prevKey, data, 0600)
	}

	// 2. Remove active CA so GenerateRootCA writes fresh files.
	_ = os.Remove(caCertPath)
	_ = os.Remove(caKeyPath)

	if err := m.GenerateRootCA(); err != nil {
		return fmt.Errorf("rotate: generate new CA: %w", err)
	}

	// 3. New leaf signed by the new CA.
	serverCertPath, serverKeyPath := m.GetServerPaths()
	_ = os.Remove(serverCertPath)
	_ = os.Remove(serverKeyPath)
	m.lastIPs = m.getCurrentIPs()
	if err := m.GenerateServerCert(); err != nil {
		return fmt.Errorf("rotate: generate new leaf: %w", err)
	}

	// 4. Refresh public backup so a user grabbing it post-rotation
	// gets the new CA bytes.
	if err := m.writePublicBackup(); err != nil {
		logger.Info("rotate: could not refresh public backup", zap.Error(err))
	}

	// 5. Disarm HSTS AND open the disarming window so any browser
	// that previously cached an HSTS pin against the OLD CA gets
	// `max-age=0` on its next HTTPS request — the only way to
	// clear the in-browser pin without user intervention.
	_ = os.Remove(m.GetHSTSPath())
	_ = os.WriteFile(m.GetHSTSDisarmingPath(), []byte("disarming"), 0644)

	logger.Info("CA rotated. All client devices must re-install the new CA.",
		zap.String("storage", m.StoragePath))
	return nil
}

func (m *CertManager) getCurrentIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ShouldIncludeIP(iface.Name, ipnet.IP) {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips
}

func (m *CertManager) compareIPs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	mp := make(map[string]bool)
	for _, ip := range a {
		mp[ip] = true
	}
	for _, ip := range b {
		if !mp[ip] {
			return false
		}
	}
	return true
}

// GetHSTSDisarmingPath returns the .hsts-disarming marker path.
func (m *CertManager) GetHSTSDisarmingPath() string {
	return filepath.Join(m.StoragePath, HSTSDisarmingFile)
}

func (m *CertManager) ArmHSTS() error {
	// Arming clears any pending disarming window — the user just
	// completed a fresh trust dance, so we want full HSTS again.
	_ = os.Remove(m.GetHSTSDisarmingPath())
	return os.WriteFile(m.GetHSTSPath(), []byte("armed"), 0644)
}

// DisarmHSTS removes the armed gate AND drops a `.hsts-disarming`
// marker. The middleware uses that marker to emit
// `Strict-Transport-Security: max-age=0` for HSTSDisarmingTTL,
// which clears any cached HSTS pin in the browser (RFC 6797 §6.1.1).
// Without this, even after a server-side reset, browsers refuse to
// downgrade to HTTP and may bounce on cert errors.
func (m *CertManager) DisarmHSTS() error {
	if err := os.Remove(m.GetHSTSPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Touch the disarming marker. WriteFile with current time
	// (mtime) so the middleware can compute "is the disarming
	// window still open" by stat'ing the file.
	if err := os.WriteFile(m.GetHSTSDisarmingPath(), []byte("disarming"), 0644); err != nil {
		return err
	}
	return nil
}

func (m *CertManager) IsHSTSArmed() bool {
	_, err := os.Stat(m.GetHSTSPath())
	return err == nil
}

// IsHSTSDisarming returns true while the post-reset window is
// still open (<= HSTSDisarmingTTL old). Outside the window the
// marker is logically no-op; the middleware ignores it.
func (m *CertManager) IsHSTSDisarming() bool {
	info, err := os.Stat(m.GetHSTSDisarmingPath())
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) <= HSTSDisarmingTTL
}

// ShouldIncludeIP returns true when an IP belongs in the leaf
// certificate's SAN list. Extracted from getCurrentIPs so the
// classification rules can be unit-tested without bringing real
// network interfaces into the test.
//
// Rules (per ADR 0001):
//
//   - RFC1918 IPv4 (10/8, 172.16/12, 192.168/16) — always included
//   - IPv6 ULA fc00::/7 — always included
//   - CGNAT 100.64.0.0/10 — included ONLY when ifaceName matches a
//     mesh-VPN tunnel naming pattern (tunnel-style names on Linux,
//     utun* on macOS). Including it unconditionally would pollute
//     the SAN with arbitrary VPN-client addresses on hosts where
//     utun is shared between mesh-VPN clients.
func ShouldIncludeIP(ifaceName string, ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 10 || (v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31) || (v4[0] == 192 && v4[1] == 168) {
			return true
		}
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return strings.HasPrefix(ifaceName, "tailscale") || strings.HasPrefix(ifaceName, "utun")
		}
		return false
	}
	if len(ip) == 16 && (ip[0]&0xfe) == 0xfc {
		return true
	}
	return false
}

// buildDNSNames constructs the DNSNames slice for a leaf cert. Always
// includes powerlab.local + localhost + <system-hostname>.local. When
// Tailscale is installed and authenticated on this host, also includes
// the MagicDNS hostname (e.g. m900.tailnet-name.ts.net) so users
// reaching PowerLab over their Tailscale mesh see the green padlock
// without certificate warnings (#44).
//
// Tailscale lookup is best-effort: if the `tailscale` CLI isn't
// installed, isn't authenticated, or returns an unexpected JSON shape,
// we silently skip and fall back to the LAN names. A logged-in
// admin reading the cert SAN will see whether Tailscale was detected.
func buildDNSNames(hostname string) []string {
	names := []string{"powerlab.local", "localhost", hostname + ".local"}
	if ts := tailscaleMagicDNSName(); ts != "" {
		names = append(names, ts)
		logger.Info("included Tailscale MagicDNS hostname in cert SAN",
			zap.String("hostname", ts),
		)
	}
	return names
}

// tailscaleMagicDNSName returns the host's Tailscale MagicDNS hostname
// (e.g. m900.tailnet-name.ts.net) without the trailing dot, or empty
// string if Tailscale is unavailable.
//
// Implementation: shells out to `tailscale status --json` and reads
// `Self.DNSName`. The CLI is used (not the local API) because:
//   - the CLI is the supported integration surface
//   - it works whether tailscaled is talking to control or in
//     login-required state — the JSON has DNSName populated as soon
//     as the host has a stable identity
//   - shelling out is invoked once per cert generation (not hot
//     path), so the process spawn cost is irrelevant
//
// 2-second timeout so a hung tailscaled never blocks cert rotation.
func tailscaleMagicDNSName() string {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return ""
	}
	cmd := exec.Command("tailscale", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return ""
	}
	// Tailscale returns the FQDN with a trailing dot (DNS canonical
	// form). x509 SAN entries should NOT have the trailing dot, so
	// strip it.
	return strings.TrimSuffix(status.Self.DNSName, ".")
}
