package route

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/IceWhaleTech/CasaOS-Gateway/service"
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
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

type CustomFS struct {
	base fs.FS
}

func NewCustomFS(prefix string) *CustomFS {
	return &CustomFS{
		base: fs.FS(os.DirFS(prefix)),
	}
}

func (c *CustomFS) Open(name string) (fs.File, error) {
	file, err := c.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &CustomFile{
		File: file,
	}, nil
}

func (c *CustomFS) Stat(name string) (fs.FileInfo, error) {
	file, err := c.base.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return &CustomFileInfo{
		FileInfo: info,
	}, nil
}

type CustomFile struct {
	fs.File
}

func (c *CustomFile) Stat() (fs.FileInfo, error) {
	info, err := c.File.Stat()
	if err != nil {
		return nil, err
	}
	return &CustomFileInfo{
		FileInfo: info,
	}, nil
}

func (c *CustomFile) Read(p []byte) (int, error) {
	if seeker, ok := c.File.(io.Reader); ok {
		return seeker.Read(p)
	}
	return 0, fmt.Errorf("file does not implement io.Reader")
}

func (c *CustomFile) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := c.File.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	}
	return 0, fmt.Errorf("file does not implement io.Seeker")
}

type CustomFileInfo struct {
	fs.FileInfo
}

func (c *CustomFileInfo) ModTime() time.Time {
	return startTime
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

	// sovle 304 cache problem by 'If-Modified-Since: Wed, 21 Oct 2015 07:28:00 GMT' from web browser
	e.StaticFS("/", NewCustomFS(s.state.GetWWWPath()))
	return e
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
