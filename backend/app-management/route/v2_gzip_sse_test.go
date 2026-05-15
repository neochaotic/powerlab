package route_test

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/app-management/route"
	"github.com/neochaotic/powerlab/backend/app-management/service"
)

// SSE responses must NEVER be gzip-compressed by the router. Echo's
// Gzip() middleware buffers chunks into a deflate stream before
// emitting them, which destroys the per-line live-streaming the
// browser EventSource expects — the modal stays blank for tens of
// seconds, then dumps everything at once when the gzip block flushes.
//
// User-reported regression: install modal "fica travado na tela de
// progresso" — image-pull SSE events appeared in batches because the
// browser sent the default Accept-Encoding: gzip and the router
// gladly buffered the stream. curl WITHOUT Accept-Encoding flushed
// per-line; WITH it returned a 1.4 KB gzip blob.
//
// Sister fix to PR #384 (ResponseWriter Flusher forwarding in audit
// middleware). Both are needed for end-to-end SSE flow.
func TestV2Router_SSEResponseIsNotGzipped(t *testing.T) {
	taskID := "test-gzip-skipper"
	task := service.MyTaskService.GetOrCreate(taskID)
	t.Cleanup(func() {
		task.Finish()
		service.MyTaskService.Remove(taskID)
	})
	task.Write([]byte("Phase 1/3: Pulling images...\n"))
	task.Write([]byte("Phase 2/3: Creating containers...\n"))
	task.Write([]byte("Phase 3/3: Starting containers...\n"))

	srv := httptest.NewServer(route.InitV2Router())
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		srv.URL+"/v2/app_management/compose/task/"+taskID+"/logs", nil)
	// Default browser behaviour — EventSource always advertises gzip.
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	// Use a transport that does NOT auto-decode so we can observe the
	// raw Content-Encoding header. http.DefaultTransport silently
	// strips the gzip layer if it set Accept-Encoding itself.
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET task logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Errorf("Content-Type: got %q, want text/event-stream", got)
	}
	if got := resp.Header.Get("Content-Encoding"); strings.Contains(strings.ToLower(got), "gzip") {
		// Sanity prove the body is actually gzip-magic so the test
		// failure narrative is unambiguous.
		readCtx, readCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer readCancel()
		head := readWithDeadline(t, resp.Body, 64, readCtx)
		if len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b {
			t.Errorf("SSE response is gzip-compressed (Content-Encoding=%q, magic=0x1f8b). EventSource buffers indefinitely. Add a GzipWithConfig{Skipper} that bypasses text/event-stream paths.", got)
		} else {
			t.Errorf("Content-Encoding=%q on SSE response — must not gzip SSE.", got)
		}
		return
	}

	// Read a small slice of the body and confirm it's raw SSE bytes
	// (data: ... \n\n), not the gzip magic header.
	readCtx, readCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer readCancel()
	head := readWithDeadline(t, resp.Body, 128, readCtx)
	if len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b {
		t.Errorf("body starts with gzip magic 0x1f8b but Content-Encoding header was absent — broken gzip framing. body[:32]=% x", head[:min(32, len(head))])
	}
	if !strings.Contains(string(head), "data:") {
		t.Errorf("expected raw SSE 'data:' prefix in body, got: %q", string(head))
	}
}

// Belt-and-suspenders: if anyone removes the skipper, this test
// proves the failure mode by decoding the gzip blob and showing
// the same SSE payload — i.e. the bytes ARE there, just buffered
// behind a gzip stream. Catches the case where someone "fixes" the
// outer test by stripping Accept-Encoding instead of skipping gzip.
func TestV2Router_SSEDataReachesClientUnbuffered(t *testing.T) {
	taskID := "test-gzip-skipper-unbuffered"
	task := service.MyTaskService.GetOrCreate(taskID)
	t.Cleanup(func() {
		task.Finish()
		service.MyTaskService.Remove(taskID)
	})
	task.Write([]byte("Phase 1/3: Pulling images...\n"))

	srv := httptest.NewServer(route.InitV2Router())
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		srv.URL+"/v2/app_management/compose/task/"+taskID+"/logs", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	readCtx, readCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer readCancel()
	body := readWithDeadline(t, resp.Body, 4096, readCtx)

	var payload string
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		// Decode for diagnostic clarity — confirms the bytes are real,
		// just gzip-framed.
		gz, err := gzip.NewReader(strings.NewReader(string(body)))
		if err == nil {
			defer gz.Close()
			b, _ := io.ReadAll(gz)
			payload = string(b)
		}
	} else {
		payload = string(body)
	}
	if !strings.Contains(payload, "Phase 1/3") {
		t.Errorf("payload missing Phase 1/3 marker: %q", payload)
	}
}

// readWithDeadline reads up to n bytes from r, returning what arrived
// before ctx fires. The SSE handler keeps the connection open forever
// (waiting for new task lines), so a plain io.ReadAll would hang.
func readWithDeadline(t *testing.T, r io.Reader, n int, ctx context.Context) []byte {
	t.Helper()
	type res struct {
		b   []byte
		err error
	}
	ch := make(chan res, 1)
	go func() {
		buf := make([]byte, n)
		count, err := r.Read(buf)
		ch <- res{buf[:count], err}
	}()
	select {
	case r := <-ch:
		return r.b
	case <-ctx.Done():
		return nil
	}
}
