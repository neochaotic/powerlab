package service

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// Regression tests for the v0.6.7 "Preparing modal stuck forever"
// bug. Root cause: Task.Subscribe() and Task.Write() were emitting
// multi-line content as a single channel message, which the SSE
// route handler then wrapped in `data: <multi-line>\n\n`. The
// HTML5 EventSource spec says any line inside `data: ... \n\n`
// that does not start with a known field name is dropped — so the
// browser only saw the FIRST line and silently lost every
// `Phase 1/3:`, `Phase 2/3:`, `Phase 3/3:` marker. The UI's
// InstallProgressBar (#329, v0.6.6) gates the determinate progress
// bar on `currentPhase` being parsed from those markers; without
// them it sat on the indeterminate "Preparing…" forever.
//
// The contract these tests lock: each channel message contains
// EXACTLY ONE line of text (no embedded `\n`). The SSE route
// handler can then safely emit `data: <line>\n\n` per channel
// recv.

func recvAll(ch <-chan string, max int, timeout time.Duration) []string {
	out := make([]string, 0, max)
	deadline := time.After(timeout)
	for len(out) < max {
		select {
		case s, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, s)
		case <-deadline:
			return out
		}
	}
	return out
}

func TestTask_Subscribe_BufferSplitOnNewlines(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-1")

	// Simulate an install that wrote multiple Phase markers BEFORE
	// the UI subscribed. The catch-up replay must NOT collapse
	// these into a single channel message.
	task.Write([]byte("Starting installation of app: blinko\n"))
	task.Write([]byte("Phase 1/3: Pulling images...\n"))
	task.Write([]byte("Phase 2/3: Creating containers...\n"))

	ch, cleanup := task.Subscribe()
	defer cleanup()

	got := recvAll(ch, 5, 200*time.Millisecond)

	if len(got) != 3 {
		t.Fatalf("expected 3 messages on catch-up replay, got %d: %#v", len(got), got)
	}
	for i, s := range got {
		if strings.Contains(s, "\n") {
			t.Errorf("message[%d] contains embedded newline (breaks SSE protocol): %q", i, s)
		}
	}
	if !strings.Contains(got[1], "Phase 1/3") {
		t.Errorf("expected Phase 1/3 marker in message[1], got: %q", got[1])
	}
}

func TestTask_Write_MultilineSplitOnNewlines(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-2")

	ch, cleanup := task.Subscribe()
	defer cleanup()

	// Single Write with embedded \n — happens e.g. when docker pull
	// emits multi-line status in one buffer flush. Must arrive as
	// multiple channel messages, one per line.
	task.Write([]byte("Phase 1/3: Pulling images...\nPulling postgres...\nDigest: sha256:abc\n"))

	got := recvAll(ch, 5, 200*time.Millisecond)
	if len(got) != 3 {
		t.Fatalf("expected 3 split messages, got %d: %#v", len(got), got)
	}
	for i, s := range got {
		if strings.Contains(s, "\n") {
			t.Errorf("message[%d] contains embedded newline: %q", i, s)
		}
	}
}

func TestTask_Write_SingleLineUntouched(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-3")
	ch, cleanup := task.Subscribe()
	defer cleanup()

	task.Write([]byte("hello\n"))

	got := recvAll(ch, 2, 200*time.Millisecond)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d: %#v", len(got), got)
	}
	if got[0] != "hello" {
		t.Errorf("trailing \\n should be stripped; got %q", got[0])
	}
}

func TestTask_Write_NoTrailingNewline(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-4")
	ch, cleanup := task.Subscribe()
	defer cleanup()

	// Some upstream writers (notably docker pull progress on its
	// final flush) do not terminate with \n. The final partial line
	// must still arrive as a message.
	task.Write([]byte("line A\nline B (no trailing nl)"))

	got := recvAll(ch, 3, 200*time.Millisecond)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d: %#v", len(got), got)
	}
	if got[1] != "line B (no trailing nl)" {
		t.Errorf("partial final line: got %q, want 'line B (no trailing nl)'", got[1])
	}
}

func TestTask_Write_EmptyLineSkipped(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-5")
	ch, cleanup := task.Subscribe()
	defer cleanup()

	// Consecutive newlines should not flood the channel with empty
	// strings — visual noise + SSE pings against the connection.
	task.Write([]byte("line A\n\n\nline B\n"))

	got := recvAll(ch, 5, 200*time.Millisecond)
	if len(got) != 2 {
		t.Fatalf("expected 2 non-empty messages, got %d: %#v", len(got), got)
	}
}

