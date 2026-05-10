package route

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/neochaotic/powerlab/backend/gateway/api/docs"
	docs_static "github.com/neochaotic/powerlab/backend/gateway/api/static/docs"
	"github.com/neochaotic/powerlab/backend/gateway/service"
)

// DocsRoute serves the API documentation portal at:
//
//	GET /docs[?service=<id>]   the Scalar host page (HTML)
//	GET /docs/spec?service=<id> the OpenAPI YAML for one service
//	GET /docs/scalar.js         the bundled Scalar runtime
//
// The portal is rendered by Scalar (https://github.com/scalar/scalar),
// a one-script API reference that consumes OpenAPI 3.x YAML. We
// embed both the per-service specs (regenerated on every build by
// start.sh) and the Scalar runtime so the gateway binary is fully
// self-contained — no CDN, no internet dependency at runtime, fitting
// the LAN-only deployment posture in ADR 0007.
//
// Specs are NEVER mutated by this package. Branding is applied
// purely via the Scalar config in portal.html (theme + metaData),
// not by editing the source openapi.yaml — keeping the YAML
// authoritative and round-trippable through codegen.
type DocsRoute struct {
	state *service.State

	// Compiled lazily on first request. Sticky failure on parse
	// error so the bug surfaces in logs rather than silently
	// re-attempting on every request.
	tmplOnce sync.Once
	tmpl     *template.Template
	tmplErr  error
}

// NewDocsRoute constructs the API-docs route bundle (Scalar viewer
// served from the gateway at /docs, per ADR-0008). State carries the
// runtime version stamp displayed in the docs portal header.
func NewDocsRoute(state *service.State) *DocsRoute {
	return &DocsRoute{state: state}
}

func (d *DocsRoute) Register(mux *http.ServeMux) {
	mux.HandleFunc("/docs", d.handleDocs)
	mux.HandleFunc("/docs/spec", d.handleSpec)
	mux.HandleFunc("/docs/scalar.js", d.handleScalarJS)
	mux.HandleFunc("/docs/logo.svg", d.handleLogo)
}

// portalView is the data passed into portal.html.
type portalView struct {
	Title    string
	SpecURL  string
	Services []portalServiceOption
}

type portalServiceOption struct {
	ID       string
	Label    string
	Selected bool
}

// handleDocs renders the Scalar host page with the requested service
// pre-selected. Unknown service ids fall back to the gateway service
// so a stale bookmark or copy-paste typo still lands on a working
// page (rather than a JSON 404).
func (d *DocsRoute) handleDocs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("service")
	current, ok := docs.LookupService(id)
	if !ok {
		current, _ = docs.LookupService("gateway")
	}

	view := portalView{
		Title:   fmt.Sprintf("PowerLab API · %s", current.Label),
		SpecURL: "/docs/spec?service=" + current.ID,
	}
	for _, s := range docs.Services {
		view.Services = append(view.Services, portalServiceOption{
			ID:       s.ID,
			Label:    s.Label,
			Selected: s.ID == current.ID,
		})
	}

	tmpl, err := d.template()
	if err != nil {
		_log.Error(r.Context(), "portal template parse failed", err)
		http.Error(w, "portal template unavailable", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		_log.Error(r.Context(), "portal template render failed", err)
		http.Error(w, "portal render failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(buf.Bytes())
}

// handleSpec returns the embedded OpenAPI YAML for the requested
// service. Unknown ids fall back to the gateway spec, same rule as
// the host page.
func (d *DocsRoute) handleSpec(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("service")
	current, ok := docs.LookupService(id)
	if !ok {
		current, _ = docs.LookupService("gateway")
	}
	d.serveSpec(w, r, current.Spec)
}

func (d *DocsRoute) handleScalarJS(w http.ResponseWriter, r *http.Request) {
	content, err := docs_static.EmbeddedAssets.ReadFile(docs_static.ScalarJSName)
	if err != nil {
		_log.Error(r.Context(), "Failed to read embedded Scalar runtime", err)
		http.Error(w, "asset not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if _, err := w.Write(content); err != nil {
		_log.Error(r.Context(), "Failed to write Scalar runtime", err)
	}
}

// handleLogo serves the PowerLab squircle logo (the same one used in
// the Launchpad / favicon) so each spec's `info.description` can
// reference `/docs/logo.svg` without depending on the SPA being
// served by the gateway. Vendored as docs/powerlab.svg, embed.FS.
func (d *DocsRoute) handleLogo(w http.ResponseWriter, r *http.Request) {
	content, err := docs_static.EmbeddedAssets.ReadFile(docs_static.PowerLabLogoSVG)
	if err != nil {
		_log.Error(r.Context(), "Failed to read embedded PowerLab logo", err)
		http.Error(w, "asset not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if _, err := w.Write(content); err != nil {
		_log.Error(r.Context(), "Failed to write PowerLab logo", err)
	}
}

// serveSpec writes the embedded YAML directly. Content-Type is set
// to a YAML-aware media type so Scalar's loader picks the right
// parser.
func (d *DocsRoute) serveSpec(w http.ResponseWriter, r *http.Request, filename string) {
	// Defensive: the filename always comes from a lookup over
	// docs.Services, but reject anything with slashes or `..`
	// components anyway, in case future code lets a request param
	// reach this far.
	if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
		http.Error(w, "invalid spec name", http.StatusBadRequest)
		return
	}

	content, err := docs.EmbeddedFiles.ReadFile(filename)
	if err != nil {
		_log.Error(r.Context(), "Failed to read embedded OpenAPI spec",
			err, slog.String("filename", filename))
		http.Error(w, "specification not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if _, err := w.Write(content); err != nil {
		_log.Error(r.Context(), "Failed to write OpenAPI spec", err)
	}
}

// template lazily parses portal.html. Returns the same instance on
// repeated calls; failure is sticky.
func (d *DocsRoute) template() (*template.Template, error) {
	d.tmplOnce.Do(func() {
		raw, err := docs.EmbeddedFiles.ReadFile(docs.PortalTemplateName)
		if err != nil {
			d.tmplErr = fmt.Errorf("read %s: %w", docs.PortalTemplateName, err)
			return
		}
		d.tmpl, d.tmplErr = template.New("portal").Parse(string(raw))
	})
	return d.tmpl, d.tmplErr
}
