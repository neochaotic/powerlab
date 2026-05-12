package main

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// volumePlaceholderRE matches any Umbrel-ecosystem placeholder that
// appears inside a service's volume reference. Three flavors land here:
//
//   - `${APP_DATA_DIR}`           — this app's own data dir
//   - `${APP_<NAME>_DATA_DIR}`    — a SIBLING app's data dir (e.g. an
//                                   app that depends on Lightning Node
//                                   refers to `${APP_LIGHTNING_NODE_DATA_DIR}`)
//   - `${UMBREL_ROOT}`            — Umbrel's installation root, used by
//                                   apps that read from `/data/storage/`
//                                   (downloads, music, paperless export, …)
//
// All three need to substitute to PATHS that compose-go accepts as bind
// mounts (start with `/`). The actual on-disk paths don't need to exist
// at catalog-read time — the validator only checks the FORMAT (is it a
// path or a named volume?). Apps that depend on sibling-app data dirs
// won't actually work in PowerLab without those siblings installed, but
// they'll at least surface in the store UI so a maintainer can decide.
var volumePlaceholderRE = regexp.MustCompile(`\$\{(APP_[A-Z0-9_]*DIR|UMBREL_ROOT)\}`)

// portPlaceholderRE matches both shell-var forms — `${APP_FOO_PORT}` AND
// the brace-less `$APP_FOO_PORT`. Some upstream Umbrel composes use the
// short form (synapse: `ports: - 8008:$APP_SYNAPSE_PORT`), so a regex
// that only matches braces silently skips them and the port spec ships
// as a literal `$VAR` that compose-go's strict port parser rejects.
var portPlaceholderRE = regexp.MustCompile(`\$\{?[A-Z_][A-Z0-9_]*\}?`)

// Umbrel-specific transform: the upstream `docker-compose.yml` files
// in `getumbrel/umbrel-apps` assume an Umbrel runtime that we don't
// replicate. Two patterns appear in ~95% of upstream apps and both
// break PowerLab's compose loader (`compose-go` strict validator):
//
//  1. A `services.app_proxy` entry with neither `image:` nor `build:`.
//     Umbrel's runtime swaps this in for a real reverse-proxy
//     container at install time; standalone compose-go rejects with
//     "service has neither an image nor a build context specified".
//     PowerLab's gateway already handles reverse-proxy concerns at the
//     platform layer, so we drop this service outright.
//
//  2. Volume references like `${APP_DATA_DIR}/data:/a0`. Umbrel
//     substitutes APP_DATA_DIR at install time with an app-specific
//     directory under `/home/umbrel/umbrel/app-data/<id>/data`.
//     compose-go treats the un-substituted `${APP_DATA_DIR}/data` as
//     a NAMED VOLUME reference, looks for `volumes.${APP_DATA_DIR}/data`
//     at the top level, doesn't find it, and rejects the project as
//     "service refers to undefined volume". We substitute with PowerLab's
//     ADR-0021 AppData path (`/DATA/PowerLabAppData/<store_app_id>`),
//     which becomes a bind mount and the validator accepts.
//
// Why we transform at sync time rather than at install time: the
// compose YAML is read by `app-management.service.BuildCatalog` BEFORE
// the app is installed, just to populate the catalog UI. If it doesn't
// parse, the app never appears, regardless of install logic. Sync-time
// transform makes the YAML self-consistent for the catalog walker, and
// the resulting `/DATA/PowerLabAppData/<id>` path is also the correct
// install-time path (no second substitution needed).
//
// Trade-offs: this loses one Umbrel feature — the `app_proxy` service's
// `APP_HOST` + `APP_PORT` environment hints that Umbrel uses to wire its
// reverse proxy. PowerLab's port-mapping flow uses `x-powerlab.port_map`
// instead (already emitted in the x-powerlab block from `emit.go`), so
// the proxy hints are functionally redundant.

