# Reverse proxy + TLS — operator recipes

> **The recommended path for any PowerLab deployment exposed beyond `localhost`.** PowerLab's in-product HTTPS (the self-signed CA + trust-dance flow you may have seen in earlier versions) is **deprecated** as of v0.7.8 — see [the deprecation note](#why-this-page-exists) below. Use one of the recipes here instead; every modern self-hosted stack assumes TLS termination at the edge anyway.

## Why this page exists

PowerLab originally shipped an in-product HTTPS flow: it generated a local CA, issued a server cert, and asked the browser to trust the CA via a multi-step "trust dance" probe ([ADRs 0009–0012](../decisions/)). That approach was always a bug magnet — browser HSTS poisoning, false-positive trust-state probes, persistent localStorage markers, and an unconditional `:8443` listener that confused operators behind reverse proxies. The trust-dance sprint was [deprioritized indefinitely](../decisions/) on 2026-05-11. With the enterprise pivot (2026-05-19) it became clear PowerLab IS NOT in the certificate-authority business — every enterprise IT department, every cloud provider, every modern container platform terminates TLS at the edge.

**The new contract:** PowerLab speaks plain HTTP on `:8765` (panel) and `:9090` (MCP). You put any TLS terminator in front. The recipes below are five paths the team has actually shipped.

> ⚠️ **Security floor.** PowerLab's panel auth is a JWT in `localStorage` and the MCP server's LAN auth is a `Authorization: Bearer <token>` header. Both are **trivial to intercept on plain HTTP over an untrusted network**. If your PowerLab box is reachable from anything other than `localhost` or a VPN you trust, **you MUST front it with a TLS terminator** (one of the recipes below). The same applies to the MCP `:9090` endpoint when a LAN client uses it.

---

## Pick your shape

