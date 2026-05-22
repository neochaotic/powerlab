package v2

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/utils/port"
)

// CheckPorts is a lightweight cross-platform port-availability probe used by
// the Custom App Builder to validate published ports as the user types. Unlike
// the Linux-only /proc/net/tcp scan, this attempts a real bind and reports the
// status per port.
//
// GET /v2/app_management/ports/check?ports=8080,9090&proto=tcp
//
// Response:
//
//	{
//	  "data": {
//	    "8080": false,   // false = NOT available (in use)
//	    "9090": true     // true  = available
//	  },
//	  "suggestions": {
//	    "8080": 8081     // first free port at or above the requested one
//	  }
//	}
//
// `proto` defaults to "tcp" when omitted; pass "udp" for UDP probes.
func (a *AppManagement) CheckPorts(ctx echo.Context) error {
	rawPorts := strings.TrimSpace(ctx.QueryParam("ports"))
	if rawPorts == "" {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"message": "missing required query param `ports` (comma-separated list)",
		})
	}

	proto := strings.ToLower(strings.TrimSpace(ctx.QueryParam("proto")))
	if proto != "tcp" && proto != "udp" {
		proto = "tcp"
	}

	availability := map[string]bool{}
	suggestions := map[string]int{}

	for _, raw := range strings.Split(rawPorts, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		p, err := strconv.Atoi(raw)
		if err != nil || p < 1 || p > 65535 {
			availability[raw] = false
			continue
		}
		ok := port.IsPortAvailable(p, proto)
		availability[raw] = ok
		if !ok {
			// Suggest the next available port (scan up to +1000)
			for i := 1; i <= 1000 && (p+i) <= 65535; i++ {
				if port.IsPortAvailable(p+i, proto) {
					suggestions[raw] = p + i
					break
				}
			}
		}
	}

	return ctx.JSON(http.StatusOK, echo.Map{
		"data":        availability,
		"suggestions": suggestions,
	})
}
