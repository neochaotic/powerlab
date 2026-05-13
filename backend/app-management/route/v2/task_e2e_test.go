package v2_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	v2 "github.com/neochaotic/powerlab/backend/app-management/route/v2"
	"github.com/neochaotic/powerlab/backend/app-management/service"
)

// End-to-end test for the install-log SSE pipeline. Builds the full
// stack used in production — Task → channel → echo route → HTTP
// response — and parses the raw HTTP bytes as an EventSource client
// would. The class of bug fixed in v0.6.8 (multi-line content
// emitted as a single `data:` block, which the browser silently
// drops) is caught HERE before it can ship past CI.
//
// What this test would have caught BEFORE v0.6.7 ship:
//   1. Modal stuck on "Preparing…" forever because Phase markers
//      were silently dropped by EventSource — those phase markers
//      arrived inside a single `data:` block.
//   2. Launchpad ghost tile spinner stuck for the same reason.
//
// Sister test: backend/app-management/service/task_test.go covers
// the channel contract (one message = one line). This file covers
// the WIRE FORMAT — the channel contract translated to bytes on
// the wire is what EventSource actually parses.

func setupTaskRoute(t *testing.T) *httptest.Server {
	t.Helper()
	e := echo.New()
	app := &v2.AppManagement{}
	e.GET("/v2/app_management/compose/task/:id/logs", app.GetTaskLogs)
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	return srv
}

func TestTaskSSE_PhaseMarkersAreSeparateEvents(t *testing.T) {
	taskID := "test-phase-markers"
	task := service.MyTaskService.GetOrCreate(taskID)
	t.Cleanup(func() {
		task.Finish()
		service.MyTaskService.Remove(taskID)
	})

	// Pre-populate buffer BEFORE subscribing — mirrors the production
	// race where the install starts (Task.Write fires) before the UI
	// opens its EventSource.
	task.Write([]byte("Starting installation of app: blinko\n"))
	task.Write([]byte("Phase 1/3: Pulling images...\n"))
	task.Write([]byte("Phase 2/3: Creating containers...\n"))
	task.Write([]byte("Phase 3/3: Starting containers...\n"))

	srv := setupTaskRoute(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v2/app_management/compose/task/"+taskID+"/logs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET task logs: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want text/event-stream", got)
	}

	// Parse the response as an EventSource client would.
	events := parseSSEStream(t, resp.Body, 4, 2*time.Second)

	flat := strings.Join(events, "\n")
	for _, p := range []string{"Phase 1/3:", "Phase 2/3:", "Phase 3/3:"} {
		if !strings.Contains(flat, p) {
			t.Errorf("expected %q in SSE event stream — was it silently dropped by an EventSource-style parser? events:\n%s", p, flat)
		}
	}

	// Each phase marker must appear in its OWN event, not co-resident
	// with another phase on the same data: line. (A regression where
	// two phases land in one data: block would still satisfy the
	// substring check above — this guards against it.)
	phaseCount := 0
	for _, ev := range events {
		if !strings.Contains(ev, "Phase ") {
			continue
		}
		phaseCount++
		if strings.Count(ev, "Phase ") > 1 {
			t.Errorf("event collapsed multiple phase markers — v0.1.0→v0.6.7 bug class regression: %q", ev)
		}
	}
	if phaseCount < 3 {
		t.Errorf("expected 3 phase events, got %d (events: %v)", phaseCount, events)
	}
}

// Live-mode after-subscribe behaviour is covered by the unit-side
// tests in service/task_test.go (TestTask_Write_MultilineSplitOnNewlines
// + TestTask_Subscribe_ConcurrentSubscribersGetSameStream): once the
// channel contract guarantees one-line-per-message at the Write side,
// the route handler trivially emits one `data: <line>\n\n` per recv.
// An end-to-end live-streaming test against the real httptest server
// was attempted but suffered from http.Client buffering nuances that
// made it flaky in CI — the cost/benefit did not justify keeping it.
// The catch-up replay test above + the wire-format test below cover
// every byte-on-the-wire path that EventSource would parse.

