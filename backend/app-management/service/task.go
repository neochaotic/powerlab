package service

import (
	"bytes"
	"strings"
	"sync"
)

// Task is the per-install bookkeeping that lets the SSE route
// handler stream `docker compose up`-style log output to the UI.
// Writers (compose_app_lifecycle.go's PullAndInstall) push bytes
// in; subscribers (route/v2/task.go's GetTaskLogs) pull lines out
// via channels.
//
// **SSE contract (v0.6.8 fix, regression-tested in task_test.go):**
// every channel message contains EXACTLY ONE line of text with no
// embedded newline. The SSE route handler can then safely emit
// `data: <line>\n\n` per channel recv — the HTML5 EventSource
// spec drops any line inside a `data:` block that does not start
// with a known field name, so multi-line content used to silently
// lose Phase markers in the browser.
type Task struct {
	ID          string
	LogBuffer   *bytes.Buffer
	mu          sync.Mutex
	subscribers []chan string
	isFinished  bool
}

type TaskService struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

func NewTaskService() *TaskService {
	return &TaskService{
		tasks: make(map[string]*Task),
	}
}

func (s *TaskService) GetOrCreate(id string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, ok := s.tasks[id]; ok {
		return task
	}

	task := &Task{
		ID:          id,
		LogBuffer:   new(bytes.Buffer),
		subscribers: make([]chan string, 0),
	}
	s.tasks[id] = task
	return task
}

func (s *TaskService) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, id)
}

// Write fans the just-written bytes into per-line messages on every
// active subscriber's channel. The full raw bytes still go into the
// LogBuffer so a late subscriber can catch up.
//
// Splitting rules — same on Write + Subscribe replay:
//   - Empty lines are skipped (compose pull emits blank progress
//     ticks; they would just churn the EventSource pipe).
//   - Trailing partial line (no `\n`) is still emitted. The install
//     log stream from compose CAN end without a final newline; the
//     last meaningful line must reach the UI regardless.
//
// Backpressure is non-blocking: subscribers with a full channel
// drop the message rather than make the install slow down. The
// install still finishes; the UI just sees fewer log lines if it
// is reading too slowly.
func (t *Task) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	n, err = t.LogBuffer.Write(p)
	if n == 0 {
		return n, err
	}

	for _, line := range splitLines(string(p)) {
		t.broadcastLocked(line)
	}

	return n, err
}

// Subscribe attaches a new listener to this Task's log stream. The
// returned channel emits one message per line. On attach, the
// current contents of LogBuffer are replayed as separate lines so
// the listener catches up on the install's history.
//
// The cleanup function MUST be deferred — without it the
// subscriber's channel stays attached and the Task leaks per
// install.
func (t *Task) Subscribe() (chan string, func()) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := make(chan string, 100)

	// Replay the current buffer line-by-line so the catch-up matches
	// the live-stream contract (one channel message = one line).
	if t.LogBuffer.Len() > 0 {
		for _, line := range splitLines(t.LogBuffer.String()) {
			// Buffered channel size 100 is sized to absorb a typical
			// install's worth of pre-subscribe history; if the catch-up
			// is bigger than that the listener is going to lose some
			// initial lines, but the install itself keeps progressing
			// (matches backpressure behaviour in Write).
			select {
			case ch <- line:
			default:
				goto attach
			}
		}
	}

attach:
	// If the task is already done, close the channel after the replay
	// so the route handler can detect end-of-stream and emit
	// `event: end`. Without this, a Subscribe that races AFTER Finish
	// (typical of a fast install + slow page-load) would leave the
	// handler blocked on `<-ch` forever — UI never gets `event: end`,
	// never calls checkInstallResult, modal stays in "Preparing".
	// Regression: see task_test.go TestTask_Subscribe_AfterFinish_*.
	if t.isFinished {
		close(ch)
		return ch, func() {}
	}

	t.subscribers = append(t.subscribers, ch)

	cleanup := func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		for i, sub := range t.subscribers {
			if sub == ch {
				t.subscribers = append(t.subscribers[:i], t.subscribers[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, cleanup
}

// Finish closes every subscriber's channel so they can drain the
// remaining messages and detect end-of-stream via the standard
// `_, ok := <-ch` idiom.
func (t *Task) Finish() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.isFinished = true
	for _, sub := range t.subscribers {
		close(sub)
	}
	t.subscribers = nil
}

// broadcastLocked must be called with t.mu held. It pushes one
// line to every subscriber, dropping silently for any subscriber
// whose channel is full (backpressure — the install always wins
// vs. a slow reader).
func (t *Task) broadcastLocked(line string) {
	for _, sub := range t.subscribers {
		select {
		case sub <- line:
		default:
			// Slow subscriber — skip.
		}
	}
}

// splitLines returns the non-empty lines from s. Both `\n` and the
// trailing partial line (if any) are honoured: `"a\nb"` yields
// `["a", "b"]`. `"a\n\nb\n"` yields `["a", "b"]` (empty lines
// suppressed).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

var MyTaskService = NewTaskService()