// transformUpstreamCompose rewrites the upstream Umbrel docker-compose
// YAML so PowerLab's compose-go loader accepts it. The returned bytes
// are functionally-equivalent compose YAML; comments + key ordering
// are NOT preserved (we round-trip through `map[string]any`).
//
// We accept the formatting loss because:
//   - The emitted file is machine-written, machine-read; no maintainer
//     edits the post-sync compose YAML by hand
//   - The maintainer override path (`description-powerlab.md`) handles
//     human-curated content separately
//   - yaml.v3 with a yaml.Node round-trip would preserve formatting
//     but adds ~80 LOC of node walking for marginal benefit
func transformUpstreamCompose(upstream []byte, storeAppID string, basePort int) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(upstream, &doc); err != nil {
		return nil, fmt.Errorf("parse upstream compose: %w", err)
	}
	// yaml.Unmarshal leaves doc == nil when input is empty/null.
	// Allocate so writes don't panic; the resulting compose with
	// just `name:` is degenerate but won't crash the sync run.
	if doc == nil {
		doc = make(map[string]any)
	}

	// Set top-level `name:` so compose-go's project name resolves
	// to the store_app_id instead of a random temp-dir basename.
	// Without this, BuildCatalog keys the catalog map by random
	// generated names (e.g. "amazing_ubs") instead of "agent-zero",
	// so the API list returns the app under a non-discoverable key
	// — exactly the bug surfaced on the user's box on 2026-05-12.
	// CasaOS-AppStore composes ship with `name: <id>` at the top
	// for the same reason; we mirror the convention here.
	doc["name"] = storeAppID

	// Order matters: extract app_proxy routing info BEFORE stripping
	// the service, then strip, then add the equivalent `ports:` mapping
	// to the target service. Without this, apps that exposed their port
	// solely via Umbrel's app_proxy (e.g. enclosed — no `ports:` in the
	// real service) lose all external accessibility after the strip and
	// the launchpad click-through opens to nothing.
	target, appPort, hasProxy := extractAppProxyTarget(doc, storeAppID)
	stripAppProxyService(doc)
	if hasProxy && basePort > 0 {
		addPortMapping(doc, target, appPort, basePort)
	}
	substituteAppDataDir(doc, storeAppID)
	dropEnvFileFromServices(doc)
	substitutePortPlaceholders(doc, basePort)

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal transformed compose: %w", err)
	}
	return out, nil
}

// extractAppProxyTarget reads Umbrel's `services.app_proxy.environment`
// to recover the inner service name + container port it was routing to.
// `APP_HOST` follows Umbrel's `<projectID>_<svcName>_<replica>` convention
// (e.g. `enclosed_web_1`); `APP_PORT` is the container's listening port.
// We need both BEFORE `stripAppProxyService` removes the service — once
// stripped, this signal is lost and we can't recover the port mapping.
//
// Returns ok=false if app_proxy isn't present or env is incomplete; the
// caller must skip the addPortMapping step in that case so apps that
// already expose `ports:` in the real service aren't double-mapped.
func extractAppProxyTarget(doc map[string]any, storeAppID string) (svcName, appPort string, ok bool) {
	services, sOK := doc["services"].(map[string]any)
	if !sOK {
		return "", "", false
	}
	proxy, pOK := services["app_proxy"].(map[string]any)
	if !pOK {
		return "", "", false
	}
	env, _ := proxy["environment"].(map[string]any)
	if env == nil {
		return "", "", false
	}
	host, _ := env["APP_HOST"].(string)
	// APP_PORT can be unmarshaled as int or string depending on whether
	// the upstream wrote `APP_PORT: 8080` (int) or `APP_PORT: "8080"` (str).
	switch p := env["APP_PORT"].(type) {
	case string:
		appPort = p
	case int:
		appPort = fmt.Sprintf("%d", p)
	}
	// Resolve APP_HOST to a service name via four strategies in order:
	//
	//   A. Direct match against a service's `hostname:` field
	//      (cloudflared-style: APP_HOST: cloudflared-web ↔ hostname:
	//      cloudflared-web on the web service).
	//
	//   B. Direct match against `container_name:` (searxng-style).
	//
	//   C. The `<storeAppID>_<svcName>_<replica>` convention — strip
	//      the storeAppID prefix + trailing `_<digits>`. This is the
	//      default Umbrel pattern (enclosed_web_1 → web).
	//
	//   D. Shell-var fallback: APP_HOST = `$APP_FOO_IP`. We can't
	//      resolve at sync time, so pick the first non-proxy service.
	//      The vast majority of apps with this pattern have a single
	//      "main" service anyway.
	//
	// The audit on 2026-05-12 surfaced cloudflared/searxng (hostname),
	// no apps using container_name (searxng-style is rare upstream),
	// and agora (shell-var) — all three flavors handled here.

	// (A) hostname match
	for sName, sAny := range services {
		if sName == "app_proxy" {
			continue
		}
		svc, _ := sAny.(map[string]any)
		if svc == nil {
			continue
		}
		if hn, _ := svc["hostname"].(string); hn != "" && hn == host {
			return sName, appPort, appPort != ""
		}
	}

	// (B) container_name match
	for sName, sAny := range services {
		if sName == "app_proxy" {
			continue
		}
		svc, _ := sAny.(map[string]any)
		if svc == nil {
			continue
		}
		if cn, _ := svc["container_name"].(string); cn != "" && cn == host {
			return sName, appPort, appPort != ""
		}
	}

	// (C) <storeAppID>_<svc>_<replica> convention
	prefix := storeAppID + "_"
	if strings.HasPrefix(host, prefix) {
		rest := strings.TrimPrefix(host, prefix)
		if i := strings.LastIndex(rest, "_"); i > 0 {
			svcName = rest[:i]
		} else {
			svcName = rest
		}
		return svcName, appPort, svcName != "" && appPort != ""
	}

	// (D) Shell-var fallback: pick first non-proxy service.
	if strings.HasPrefix(host, "$") || host == "" {
		for sName := range services {
			if sName != "app_proxy" {
				return sName, appPort, appPort != ""
			}
		}
	}

	return "", "", false
}