func TestTask_Subscribe_BufferWithoutTrailingNewline(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-6")

	// Write content WITHOUT trailing \n, then subscribe — the
	// catch-up replay must include the partial final line.
	task.Write([]byte("Phase 1/3: Pulling...\nPhase 2/3: Creating..."))

	ch, cleanup := task.Subscribe()
	defer cleanup()

	got := recvAll(ch, 3, 200*time.Millisecond)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages from catch-up, got %d: %#v", len(got), got)
	}
}

func TestTask_Finish_ClosesAllSubscribers(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-7")
	ch, cleanup := task.Subscribe()
	defer cleanup()

	task.Write([]byte("one\n"))
	task.Finish()

	// Drain — channel must close, signalling task completion.
	drained := 0
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				if drained < 1 {
					t.Errorf("expected to drain at least 1 message before close, drained %d", drained)
				}
				return
			}
			drained++
		case <-deadline:
			t.Fatal("channel did not close after Finish()")
		}
	}
}

func TestTask_Subscribe_BackpressureDoesNotDeadlock(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-8")
	ch, cleanup := task.Subscribe()
	defer cleanup()

	// Don't drain — flood with more writes than the channel buffer.
	// A slow subscriber should be allowed to fall behind without
	// blocking Write().
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			task.Write([]byte("flood\n"))
		}
	}()

	select {
	case <-done:
		// Drain whatever made it.
		_ = recvAll(ch, 500, 50*time.Millisecond)
	case <-time.After(2 * time.Second):
		t.Fatal("Write() blocked on full channel — backpressure regression")
	}
}

func TestTask_Subscribe_AfterFinish_ChannelClosesAfterReplay(t *testing.T) {
	// REGRESSION for v0.6.7 user report: "fica preparando, bar
	// carrega, mas modal nunca termina". Root cause: when the
	// install completed BEFORE the UI subscribed (race typical
	// of a fast install or a slow page-load), Subscribe replayed
	// the buffer to the new subscriber's channel but NEVER closed
	// the channel — Finish had already run and would not run
	// again. The route handler then drained the replay and
	// blocked on `<-ch` forever, never emitting `event: end`,
	// so the UI never called checkInstallResult and the modal
	// stayed in 'starting' state showing "Preparing…".
	//
	// Contract this test locks: if a Subscribe happens after
	// Finish, the channel must close once the replay is drained.
	svc := NewTaskService()
	task := svc.GetOrCreate("test-after-finish")

	// Simulate a quick install that completes before the UI hits
	// the SSE endpoint.
	task.Write([]byte("Phase 1/3: Pulling...\n"))
	task.Write([]byte("Phase 2/3: Creating...\n"))
	task.Write([]byte("Phase 3/3: Starting...\n"))
	task.Write([]byte("Installation completed successfully!\n"))
	task.Finish()

	ch, cleanup := task.Subscribe()
	defer cleanup()

	got := recvAll(ch, 10, 500*time.Millisecond)

	// Should have received the replay AND the channel should now be
	// closed so the route handler can emit event:end.
	if len(got) < 4 {
		t.Errorf("expected ≥4 replay messages, got %d", len(got))
	}

	// Test channel-close detection: read one more — must get !ok.
	deadline := time.After(500 * time.Millisecond)
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel still open after replay drained — route handler will hang forever")
		}
	case <-deadline:
		t.Fatal("channel still has buffered messages or never closed; route handler would block")
	}
}

func TestTask_Subscribe_ConcurrentSubscribersGetSameStream(t *testing.T) {
	svc := NewTaskService()
	task := svc.GetOrCreate("test-9")

	ch1, c1 := task.Subscribe()
	defer c1()
	ch2, c2 := task.Subscribe()
	defer c2()

	var wg sync.WaitGroup
	wg.Add(2)
	got1, got2 := []string{}, []string{}
	go func() {
		defer wg.Done()
		got1 = recvAll(ch1, 3, 300*time.Millisecond)
	}()
	go func() {
		defer wg.Done()
		got2 = recvAll(ch2, 3, 300*time.Millisecond)
	}()

	task.Write([]byte("a\nb\nc\n"))
	wg.Wait()

	if len(got1) != 3 || len(got2) != 3 {
		t.Errorf("both subs should get all 3 lines; got1=%d got2=%d", len(got1), len(got2))
	}
}
