package service

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/grandcat/zeroconf"
	"go.uber.org/zap"
)

// MDNSService publishes the gateway over Bonjour/mDNS so users can reach
// PowerLab at `powerlab.local` (or whatever hostname is configured)
// without running a DNS server. macOS, iOS, and Linux/Windows hosts with
// nss-mdns / Bonjour Print Services resolve `.local` out of the box.
//
// On Linux this gets tricky because most distros ship `avahi-daemon`,
// which already owns the IPv4 multicast socket. If we tried to bind it
// directly, our process would race with avahi for incoming queries —
// usually losing silently. The right pattern is dual-path:
//
//   1. If `/etc/avahi/services/` exists, drop a `powerlab.service` XML
//      file there. avahi watches the directory and re-publishes it on
//      our behalf. This is what AirPrint, Plex, and most well-behaved
//      Linux daemons do.
//   2. ALSO call grandcat/zeroconf. On hosts without avahi this is the
//      only path; on hosts with avahi the bind fails harmlessly (avahi
//      owns the socket), we log it, the avahi service file does the
//      actual work.
//
// Service type: `_http._tcp.local.` — the standard for web UIs; shows
// up in Safari's Bonjour bookmarks and in `dns-sd -B _http._tcp.`.
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

// Announce registers the service on the local network. Call once after
// the gateway port is finalised; safe to call again (re-registers the
// service so the SRV port is up to date if the gateway port changes
// at runtime).
func (m *MDNSService) Announce(port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
	}

	// Path 1: drop an avahi service file (best-effort, Linux only).
	// If the directory exists, avahi is running and will pick it up
	// within a couple of seconds. If anything fails (file system
	// readonly, permission denied, etc.) we log and continue — the
	// grandcat path below is the fallback.
	if err := writeAvahiServiceFile(m.hostname, port); err != nil {
		logger.Info("avahi service file not written (will fall back to direct multicast)",
			zap.Any("error", err),
		)
	}

	// Path 2: direct multicast via grandcat/zeroconf. Publishes the A
	// records for our chosen hostname against every LAN IP we have.
	// On hosts where avahi already owns the multicast socket, this
	// will fail to bind — we log and accept that, because the avahi
	// service file from path 1 is doing the actual announcement.
	ips := lanIPs()
	if len(ips) == 0 {
		// Fallback to loopback so registration does not fail in
		// headless-test environments. Useless for LAN clients but
		// harmless.
		ips = []string{"127.0.0.1"}
	}

	server, err := zeroconf.RegisterProxy(
		"PowerLab",   // service instance name (shown in Bonjour browsers)
		"_http._tcp", // service type
		"local.",     // domain
		port,         // port
		m.hostname,   // hostname → results in `<hostname>.local`
		ips,
		[]string{"path=/", "powerlab=true"},
		nil, // interfaces — nil = all
	)
	if err != nil {
		// Don't return the error — the avahi path above may have
		// succeeded even when the direct one fails. We log so the
		// admin can see what's happening.
		logger.Info("direct mDNS bind failed (likely avahi owns the socket — that is fine)",
			zap.Any("error", err),
			zap.String("hostname", m.hostname+".local"),
			zap.Int("port", port),
		)
		return nil
	}

	m.server = server
	logger.Info("mDNS service announced (direct multicast)",
		zap.String("hostname", m.hostname+".local"),
		zap.Int("port", port),
		zap.Strings("ips", ips),
	)
	return nil
}

// Shutdown unregisters the service. Removes the avahi service file (so
// avahi stops broadcasting) and tears down the direct-multicast
// announcer.
func (m *MDNSService) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	_ = removeAvahiServiceFile()

	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
		logger.Info("mDNS service withdrawn")
	}
}

// lanIPs returns every IPv4 / IPv6 address bound to a non-loopback
// interface that lies in a private LAN range. Crucially it filters out:
//
//   - link-local (169.254/16, fe80::/10) — never useful to advertise
//   - loopback                               (handled by the FlagLoopback bit)
//   - Tailscale's CGNAT range (100.64.0.0/10) — usable on the Tailscale
//     mesh but useless for LAN-only clients; advertising it confuses
//     non-Tailscale macs that try to connect there
//   - public IPv4 / IPv6 GUA — would advertise the host's WAN IP,
//     which is useless for LAN clients and bad for privacy
//   - Docker bridge networks (typically 172.17.0.0/16 by default but
//     not guaranteed) — captured by the "not in RFC 1918" rule below
//     because Docker DOES use 172.16/12 by default. We accept that
//     trade-off rather than scan `docker network ls` here.
//
// We keep RFC 1918 (10/8, 172.16/12, 192.168/16) and IPv6 ULA (fc00::/7).
// The cost of an over-aggressive filter (no IP advertised) is the
// gateway falling back to the avahi path, which works fine on Linux.
func lanIPs() []string {
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
			if !isLANRange(ip) {
				continue
			}
			out = append(out, ip.String())
		}
	}
	return out
}

