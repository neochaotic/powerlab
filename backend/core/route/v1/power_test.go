package v1

// Handler tests for the power-action routes (#260).
//
// Service-layer logic (whitelist enforcement, systemctl arg construction,
// gateway cgroup escape) is tested exhaustively in service/power_actions_test.go.
// These tests lock the HTTP contract: status code, response shape, field count.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/service"
)

func TestGetPowerLabServicesPreflight_Returns200WithAllServices(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/sys/services/preflight", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := GetPowerLabServicesPreflight(ctx); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	var result model.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("cannot decode response body: %v", err)
	}
	if result.Success != common_err.SUCCESS {
		t.Errorf("success field: got %d, want %d (SUCCESS)", result.Success, common_err.SUCCESS)
	}

	// data is []ServiceEnabledState; re-marshal to assert shape.
	dataBytes, _ := json.Marshal(result.Data)
	var states []service.ServiceEnabledState
	if err := json.Unmarshal(dataBytes, &states); err != nil {
		t.Fatalf("data is not []ServiceEnabledState: %v", err)
	}
	if len(states) != len(service.PowerLabServices) {
		t.Errorf("expected %d entries (one per PowerLab unit), got %d",
			len(service.PowerLabServices), len(states))
	}
	for _, s := range states {
		if s.Name == "" {
			t.Errorf("entry has empty name: %+v", s)
		}
	}
}