// addPortMapping adds a `ports:` entry to the target service so the
// host port == basePort (= manifest.Port = x-powerlab.port_map) routes
// to the container's internal appPort. No-op if the target service
// already declares a `ports:` list (don't override the upstream's
// explicit port choices).
func addPortMapping(doc map[string]any, svcName, appPort string, basePort int) {
	services, _ := doc["services"].(map[string]any)
	svc, ok := services[svcName].(map[string]any)
	if !ok {
		return
	}
	if existing, has := svc["ports"].([]any); has && len(existing) > 0 {
		return
	}
	svc["ports"] = []any{fmt.Sprintf("%d:%s", basePort, appPort)}
}

// stripAppProxyService removes `services.app_proxy` from the parsed
// compose document if present. No-op if the service or `services` key
// is absent. The function is tolerant of unexpected shapes — a
// non-map services key is left alone (the validator will surface the
// real shape error later).
func stripAppProxyService(doc map[string]any) {
	services, ok := doc["services"].(map[string]any)
	if !ok {
		return
	}
	delete(services, "app_proxy")
}

// dropEnvFileFromServices removes the `env_file:` directive from every
// service. Umbrel-emitted composes routinely reference env files like
// `${APP_DATA_DIR}/settings.env` which Umbrel writes at install time
// with user-provided values (passwords, API keys). Two problems for
// PowerLab catalog reads:
//
//  1. compose-go's loader tries to OPEN env_file paths at parse time
//     (to merge their content into the project env), and fails because
//     the path `${APP_DATA_DIR}/settings.env` is not a real path. The
//     whole compose project is rejected.
//
//  2. Even after `${APP_DATA_DIR}` substitution, the actual file
//     `/DATA/PowerLabAppData/<id>/settings.env` does NOT exist at
//     catalog-read time — it would only exist post-install, and only
//     if PowerLab grew an install-time env-file-generation step that
//     mirrors Umbrel's runtime.
//
// Decision: drop env_file directives from the catalog YAML. The app
// still appears in the store; an `environment:` list in the same
// service carries any default vars the upstream maintainer chose to
// inline. Apps that fundamentally depend on env_file (rare —
// usually only Umbrel-managed secrets) install but may fail at
// runtime without an installer-driven env step. That follow-up
// lives outside this fix; the catalog visibility issue is what
// v0.6.2 is fixing.
func dropEnvFileFromServices(doc map[string]any) {
	services, ok := doc["services"].(map[string]any)
	if !ok {
		return
	}
	for _, svc := range services {
		svcMap, ok := svc.(map[string]any)
		if !ok {
			continue
		}
		delete(svcMap, "env_file")
	}
}

