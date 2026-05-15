package audit

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// envelope mirrors the existing model.Result shape used by other
// PowerLab endpoints (data wrapper). Kept local here so we don't
// import model.Result and create a circular package edge.
type envelope[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message,omitempty"`
}

// parseRecentOptions extracts limit / user_id / since from an URL
// query — shared between the Echo and stdlib handlers.
func parseRecentOptions(get func(string) string) RecentOptions {
	opts := RecentOptions{}
	if l := get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			opts.Limit = n
		}
	}
	if u := get("user_id"); u != "" {
		if uid, err := strconv.ParseInt(u, 10, 64); err == nil {
			opts.UserID = &uid
		}
	}
	if s := get("since"); s != "" {
		if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
			opts.SinceUnixMicros = ts
		}
	}
	return opts
}

// RecentHandler binds store to an echo.HandlerFunc serving
// GET /v1/audit/recent. Reads limit / user_id / since query params
// and clamps them via RecentOptions.
func RecentHandler(store *Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		opts := parseRecentOptions(c.QueryParam)
		rows := store.Recent(opts)
		return c.JSON(http.StatusOK, envelope[[]Record]{Data: rows})
	}
}

// StatsHandler binds store to an echo.HandlerFunc serving
// GET /v1/audit/stats.
func StatsHandler(store *Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		s, err := store.Stats(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, envelope[Stats]{
				Data:    Stats{},
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusOK, envelope[Stats]{Data: s})
	}
}