| Your situation | Recommended recipe |
|---|---|
| Single VPS / cloud VM with a domain | [Caddy (auto Let's Encrypt)](#1-caddy-recommended-for-most-self-hosted-deployments) |
| Existing nginx / want manual control | [nginx + certbot](#2-nginx--certbot-the-classic) |
| Home LAN, no domain, no port-forward | [Tailscale Funnel](#3-tailscale-funnel-zero-config-https-on-your-tailnet) |
| Home LAN, want public access without exposing your IP | [Cloudflare Tunnel](#4-cloudflare-tunnel-no-inbound-firewall-rules) |
| Managed deployment on a cloud provider | [Cloud LB (EC2 ALB, GCP HTTPS LB, Azure App GW)](#5-cloud-managed-load-balancer-ec2--gcp--azure) |
| Kubernetes | [Ingress controller + cert-manager](#6-kubernetes-ingress--cert-manager) |

PowerLab does NOT prefer one over the others — pick whichever fits your existing tooling.

---

## 1. Caddy (recommended for most self-hosted deployments)

[Caddy 2](https://caddyserver.com/docs/automatic-https) does automatic Let's Encrypt enrolment + renewal + HTTP→HTTPS redirect with **two lines of config**. If you have a domain pointing at your PowerLab host, this is the lowest-effort path.

### Install + run

```bash
# Debian / Ubuntu
sudo apt install -y caddy

# Or Docker
docker run -d --name caddy --restart unless-stopped \
  -p 80:80 -p 443:443 \
  -v $PWD/Caddyfile:/etc/caddy/Caddyfile \
  -v caddy_data:/data \
  caddy
```

### `/etc/caddy/Caddyfile`

```caddy
# Panel
panel.example.com {
    reverse_proxy localhost:8765
}

# MCP (only if you want LAN agents to reach it)
mcp.example.com {
    reverse_proxy localhost:9090
}
```

```bash
sudo systemctl reload caddy
```

Done. Caddy provisions the Let's Encrypt cert on the first request, renews automatically. Browsers reach `https://panel.example.com` with a valid cert.

### MCP client config

```json
{
  "mcpServers": {
    "powerlab": {
      "command": "npx",
      "args": [
        "-y", "mcp-remote@latest",
        "https://mcp.example.com/mcp",
        "--header", "Authorization: Bearer <your-token>"
      ]
    }
  }
}
```

(Note `https://` and no `--allow-http` — Caddy is doing TLS now.)

---

## 2. nginx + certbot (the classic)

If you already run nginx, drop a `server` block in.

### Install certbot

```bash
sudo apt install -y nginx python3-certbot-nginx
```

### `/etc/nginx/sites-available/powerlab.conf`

```nginx
server {
    listen 80;
    server_name panel.example.com mcp.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name panel.example.com;

    # certbot fills these in automatically (next step)
    ssl_certificate     /etc/letsencrypt/live/panel.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/panel.example.com/privkey.pem;

    location / {
        proxy_pass         http://localhost:8765;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        # PowerLab serves SSE on /v1/audit/stream and similar — keep
        # buffering off so events flush in real time.
        proxy_buffering    off;
        proxy_read_timeout 1d;
    }
}

server {
    listen 443 ssl http2;
    server_name mcp.example.com;

    ssl_certificate     /etc/letsencrypt/live/mcp.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/mcp.example.com/privkey.pem;

    location / {
        proxy_pass         http://localhost:9090;
        proxy_http_version 1.1;
        proxy_set_header   Connection          "";
        proxy_set_header   Host                $host;
        proxy_set_header   X-Forwarded-For     $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto   $scheme;
        # MCP streaming HTTP: never buffer; SSE-style responses must
        # flush per chunk.
        proxy_buffering    off;
        proxy_read_timeout 1d;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/powerlab.conf /etc/nginx/sites-enabled/
sudo certbot --nginx -d panel.example.com -d mcp.example.com
sudo systemctl reload nginx
```

certbot edits the `ssl_certificate*` lines in-place and installs a renewal timer. Done.

---

## 3. Tailscale Funnel (zero-config HTTPS on your tailnet)

If your PowerLab box is on a [Tailscale](https://tailscale.com) network already, Funnel exposes a tailnet service at a real `https://<machine>.<tail>.ts.net` URL with a Tailscale-managed Let's Encrypt cert. **No domain, no port-forward, no firewall rules.**

```bash
# On the PowerLab host
sudo tailscale up                    # if not already up
sudo tailscale funnel 8765 on        # panel goes live at the ts.net URL
```

Tailscale prints the URL (`https://<hostname>.<tailnet>.ts.net`). Use that in browsers + MCP configs.

For MCP separately:
```bash
sudo tailscale serve --bg --tcp 443 tcp://localhost:9090   # tailnet-only HTTPS
# Or for public Funnel exposure (be careful — public!)
sudo tailscale funnel 9090 on
```

**Trade-off:** Funnel is rate-limited and intended for low/moderate-volume control-plane traffic, which fits PowerLab's panel-style usage. Don't put a media stream behind Funnel.

---

## 4. Cloudflare Tunnel (no inbound firewall rules)

[Cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) creates an outbound tunnel from your host to Cloudflare's edge. Your PowerLab box never needs an inbound port open — useful behind CGNAT or restrictive ISPs.

```bash
# Install
curl -fsSL https://pkg.cloudflare.com/install.sh | sudo bash
sudo apt install -y cloudflared

# Authenticate (opens a browser)
cloudflared tunnel login

# Create a tunnel
cloudflared tunnel create powerlab

# Configure
cat > ~/.cloudflared/config.yml <<EOF
tunnel: powerlab
credentials-file: /root/.cloudflared/<tunnel-id>.json

ingress:
  - hostname: panel.example.com
    service: http://localhost:8765
  - hostname: mcp.example.com
    service: http://localhost:9090
  - service: http_status:404
EOF

# Route the hostnames + start
cloudflared tunnel route dns powerlab panel.example.com
cloudflared tunnel route dns powerlab mcp.example.com
sudo cloudflared service install
sudo systemctl start cloudflared
```

Cloudflare terminates TLS at the edge; the tunnel carries plain HTTP back to localhost. Operator threat model: Cloudflare can see panel content + MCP traffic — acceptable for most use cases, but read [Cloudflare's data handling](https://developers.cloudflare.com/cloudflare-one/policies/data-loss-prevention/) before sending sensitive workloads through.

---

## 5. Cloud managed load balancer (EC2 / GCP / Azure)

Managed cloud TLS termination is the standard production shape — your cloud provider manages the certificate, renews it automatically, and gives you DDoS + WAF if you want them.

### AWS — ALB + ACM

1. Request a public cert in **AWS Certificate Manager** for your domain.
2. Create an **Application Load Balancer** in front of your EC2 instance.
3. Listener: `HTTPS:443` → forward to a Target Group at `HTTP:8765` (panel) or `HTTP:9090` (MCP).
4. Attach the ACM cert to the HTTPS listener.
5. Security group on the EC2: inbound `8765` + `9090` from the ALB's security group only (not the world).

### GCP — HTTPS Load Balancer

1. Reserve a global static IP.
2. Create a backend service pointing at your Compute Engine instance group on port `8765`.
3. Create a Google-managed certificate for your domain.
4. Create an HTTPS Load Balancer with the backend service + the cert.
5. Firewall rule: allow inbound from the GCP LB CIDR (`130.211.0.0/22` + `35.191.0.0/16`) to ports `8765` + `9090`.

### Azure — Application Gateway

1. Create an Application Gateway with a public IP.
2. HTTPS listener on `443` with an Azure-managed cert or one you upload.
3. Backend pool: your VM on `8765` (and a second pool on `9090` if you want MCP exposed).
4. Path-based routing or host-based routing as you prefer.

Common note across all three: the LB's health check should target `/healthz` on the gateway (`http://<vm>:8765/healthz`).

---

## 6. Kubernetes (Ingress + cert-manager)

If you're running PowerLab in a container on Kubernetes — sample `Ingress` for the standard `nginx-ingress-controller` + `cert-manager`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: powerlab
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/proxy-body-size: "20m"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "86400"
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
spec:
  ingressClassName: nginx
  tls:
    - hosts: [panel.example.com, mcp.example.com]
      secretName: powerlab-tls
  rules:
    - host: panel.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: powerlab-gateway
                port: { number: 8765 }
    - host: mcp.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: powerlab-mcp
                port: { number: 9090 }
```

cert-manager handles enrolment + renewal. The annotations disable buffering and bump the read timeout so PowerLab's SSE-style streams (audit feed, MCP responses) work.

---

## Migration from in-product HTTPS (v0.7.7 and earlier)

If you previously turned on PowerLab's in-product HTTPS:

1. **Pick a recipe above and get it working** on `https://<your-domain>` → `http://<host>:8765`.
2. **Test in a private window** — confirm the panel loads, JWT login works, the audit stream renders without buffering.
3. **Stop relying on the `:8443` listener.** PowerLab v0.7.8 still binds it if a server cert exists from a prior install, but the in-product flow is deprecated. The startup log emits a deprecation warning.
4. **(Optional, ~v0.7.10) Clean up the local trust state**:
   ```bash
   sudo systemctl stop powerlab-gateway
   sudo rm -rf /etc/powerlab/tls
   sudo systemctl start powerlab-gateway
   ```
   That removes the local CA + server cert. PowerLab will boot HTTP-only on `:8765` afterwards, and your reverse proxy handles TLS.
5. **Browser cleanup** (only if you were testing trust-dance in earlier versions and the browser has a sticky HSTS entry for `https://<powerlab-host>:8443`):
   - Chrome: `chrome://net-internals/#hsts` → Delete domain security policies → `<powerlab-host>`
   - Firefox: hold Shift while clicking the reload button on the HTTPS page; or clear site data for the host

---

## Verification

After your reverse proxy is in place:

```bash
# Panel reachable, valid cert
curl -fI https://panel.example.com/healthz
# → 200, Server: <your reverse proxy>

# MCP reachable, valid cert
curl -fI https://mcp.example.com/version
# → 200

# JWT login still works
curl -fsSL -X POST https://panel.example.com/v1/users/login \
    -H 'Content-Type: application/json' \
    -d '{"username":"<u>","password":"<p>"}'
# → returns a JWT
```

If the panel renders cleanly and `audit://` stream flushes events in real time, the reverse proxy is correctly disabling response buffering. If audit feels "laggy," check that `proxy_buffering off` (nginx) or the equivalent setting is on.

---

## Where to learn more

- [ADR-0044](../decisions/0044-mcp-hybrid-architecture-thin-proxy-to-core.md) — gateway + MCP transport overview (what the reverse proxy fronts)
- [MCP operator quickstart — Step 3](mcp-quickstart.md#step-3--pair-your-ai-client-3-minutes) — pairing AI clients with the reverse-proxied MCP endpoint
- [Caddy docs — Automatic HTTPS](https://caddyserver.com/docs/automatic-https)
- [Tailscale Funnel](https://tailscale.com/kb/1223/funnel/)
- [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
