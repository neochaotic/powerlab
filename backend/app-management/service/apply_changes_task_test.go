package service

import (
	"context"
	"testing"
	"time"
)

// Locks the edit-app progress contract: runApplyChanges must finish the
// per-app Task so the UI's task/{id}/logs SSE stream gets its terminal
// `event: end`. Before the fix the update path never touched the Task,
// so the progress modal spun forever even though the apply succeeded.
//
// We assert Finish() is called both when the apply succeeds and when it
// fails — the modal must resolve either way. The docker work is stubbed
// via applyChangesRunner so the test needs no daemon.
func TestRunApplyChanges_FinishesTaskForUI(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runnErr error
	}{
		{"apply succeeds", nil},
		{"apply fails", context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const appName = "redeploy-app"

			orig := applyChangesRunner
			applyChangesRunner = func(_ *ComposeApp, _ context.Context, _ []byte) error {
				return tc.runnErr
			}
			defer func() { applyChangesRunner = orig }()
			defer MyTaskService.Remove(appName)

			// Subscribe BEFORE running so we observe the channel close
			// (the `event: end` the SSE handler relays to the modal).
			task := MyTaskService.GetOrCreate(appName)
			ch, cleanup := task.Subscribe()
			defer cleanup()

			app := &ComposeApp{Name: appName}
			done := make(chan struct{})
			go func() {
				app.runApplyChanges(context.Background(), []byte("name: "+appName))
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("runApplyChanges did not return")
			}

			// Drain to the close. A finished task closes subscriber
			// channels; if Finish were missing this would block.
			select {
			case _, ok := <-drainToClose(ch):
				if ok {
					t.Fatal("expected the task channel to be closed (event: end)")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("task channel never closed — the UI progress modal would spin forever")
			}
		})
	}
}

// drainToClose reads (and discards) any buffered lines and returns a
// channel that yields the zero value once the input is closed.
func drainToClose(ch chan string) <-chan string {
	out := make(chan string)
	go func() {
		for range ch { //nolint:revive // intentional drain
		}
		close(out)
	}()
	return out
}