// isLANRange reports whether the IP is in a private-network range we
// want to advertise via mDNS.
func isLANRange(ip net.IP) bool {
	// IPv4
	if v4 := ip.To4(); v4 != nil {
		// 10.0.0.0/8
		if v4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if v4[0] == 192 && v4[1] == 168 {
			return true
		}
		// Tailscale's CGNAT range 100.64.0.0/10 — explicitly NOT
		// returned, see comment on lanIPs(). Listed here so future
		// readers see the deliberate decision.
		// if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		//     return false
		// }
		return false
	}
	// IPv6 ULA fc00::/7
	if len(ip) == 16 && (ip[0]&0xfe) == 0xfc {
		return true
	}
	return false
}

// avahiServicesDir is where avahi-daemon watches for service files.
// Standard across every distro that ships avahi.
const avahiServicesDir = "/etc/avahi/services"
const avahiServiceFile = "powerlab.service"

// writeAvahiServiceFile drops a per-PowerLab service definition into
// /etc/avahi/services/. avahi-daemon picks it up automatically — no
// reload needed. The XML format is the avahi-service-file(5) standard.
//
// The function is best-effort: it returns errors for diagnostic
// logging, but never panics or blocks the gateway.
//
// We use the literal hostname in the SRV record so the resolved name
// matches `<hostname>.local` rather than whatever the OS calls itself
// (`neochaotic.local` on a stock Mac mini).
func writeAvahiServiceFile(hostname string, port int) error {
	// Only attempt on hosts that actually have an avahi config dir.
	// Cheaper than calling `systemctl is-active avahi-daemon` and
	// gives the same correctness guarantee — if /etc/avahi exists,
	// avahi will read what we write the next time it polls (or
	// immediately, since it watches with inotify).
	if _, err := os.Stat(avahiServicesDir); err != nil {
		if os.IsNotExist(err) {
			return nil // no avahi on this host — silently skip
		}
		return fmt.Errorf("stat %s: %w", avahiServicesDir, err)
	}

	// Crucially we DO NOT emit a <host-name> element here. avahi only
	// publishes hostnames it owns (the system hostname); declaring an
	// arbitrary <host-name>powerlab.local</host-name> made avahi
	// silently reject the service registration. Without the element,
	// avahi publishes against the system's own `<hostname>.local`.
	// Users reach PowerLab at `<hostname>.local:<port>` (where
	// hostname is whatever `hostnamectl --static` returns).
	//
	// To reach the box at the literal `powerlab.local`, the host's
	// own static hostname must be `powerlab` — install.sh prints the
	// `hostnamectl set-hostname powerlab` recommendation when it
	// detects a non-`powerlab` hostname.
	_ = hostname // kept in the signature for callers; not used in XML
	xml := fmt.Sprintf(`<?xml version="1.0" standalone='no'?>
<!DOCTYPE service-group SYSTEM "avahi-service.xsd">
<!--
  Auto-generated by powerlab-gateway. Do not edit by hand — the file
  is rewritten on every gateway restart and removed on shutdown.
  Add custom advertisements as a separate file in this directory.
-->
<service-group>
  <name replace-wildcards="yes">PowerLab on %%h</name>
  <service>
    <type>_http._tcp</type>
    <port>%d</port>
    <txt-record>path=/</txt-record>
    <txt-record>powerlab=true</txt-record>
  </service>
</service-group>
`, port)

	dst := filepath.Join(avahiServicesDir, avahiServiceFile)
	return os.WriteFile(dst, []byte(xml), 0o644)
}

// removeAvahiServiceFile is the Shutdown counterpart. Best-effort.
func removeAvahiServiceFile() error {
	dst := filepath.Join(avahiServicesDir, avahiServiceFile)
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
