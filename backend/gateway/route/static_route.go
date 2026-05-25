package route

import (
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
	"github.com/neochaotic/powerlab/backend/gateway/service"
)

// StaticRoute serves the SvelteKit SPA bundle from the gateway. The
// bundle comes from an fs.FS — either the UI embedded in the binary
// (ADR-0043) or an on-disk directory via the `-w` override — resolved
// by service.State.GetWWWFS(). Single-page-app routing falls back to
// index.html for any path that doesn't match a real asset, so the
// client-side router takes over after first load.
type StaticRoute struct {
	state *service.State
}

var startTime = time.Now()

// NewStaticRoute constructs the static-asset route bundle. State
// carries the runtime-injected version stamp + asset manifest used
// by the cache-busting logic.
func NewStaticRoute(state *service.State) *StaticRoute {
	return &StaticRoute{
		state: state,
	}
}

var indexRE = regexp.MustCompile(`/($|modules/[^\/]*/($|(index\.(html?|aspx?|cgi|do|jsp))|((default|index|home)\.php)))`)

// immutableAssetRE matches SvelteKit's content-hashed static assets.
// `_app/immutable/...` is the convention used by @sveltejs/adapter-static
// and the file names embed a contents hash (e.g. `hJICLhXx2.js`), so a
// given URL never changes — it is safe (and desirable) to tell the
// browser to cache it forever.
var immutableAssetRE = regexp.MustCompile(`^/(_app/immutable|favicon-[^/]+\.png|apple-touch-icon-[^/]+\.png)`)

func (s *StaticRoute) GetRoute() http.Handler {
	e := echo.New()

	e.Use(echo_middleware.Gzip())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			path := ctx.Request().URL.Path
			h := ctx.Response().Writer.Header()

			switch {
			case immutableAssetRE.MatchString(path):
				// Hashed assets are immutable for the life of the
				// release. A 1-year max-age + `immutable` lets every
				// modern browser skip the revalidation round-trip
				// entirely.
				h.Set("Cache-Control", "public, max-age=31536000, immutable")

			case indexRE.MatchString(path) || isHTMLPath(path):
				// index.html and SPA fallback routes (any path that
				// resolves to index.html). MUST be revalidated on
				// every load — without this, a browser holding a
				// stale index.html keeps loading old hashed asset
				// names long after the server has moved on, and the
				// version-handshake banner is the only way the user
				// learns about it. The handshake is the safety net;
				// no-cache here is the actual fix.
				h.Set("Cache-Control", "no-cache, no-store, must-revalidate, proxy-revalidate, max-age=0")
			}
			return next(ctx)
		}
	})

	// SPA fallback chain: try the literal path, then `<path>.html`
	// (SvelteKit's adapter-static emits `settings.html` for the
	// `/settings` route), then index.html (the fallback SPA shell that
	// adapter-static configures via `fallback: 'index.html'`). Without
	// this chain, deep links / refreshes / direct URL-bar navigation
	// to any client-side route return 404.
	//
	// We use http.ServeContent (not http.FileServer) so we can pick the
	// exact file to serve without triggering FileServer's automatic
	// "/index.html → /" 301 dance. GetWWWFS resolves to the on-disk
	// bundle (dev/`-w`/legacy install) or the embedded one (ADR-0043).
	wwwFS := http.FS(s.state.GetWWWFS())

	e.GET("/*", func(ctx echo.Context) error {
		urlPath := ctx.Request().URL.Path
		req := ctx.Request()
		w := ctx.Response().Writer
		served := serveSPAPath(w, req, wwwFS, urlPath)
		if !served {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		return nil
	})

	return e
}

// serveSPAPath resolves a request path against an adapter-static
// bundle (any http.FileSystem — embedded or on-disk) and writes the
// response. Returns false ONLY when no candidate exists (the caller
// turns that into 404).
//
// Resolution order:
//
//	asset-shaped path (under /_app/ or has a non-html extension)
//	  → serve the file directly, 404 on miss (no fallback so a
//	    missing asset stays a missing asset, not a stealth HTML).
//	"/" or any html-shaped path
//	  → try `<path>` (already a static file)
//	  → try `<path>.html` (adapter-static route convention)
//	  → try `<path>/index.html` (just in case)
//	  → fall back to `index.html` (SPA shell — handles unknown
//	    routes via client-side routing).
//
// Path traversal is structurally impossible: http.FS rejects any name
// that is not a valid slash-rooted path (no `..`, no absolute escape),
// so a malicious URL simply misses every candidate and 404s.
func serveSPAPath(w http.ResponseWriter, req *http.Request, hfs http.FileSystem, urlPath string) bool {
	clean := strings.TrimPrefix(path.Clean("/"+urlPath), "/")

	if isAssetPath(urlPath) {
		return serveFileFS(w, req, hfs, clean)
	}

	candidates := []string{
		clean,
		clean + ".html",
		path.Join(clean, "index.html"),
		"index.html",
	}
	for _, c := range candidates {
		if c == "" || c == "." {
			c = "index.html"
		}
		if serveFileFS(w, req, hfs, c) {
			return true
		}
	}
	return false
}

// serveFileFS serves a single named file from hfs, returning false if
// the name does not resolve to a regular file (missing, a directory,
// or rejected by http.FS as an invalid path). name is a slash path
// relative to the FS root, without a leading slash.
func serveFileFS(w http.ResponseWriter, req *http.Request, hfs http.FileSystem, name string) bool {
	if name == "" {
		return false
	}
	f, err := hfs.Open("/" + name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	http.ServeContent(w, req, info.Name(), info.ModTime(), f)
	return true
}

// isAssetPath returns true when a path looks like a versioned asset
// (under /_app/, hashed favicon, or with a file extension that is
// NOT .html). For these we never fall back to index.html — a missing
// asset should 404, not silently return HTML and break MIME sniffing.
func isAssetPath(path string) bool {
	if len(path) >= 5 && path[:5] == "/_app" {
		return true
	}
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			ext := path[i:]
			return ext != ".html"
		}
	}
	return false
}

// isHTMLPath returns true for paths that an SPA fallback would serve
// index.html for (basically: anything that isn't a hashed-asset URL).
// Used to widen the no-cache rule beyond bare "/" so SvelteKit-style
// SPA routes like "/files" or "/settings" also revalidate.
func isHTMLPath(path string) bool {
	if path == "" || path == "/" {
		return true
	}
	// Anything under /_app/ that is NOT immutable is metadata
	// (version.json, env.js) and should also be no-cache.
	if len(path) >= 5 && path[:5] == "/_app" {
		return !immutableAssetRE.MatchString(path)
	}
	// Heuristic: paths without a file extension are SPA route names
	// that resolve to index.html.
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return false
		}
	}
	return true
}
