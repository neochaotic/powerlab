package service

import (
	"net"
	"sync"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/grandcat/zeroconf"
	"go.uber.org/zap"
)

// MDNSService publishes the gateway over Bonjour/mDNS so users can reach
// PowerLab at `powerlab.local` (or whatever hostname is configured) without
// running a DNS server. macOS, iOS, and modern Linux/Windows resolve `.local`
// out of the box; this exists so users don't need to remember an IP address.
//
// Service type: _http._tcp.local. (standard for web UIs; shows up in Safari
// Bonjour bookmarks and similar discovery tools).
type MDNSService struct {
	hostname string
	server   *zeroconf.Server
	mu       sync.Mutex
}

func NewMDNSService(hostname string) *MDNSService {
	if hostname == "" {
		hostname = "powerlab"
	}
	return &MDNSService{hostname: hostname}
}

// Announce registers the service on the local network. Call once after the
// gateway port is finalized. Safe to call multiple times — re-registers.
func (m *MDNSService) Announce(port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
	}

	// We use RegisterProxy (not Register) so we can publish A records under
	// our own hostname (`powerlab.local`) regardless of the OS hostname.
	// Otherwise mDNS would resolve to whatever the host is called (e.g.
	// `neochaotic.local` on a Mac).
	ips := localIPs()
	if len(ips) == 0 {
		// Fallback to loopback so registration doesn't fail in headless tests
		ips = []string{"127.0.0.1"}
	}

	server, err := zeroconf.RegisterProxy(
		"PowerLab",      // service instance name (shown in Bonjour browsers)
		"_http._tcp",    // service type
		"local.",        // domain
		port,            // port
		m.hostname,      // hostname → results in `<hostname>.local`
		ips,             // host IP addresses to publish in the A records
		[]string{"path=/", "powerlab=true"},
		nil,             // interfaces — nil = all
	)
	if err != nil {
		return err
	}

	m.server = server
	logger.Info("mDNS service announced",
		zap.String("hostname", m.hostname+".local"),
		zap.Int("port", port),
	)
	return nil
}

// Shutdown unregisters the service. Call on graceful exit so other devices
// stop seeing the entry immediately.
func (m *MDNSService) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
		logger.Info("mDNS service withdrawn")
	}
}

// localIPs returns all non-loopback IPv4 + IPv6 addresses bound to up
// interfaces. We publish all of them so the host is reachable on whichever
// interface (wifi, ethernet, etc.) the client is using.
func localIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			if ip.IsLinkLocalUnicast() || ip.IsLoopback() {
				continue
			}
			out = append(out, ip.String())
		}
	}
	return out
}
