package audit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// setupReadDB returns a DB pre-populated with `count` records
// timestamped progressively older (i=0 newest). User IDs cycle 1..3.
func setupReadDB(t *testing.T, count int) *audit.DB {
	t.Helper()
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now()
	for i := 0; i < count; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute)
		r := sampleRecord(ts, int64(1+(i%3)))
		if err := db.Insert(context.Background(), r); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// TestDB_Recent_DescendingByTs — most recent rows first, contract.
func TestDB_Recent_DescendingByTs(t *testing.T) {
	db := setupReadDB(t, 10)
	rows, err := db.Recent(context.Background(), audit.RecentOptions{Limit: 5})
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("len = %d, want 5", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].TsUnixMicros < rows[i].TsUnixMicros {
			t.Errorf("rows[%d].Ts (%d) < rows[%d].Ts (%d) — not descending",
				i-1, rows[i-1].TsUnixMicros, i, rows[i].TsUnixMicros)
		}
	}
}

// TestDB_Recent_LimitDefaultAndCap — Limit=0 → 100 default, Limit
// > 1000 capped at 1000.
func TestDB_Recent_LimitDefaultAndCap(t *testing.T) {
	db := setupReadDB(t, 5)

	// default 100 when not set
	rows, err := db.Recent(context.Background(), audit.RecentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Errorf("default limit returned %d rows (table has 5)", len(rows))
	}

	// over-cap clamped
	db2 := setupReadDB(t, 1500)
	rows, err = db2.Recent(context.Background(), audit.RecentOptions{Limit: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) > 1000 {
		t.Errorf("limit not capped: len = %d, want <= 1000", len(rows))
	}
}

// TestDB_Recent_FilterByUser — UserID filter returns only that user.
func TestDB_Recent_FilterByUser(t *testing.T) {
	db := setupReadDB(t, 30) // users cycle 1..3
	uid := int64(2)
	rows, err := db.Recent(context.Background(), audit.RecentOptions{
		Limit:  100,
		UserID: &uid,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no rows returned with user_id=2")
	}
	for _, r := range rows {
		if r.UserID == nil || *r.UserID != 2 {
			t.Errorf("row %d has UserID=%v, want 2", r.ID, r.UserID)
		}
	}
}

// TestDB_Recent_FilterBySince — only rows with ts > since timestamp.
func TestDB_Recent_FilterBySince(t *testing.T) {
	db := setupReadDB(t, 10)
	// Set since to 5min ago — only first 5 rows (i=0..4) should match.
	since := time.Now().Add(-5 * time.Minute).UnixMicro()
	rows, err := db.Recent(context.Background(), audit.RecentOptions{
		Limit:        100,
		SinceUnixMicros: since,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.TsUnixMicros < since {
			t.Errorf("row id=%d ts=%d < since=%d", r.ID, r.TsUnixMicros, since)
		}
	}
}

// TestDB_Stats — count + oldest + newest + file size all populated.
func TestDB_Stats(t *testing.T) {
	db := setupReadDB(t, 50)
	stats, err := db.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.RowCount != 50 {
		t.Errorf("RowCount = %d, want 50", stats.RowCount)
	}
	if stats.OldestUnixMicros == 0 || stats.NewestUnixMicros == 0 {
		t.Errorf("Oldest/Newest unset: oldest=%d newest=%d", stats.OldestUnixMicros, stats.NewestUnixMicros)
	}
	if stats.OldestUnixMicros >= stats.NewestUnixMicros {
		t.Errorf("Oldest (%d) >= Newest (%d) — invariant broken", stats.OldestUnixMicros, stats.NewestUnixMicros)
	}
	if stats.FileSizeBytes <= 0 {
		t.Errorf("FileSizeBytes = %d, want > 0", stats.FileSizeBytes)
	}
}

// TestDB_Stats_EmptyTable — empty DB returns RowCount=0 and zero
// timestamps without erroring (operator opens a fresh box).
func TestDB_Stats_EmptyTable(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stats, err := db.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats on empty: %v", err)
	}
	if stats.RowCount != 0 {
		t.Errorf("RowCount = %d, want 0", stats.RowCount)
	}
	if stats.OldestUnixMicros != 0 || stats.NewestUnixMicros != 0 {
		t.Errorf("Empty table should have zero timestamps, got %d/%d",
			stats.OldestUnixMicros, stats.NewestUnixMicros)
	}
}

// TestRecentHandler_200_JSON — handler returns 200 with the row list
// wrapped in {data: [...]} so the existing UI Envelope<T> pattern
// works without per-endpoint adapter.
func TestRecentHandler_200_JSON(t *testing.T) {
	db := setupReadDB(t, 5)
	e := echo.New()
	e.GET("/v1/audit/recent", audit.RecentHandler(db))

	req := httptest.NewRequest(http.MethodGet, "/v1/audit/recent?limit=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data []audit.Record `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	if len(resp.Data) != 3 {
		t.Errorf("data len = %d, want 3", len(resp.Data))
	}
}

// TestStatsHandler_200_JSON — same envelope for the stats endpoint.
func TestStatsHandler_200_JSON(t *testing.T) {
	db := setupReadDB(t, 7)
	e := echo.New()
	e.GET("/v1/audit/stats", audit.StatsHandler(db))

	req := httptest.NewRequest(http.MethodGet, "/v1/audit/stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Data audit.StatsResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.RowCount != 7 {
		t.Errorf("RowCount = %d, want 7", resp.Data.RowCount)
	}
}
