package v1

// Handler test for GET /v1/sys/disk (issue: MCP quality audit found
// the route returned only the root mount, missing the `physical` +
// `mounts` shape its description + MCP system://disk advertise).
//
// Service-layer logic (gopsutil enumeration, smartctl best-effort,
// roundPercent rounding) is covered by service/disks_test.go.
// This test locks the HTTP wire shape so a future refactor of the
// service can't silently revert to the pre-bug single-mount object.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/model"
)

func TestGetSystemDiskInfo_ReturnsPhysicalAndMounts(t *testing.T) {
	// Stub the service call so the handler test never depends on
	// gopsutil or smartctl being present in the CI sandbox.
	prev := getDisksForRoute
	defer func() { getDisksForRoute = prev }()
	getDisksForRoute = func() interface{} {
		return model.DisksInfo{
			Physical: []model.PhysicalDisk{
				{
					Name:         "/dev/sda",
					Model:        "Samsung SSD 870",
					Serial:       "S5RZNF0R000123",
					SizeBytes:    1_000_204_886_016,
					TemperatureC: 41,
					HealthStatus: "PASSED",
				},
			},
			Mounts: []model.MountInfo{
				{
					Path:        "/",
					FSType:      "ext4",
					Total:       999_000_000_000,
					Used:        500_000_000_000,
					Free:        499_000_000_000,
					UsedPercent: 50.1,
				},
			},
		}
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/sys/disk", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := GetSystemDiskInfo(ctx); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	// Decode into a permissive shape: assert the snake-case JSON
	// tags the MCP description pins (physical + mounts at the
	// top of the Data envelope; mount fields path/fs_type/total/
	// used/free/used_percent; physical fields name/model/serial/
	// size_bytes/temperature_c/health_status).
	var resp struct {
		Success int    `json:"success"`
		Message string `json:"message"`
		Data    struct {
			Physical []map[string]interface{} `json:"physical"`
			Mounts   []map[string]interface{} `json:"mounts"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("cannot decode response body: %v\nbody=%s", err, rec.Body.String())
	}
	if resp.Success != common_err.SUCCESS {
		t.Errorf("success: got %d, want %d", resp.Success, common_err.SUCCESS)
	}
	if len(resp.Data.Mounts) != 1 {
		t.Fatalf("mounts len: got %d want 1; body=%s", len(resp.Data.Mounts), rec.Body.String())
	}
	if len(resp.Data.Physical) != 1 {
		t.Fatalf("physical len: got %d want 1; body=%s", len(resp.Data.Physical), rec.Body.String())
	}

	m := resp.Data.Mounts[0]
	for _, key := range []string{"path", "fs_type", "total", "used", "free", "used_percent"} {
		if _, ok := m[key]; !ok {
			t.Errorf("mounts[0] missing key %q (snake_case is the wire contract); got keys=%v", key, mapKeys(m))
		}
	}

	p := resp.Data.Physical[0]
	for _, key := range []string{"name", "model", "serial", "size_bytes", "temperature_c", "health_status"} {
		if _, ok := p[key]; !ok {
			t.Errorf("physical[0] missing key %q (snake_case is the wire contract); got keys=%v", key, mapKeys(p))
		}
	}

	// Lock the rounding contract: 50.1 from the service stays 50.1
	// on the wire (no JSON-encoder drift).
	if got := m["used_percent"]; got != 50.1 {
		t.Errorf("used_percent: got %v want 50.1", got)
	}
}

// TestGetSystemDiskInfo_EmptyPhysicalMarshalsAsArray locks the
// graceful-degrade contract: when smartctl isn't installed the
// route MUST still emit `"physical": []` (not `null`), because
// MCP agents pattern-match on the array length to decide whether
// to surface "no SMART available" to the user vs. enumerate disks.
func TestGetSystemDiskInfo_EmptyPhysicalMarshalsAsArray(t *testing.T) {
	prev := getDisksForRoute
	defer func() { getDisksForRoute = prev }()
	getDisksForRoute = func() interface{} {
		return model.DisksInfo{
			Physical: []model.PhysicalDisk{},
			Mounts: []model.MountInfo{
				{Path: "/", FSType: "ext4", Total: 1, Used: 0, Free: 1, UsedPercent: 0},
			},
		}
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/sys/disk", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := GetSystemDiskInfo(ctx); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	body := rec.Body.String()
	if !contains(body, `"physical":[]`) {
		t.Errorf("expected literal \"physical\":[] in body, got: %s", body)
	}
}

func mapKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
