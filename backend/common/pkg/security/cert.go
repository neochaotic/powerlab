package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/devmode"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"go.uber.org/zap"
)

const (
	DefaultProductionPath = "/etc/powerlab/tls"
	HSTSGateFile         = ".hsts-armed"
)

type CertManager struct {
	StoragePath string
	mu          sync.Mutex
	lastIPs     []string
}

func NewCertManager(runtimePath string) *CertManager {
	storagePath := DefaultProductionPath
	if devmode.IsDev() {
		storagePath = filepath.Join(runtimePath, "tls")
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

func (m *CertManager) Setup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.StoragePath, 0755); err != nil {
		return err
	}

	caCertPath, _ := m.GetCAPaths()
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		if err := m.GenerateRootCA(); err != nil {
			return err
		}
	}

	// Initial IP capture
	m.lastIPs = m.getCurrentIPs()

	// Initial check/generation
	return m.CheckAndRotate()
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
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"powerlab.local", "localhost", hostname + ".local"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
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
			if ok {
				ip := ipnet.IP
				// IPv4 RFC1918
				if v4 := ip.To4(); v4 != nil {
					if v4[0] == 10 || (v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31) || (v4[0] == 192 && v4[1] == 168) {
						ips = append(ips, ip.String())
					}
				} else {
					// IPv6 ULA fc00::/7
					if len(ip) == 16 && (ip[0]&0xfe) == 0xfc {
						ips = append(ips, ip.String())
					}
				}
			}
		}
	}
	return ips
}

func (m *CertManager) compareIPs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m_map := make(map[string]bool)
	for _, ip := range a {
		m_map[ip] = true
	}
	for _, ip := range b {
		if !m_map[ip] {
			return false
		}
	}
	return true
}

func (m *CertManager) ArmHSTS() error {
	return os.WriteFile(m.GetHSTSPath(), []byte("armed"), 0644)
}

func (m *CertManager) IsHSTSArmed() bool {
	_, err := os.Stat(m.GetHSTSPath())
	return err == nil
}
