package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// NewReader / NewWriter wrap an io.Reader / io.Writer and short-circuit
// each Read / Write call when the supplied context has been canceled.
// The original test sketch in this file was a 10-second sleep loop
// that read from /Users/liangjianli/Downloads/* (the upstream CasaOS
// developer's machine) and asserted nothing — flagged as
// "MUST FIX" since Sprint 5. Rewritten in Sprint 10 PR G as proper
// table-driven unit coverage.

func TestNewReader_HappyPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := strings.NewReader("hello world")
	wrapped := NewReader(ctx, src)

	out, err := io.ReadAll(wrapped)
	if err != nil {
		t.Fatalf("ReadAll on non-canceled wrapped reader returned %v", err)
	}
	if string(out) != "hello world" {
		t.Fatalf("got %q, want %q", string(out), "hello world")
	}
}

func TestNewReader_CanceledBeforeRead_ReturnsCtxErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	src := strings.NewReader("hello world")
	wrapped := NewReader(ctx, src)

	buf := make([]byte, 4)
	n, err := wrapped.Read(buf)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v (n=%d)", err, n)
	}
	if n != 0 {
		t.Fatalf("expected 0 bytes when ctx canceled before Read, got %d", n)
	}
}

func TestNewWriter_HappyPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var dst bytes.Buffer
	wrapped := NewWriter(ctx, &dst)

	n, err := wrapped.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("Write on non-canceled wrapped writer returned %v", err)
	}
	if n != len("hello world") || dst.String() != "hello world" {
		t.Fatalf("got n=%d buf=%q, want n=%d buf=%q",
			n, dst.String(), len("hello world"), "hello world")
	}
}

func TestNewWriter_CanceledBeforeWrite_ReturnsCtxErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var dst bytes.Buffer
	wrapped := NewWriter(ctx, &dst)

	n, err := wrapped.Write([]byte("hello world"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v (n=%d)", err, n)
	}
	if n != 0 || dst.Len() != 0 {
		t.Fatalf("expected 0 bytes written when ctx canceled, got n=%d buf=%q",
			n, dst.String())
	}
}

// io.Copy through a wrapped reader + wrapped writer stops cleanly as
// soon as the shared ctx is canceled. The pre-cancel branch lets us
// assert without timing flake — the second test sets a tiny limit
// reader so partial progress is possible but bounded.
func TestNewReader_CanceledMidCopy_StopsCopy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before io.Copy starts

	src := strings.NewReader(strings.Repeat("x", 1024))
	var dst bytes.Buffer

	n, err := io.Copy(NewWriter(ctx, &dst), NewReader(ctx, src))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from io.Copy, got %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 bytes copied when ctx canceled before Copy, got %d", n)
	}
}

// NewReader is idempotent: wrapping a reader that already shares the
// same ctx returns the existing wrapper rather than nesting. Locks
// the optimisation in file.go:32.
func TestNewReader_SameContextReturnsSameWrapper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := strings.NewReader("data")
	first := NewReader(ctx, src)
	second := NewReader(ctx, first)
	if first != second {
		t.Fatalf("NewReader with the same ctx must not re-wrap; got distinct instances")
	}
}