// substitutePortPlaceholders rewrites `${APP_<NAME>_PORT}` /
// `${DOCSERVER_PORT}` style references inside `services.*.ports`
// entries. compose-go's port parser is strict — it expects either
// integers or `host:container` strings with both sides numeric — so
// any un-substituted env var in the port spec triggers
// "Invalid containerPort: ${...}" and the whole project is dropped.
//
// We substitute with a per-service sequential placeholder starting at
// 18000. The actual port mapping is recorded in `x-powerlab.port_map`
// (the field the PowerLab UI reads) — the runtime port the user picks
// at install time is independent of what's in the compose `ports:`.
// 18000 is high enough to avoid the well-known-ports range (which
// compose-go warns about) and low enough to be a valid TCP port.
//
// Service-level counter avoids collisions: if a compose has 3
// services with placeholder ports, each gets a distinct integer so
// the project still passes compose-go's "no duplicate host ports"
// check.
func substitutePortPlaceholders(doc map[string]any, basePort int) {
	services, ok := doc["services"].(map[string]any)
	if !ok {
		return
	}
	// First placeholder = basePort (== manifest's port:, == x-powerlab.port_map),
	// so the launchpad's click-through URL (built from port_map) hits the right
	// container port. Subsequent placeholders in multi-port apps get +1 to avoid
	// host-port collisions inside compose-go's validator. Fallback 18000 only
	// kicks in when the manifest has no port (rare; defensive).
	port := basePort
	if port <= 0 {
		port = 18000
	}
	for _, svc := range services {
		svcMap, ok := svc.(map[string]any)
		if !ok {
			continue
		}
		ports, ok := svcMap["ports"].([]any)
		if !ok {
			continue
		}
		for i, p := range ports {
			switch pp := p.(type) {
			case string:
				if strings.Contains(pp, "$") {
					ports[i] = replacePlaceholderPort(pp, &port)
				}
			case map[string]any:
				// Long-form: { target: 80, published: ${APP_X_PORT} }
				if pub, ok := pp["published"].(string); ok && strings.Contains(pub, "$") {
					pp["published"] = fmt.Sprintf("%d", port)
					port++
				}
				if tgt, ok := pp["target"].(string); ok && strings.Contains(tgt, "$") {
					pp["target"] = fmt.Sprintf("%d", port)
					port++
				}
			}
		}
	}
}

// replacePlaceholderPort returns the input with any shell-var-style
// placeholder (`${VAR}` or `$VAR`) replaced by an integer pulled from
// the counter. The counter is advanced once per substitution so
// multi-port specs (e.g. `${APP_HTTP}:$APP_TCP`) get distinct values.
// The regex-based replace handles both brace forms uniformly.
func replacePlaceholderPort(spec string, counter *int) string {
	return portPlaceholderRE.ReplaceAllStringFunc(spec, func(_ string) string {
		v := fmt.Sprintf("%d", *counter)
		*counter++
		return v
	})
}


// substituteAppDataDir walks every service's `volumes:` list and
// replaces literal `${APP_DATA_DIR}` substrings with PowerLab's
// app-scoped AppData path. Only the volumes list is touched — env
// vars and other places that reference `${APP_DATA_DIR}` are left
// alone because they don't trigger compose-go's validator (it cares
// about volume references, not env var values).
//
// We use ADR-0021's `/DATA/PowerLabAppData/<store_app_id>` namespace.
// This matches what a PowerLab user would expect: per-app data lives
// under the same root as other PowerLab-native apps.
//
// Volume entries can be either strings (`- ${APP_DATA_DIR}/foo:/bar`)
// or maps (`- {type: bind, source: ..., target: ...}`). We handle both
// shapes; non-string non-map entries are passed through unchanged.
func substituteAppDataDir(doc map[string]any, storeAppID string) {
	// Replacement function: maps each captured placeholder to a
	// PowerLab-style path. The default treats every `*_DATA_DIR` and
	// `${UMBREL_ROOT}` reference as "this app's own data". That's
	// imperfect for sibling-app dependencies (an app referencing
	// `${APP_LIGHTNING_NODE_DATA_DIR}` won't actually find Lightning
	// Node's data under this app's dir), but it lets the catalog
	// parse so the app surfaces in the store — letting the operator
	// see the app + decide what to do, instead of silently dropping
	// it.
	appData := fmt.Sprintf("/DATA/PowerLabAppData/%s", storeAppID)

	subst := func(s string) string {
		return volumePlaceholderRE.ReplaceAllStringFunc(s, func(match string) string {
			// match looks like `${UMBREL_ROOT}` or `${APP_FOO_DATA_DIR}`
			if match == "${UMBREL_ROOT}" {
				return "/DATA"
			}
			return appData
		})
	}

	services, ok := doc["services"].(map[string]any)
	if !ok {
		return
	}

	for _, svc := range services {
		svcMap, ok := svc.(map[string]any)
		if !ok {
			continue
		}
		volumes, ok := svcMap["volumes"].([]any)
		if !ok {
			continue
		}
		for i, v := range volumes {
			switch vv := v.(type) {
			case string:
				if strings.Contains(vv, "${") {
					volumes[i] = subst(vv)
				}
			case map[string]any:
				if src, ok := vv["source"].(string); ok && strings.Contains(src, "${") {
					vv["source"] = subst(src)
				}
			}
		}
	}
}