func TestTaskSSE_NoMultilineInDataBlock(t *testing.T) {
	// Strictest WIRE-FORMAT assertion: a `data:` line in the SSE
	// stream must NEVER be followed by another non-empty line that
	// doesn't start with a SSE field name. That is exactly the
	// pattern EventSource silently drops.
	taskID := "test-wire-format"
	task := service.MyTaskService.GetOrCreate(taskID)
	t.Cleanup(func() {
		task.Finish()
		service.MyTaskService.Remove(taskID)
	})

	task.Write([]byte("Starting installation of app: blinko\n"))
	task.Write([]byte("Phase 1/3: Pulling images...\n"))
	task.Write([]byte("  Digest: sha256:abc\n"))
	task.Write([]byte("Phase 2/3: Creating containers...\n"))

	srv := setupTaskRoute(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v2/app_management/compose/task/"+taskID+"/logs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	// Read raw bytes up to 4 KiB and inspect the wire format directly.
	readCtx, readCancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer readCancel()
	type readResult struct {
		bytes []byte
		err   error
	}
	rawCh := make(chan readResult, 1)
	go func() {
		buf := make([]byte, 4096)
		n, err := resp.Body.Read(buf)
		rawCh <- readResult{buf[:n], err}
	}()
	var raw []byte
	select {
	case r := <-rawCh:
		raw = r.bytes
	case <-readCtx.Done():
		t.Fatal("timed out reading SSE body")
	}

	lines := strings.Split(string(raw), "\n")
	for i := 0; i < len(lines)-1; i++ {
		l := lines[i]
		next := lines[i+1]
		if !strings.HasPrefix(l, "data:") {
			continue
		}
		if next == "" {
			continue // event boundary
		}
		known := strings.HasPrefix(next, "data:") ||
			strings.HasPrefix(next, "event:") ||
			strings.HasPrefix(next, "id:") ||
			strings.HasPrefix(next, "retry:") ||
			strings.HasPrefix(next, ":")
		if !known {
			t.Errorf("SSE wire format violation: data: line followed by non-field line %q — EventSource will drop it. Context:\n%s",
				next,
				strings.Join(lines[max(0, i-1):min(len(lines), i+3)], "\n"))
		}
	}
}

// parseSSEStream reads up to `wantEvents` SSE events from body (or
// until the deadline) and returns them as raw `data:` payloads —
// one per event boundary. Multi-`data:` events are concatenated
// with `\n` per spec; lines without a known field prefix are
// ignored (matching EventSource semantics — the exact behaviour
// that masked the v0.1.0→v0.6.7 bug).
func parseSSEStream(t *testing.T, body io.Reader, wantEvents int, deadline time.Duration) []string {
	t.Helper()
	type readResult struct {
		line string
		err  error
	}
	br := bufio.NewReader(body)
	events := make([]string, 0, wantEvents)
	var current strings.Builder
	endAt := time.Now().Add(deadline)

	readLine := func() (string, error) {
		ch := make(chan readResult, 1)
		go func() {
			s, e := br.ReadString('\n')
			ch <- readResult{s, e}
		}()
		select {
		case r := <-ch:
			return r.line, r.err
		case <-time.After(time.Until(endAt)):
			return "", io.EOF
		}
	}

	for len(events) < wantEvents {
		line, err := readLine()
		if err != nil {
			if current.Len() > 0 {
				events = append(events, current.String())
			}
			return events
		}
		trimmed := strings.TrimRight(line, "\n\r")
		if trimmed == "" {
			if current.Len() > 0 {
				events = append(events, current.String())
				current.Reset()
			}
			continue
		}
		if strings.HasPrefix(trimmed, "data:") {
			data := strings.TrimPrefix(trimmed, "data:")
			data = strings.TrimPrefix(data, " ") // optional space per SSE spec
			if current.Len() > 0 {
				current.WriteByte('\n')
			}
			current.WriteString(data)
		}
	}
	return events
}
