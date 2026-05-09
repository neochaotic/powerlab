package route

import (
	"net/http"
	"strings"

	"github.com/IceWhaleTech/CasaOS-Common/pkg/security"
	"github.com/neochaotic/powerlab/backend/gateway/service"
)

type GatewayRoute struct {
	management *service.Management
	cm         *security.CertManager
	security   *SecurityRoute
	docs       *DocsRoute
}

func NewGatewayRoute(management *service.Management, cm *security.CertManager, state *service.State) *GatewayRoute {
	return &GatewayRoute{
		management: management,
		cm:         cm,
		security:   NewSecurityRoute(cm),
		docs:       NewDocsRoute(state),
	}
}

// the function is to ensure the request source IP is correct.
func rewriteRequestSourceIP(r *http.Request) {
	// we may receive two kinds of requests. a request from reverse proxy. a request from client.

	// in reverse proxy, X-Forwarded-For will like
	// `X-Forwarded-For:[192.168.6.102]`(normal)
	// `X-Forwarded-For:[::1, 192.168.6.102]`(hacked) Note: the ::1 is inject by attacker.
	// `X-Forwarded-For:[::1]`(normal or hacked) local request. But it from browser have JWT. So we can and need to verify it
	// `X-Forwarded-For:[::1,::1]`(normal or hacked) attacker can build the request to bypass the verification.
	// But in the case. the remoteAddress should be the real ip. So we can use remoteAddress to verify it.

	ipList := []string{}

	// when r.Header.Get("X-Forwarded-For") is "". the ipList should be empty.
	// fix https://github.com/IceWhaleTech/CasaOS/issues/1247
	if r.Header.Get("X-Forwarded-For") != "" {
		ipList = strings.Split(r.Header.Get("X-Forwarded-For"), ",")

		// when r.Header.Get("X-Forwarded-For") is "". to clean the ipList.
		// fix https://github.com/IceWhaleTech/CasaOS/issues/1247
		if len(ipList) == 1 && ipList[0] == "" {
			ipList = []string{}
		}
	}

	r.Header.Del("X-Forwarded-For")
	r.Header.Del("X-Real-IP")

	// Note: the X-Forwarded-For depend the correct config from reverse proxy.
	// otherwise the X-Forwarded-For may be empty.
	remoteIP := r.RemoteAddr[:strings.LastIndex(r.RemoteAddr, ":")]
	if len(ipList) > 0 && (remoteIP == "127.0.0.1" || remoteIP == "::1") {
		// to process the request from reverse proxy

		// in reverse proxy, X-Forwarded-For will container multiple IPs.
		// if the request is from reverse proxy, the r.RemoteAddr will be 127.0.0.1.
		// So we need get ip from X-Forwarded-For
		r.Header.Add("X-Forwarded-For", ipList[len(ipList)-1])
	}
	// to process the request from client.
	// the gateway will add the X-Forwarded-For to request header.
	// So we didn't need to add it.
}

func (g *GatewayRoute) GetRoute() *http.ServeMux {
	gatewayMux := http.NewServeMux()

	// Security routes (CA download, mobileconfig, trust-confirmed) —
	// must register BEFORE the catch-all "/" handler so the more
	// specific paths win. Handlers live in security_route.go.
	g.security.Register(gatewayMux)
	g.docs.Register(gatewayMux)

	gatewayMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("pong from gateway service")); err != nil {
				_log.Error(r.Context(), "Failed to `pong` in resposne to `ping`", err)
			}
			return
		}

		proxy := g.management.GetProxy(r.URL.Path)

		if proxy == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// to fix https://github.com/IceWhaleTech/CasaOS/security/advisories/GHSA-32h8-rgcj-2g3c#event-102885
		// API V1 and V2 both read ip from request header. So the fix is effective for v1 and v2.
		rewriteRequestSourceIP(r)

		proxy.ServeHTTP(w, r)
	})

	return gatewayMux
}

// WrapHSTS returns a handler that wraps `next` with the HSTS gate
// behavior described in ADR 0006:
//
//   - On HTTPS requests: emit `Strict-Transport-Security` header IF
//     `IsHSTSArmed()` returns true. Always pass through to next.
//   - On HTTP requests:
//     · If gate is armed → 301 redirect to https://<host>:<httpsPort>
//     · If gate is NOT armed → pass through to next (HTTP keeps working
//     so the user can complete the trust dance).
//
// httpsPort is the port the HTTPS listener is bound to (typically
// "8443"). If empty, the redirect omits the port (so it goes to the
// default 443).
func (g *GatewayRoute) WrapHSTS(next http.Handler, httpsPort string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		armed := g.cm.IsHSTSArmed()
		disarming := g.cm.IsHSTSDisarming()

		if r.TLS != nil {
			// Disarming window takes precedence over armed — even
			// if the user re-armed quickly, any browser that
			// already pinned needs its pin cleared first. RFC 6797
			// §6.1.1: max-age=0 evicts the cached entry.
			switch {
			case disarming:
				w.Header().Set("Strict-Transport-Security",
					"max-age=0")
			case armed:
				w.Header().Set("Strict-Transport-Security",
					"max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
			return
		}
		// Plain HTTP. Redirect only when armed; otherwise serve as
		// normal so the user can finish the trust dance over HTTP.
		// During the disarming window, HTTP also works (no redirect)
		// so a browser that just had its pin cleared can recover.
		if !armed || disarming {
			next.ServeHTTP(w, r)
			return
		}
		host := r.Host
		// Strip any incoming port; we always rewrite to the HTTPS port.
		if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host[i:], "]") {
			host = host[:i]
		}
		target := "https://" + host
		if httpsPort != "" && httpsPort != "443" {
			target += ":" + httpsPort
		}
		target += r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}
