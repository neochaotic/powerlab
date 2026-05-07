package route

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS-Gateway/api/docs"
	docs_static "github.com/IceWhaleTech/CasaOS-Gateway/api/static/docs"
	"github.com/IceWhaleTech/CasaOS-Gateway/service"
	"go.uber.org/zap"
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

func NewDocsRoute(state *service.State) *DocsRoute {
	return &DocsRoute{state: state}
}

func (d *DocsRoute) Register(mux *http.ServeMux) {
	mux.HandleFunc("/docs", d.handleDocs)
	mux.HandleFunc("/docs/spec", d.handleSpec)
	mux.HandleFunc("/docs/scalar.js", d.handleScalarJS)
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
		logger.Error("portal template parse failed", zap.Error(err))
		http.Error(w, "portal template unavailable", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		logger.Error("portal template render failed", zap.Error(err))
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
	d.serveSpec(w, current.Spec)
}

func (d *DocsRoute) handleScalarJS(w http.ResponseWriter, r *http.Request) {
	content, err := docs_static.EmbeddedAssets.ReadFile(docs_static.ScalarJSName)
	if err != nil {
		logger.Error("Failed to read embedded Scalar runtime", zap.Error(err))
		http.Error(w, "asset not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if _, err := w.Write(content); err != nil {
		logger.Error("Failed to write Scalar runtime", zap.Error(err))
	}
}

// serveSpec writes the embedded YAML directly. Content-Type is set
// to a YAML-aware media type so Scalar's loader picks the right
// parser.
func (d *DocsRoute) serveSpec(w http.ResponseWriter, filename string) {
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
		logger.Error("Failed to read embedded OpenAPI spec",
			zap.String("filename", filename), zap.Error(err))
		http.Error(w, "specification not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if _, err := w.Write(content); err != nil {
		logger.Error("Failed to write OpenAPI spec", zap.Error(err))
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
