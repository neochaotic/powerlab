package route

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
	"github.com/neochaotic/powerlab/backend/gateway/service"
)

type StaticRoute struct {
	state *service.State
}

var startTime = time.Now()

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
	// We use http.ServeFile (not http.FileServer) so we can pick the
	// exact file to serve without triggering FileServer's automatic
	// "/index.html → /" 301 dance.
	wwwRoot := s.state.GetWWWPath()

	e.GET("/*", func(ctx echo.Context) error {
		path := ctx.Request().URL.Path
		req := ctx.Request()
		w := ctx.Response().Writer
		served := serveSPAPath(w, req, wwwRoot, path)
		if !served {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		return nil
	})

	return e
}

// serveSPAPath resolves a request path against an adapter-static
// build directory and writes the response. Returns false ONLY when
// no candidate exists (the caller turns that into 404).
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
func serveSPAPath(w http.ResponseWriter, req *http.Request, wwwRoot, urlPath string) bool {
	clean := strings.TrimPrefix(urlPath, "/")

	if isAssetPath(urlPath) {
		full := filepath.Join(wwwRoot, clean)
		if !pathInsideRoot(wwwRoot, full) {
			return false
		}
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			http.ServeFile(w, req, full)
			return true
		}
		return false
	}

	candidates := []string{
		clean,
		clean + ".html",
		filepath.Join(clean, "index.html"),
		"index.html",
	}
	for _, c := range candidates {
		if c == "" || c == "." {
			c = "index.html"
		}
		full := filepath.Join(wwwRoot, c)
		if !pathInsideRoot(wwwRoot, full) {
			continue
		}
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			http.ServeFile(w, req, full)
			return true
		}
	}
	return false
}

// pathInsideRoot defends against `..`-style traversal in any path the
// caller derived from a URL. We rely on filepath.Rel against the
// non-eval'd absolute paths — symlinks are intentional here (dev maps
// backend/data/www → ui/build) and we want them followed. The check is
// purely textual: target's path must be reachable from root without
// stepping out via "..".
func pathInsideRoot(root, target string) bool {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
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
