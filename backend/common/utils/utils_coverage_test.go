package utils

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestCompareSlices(t *testing.T) {
	cases := []struct {
		name string
		a, b []any
		want bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", []any{}, []any{}, true},
		{"equal", []any{1, "x", true}, []any{1, "x", true}, true},
		{"diff length", []any{1, 2}, []any{1}, false},
		{"diff content", []any{1, 2}, []any{1, 3}, false},
		{"order matters", []any{1, 2}, []any{2, 1}, false},
	}
	for _, c := range cases {
		if got := CompareSlices(c.a, c.b); got != c.want {
			t.Errorf("%s: CompareSlices = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCompareStringSlices(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"equal", []string{"a", "b"}, []string{"a", "b"}, true},
		{"diff length", []string{"a"}, []string{"a", "b"}, false},
		{"diff content", []string{"a", "b"}, []string{"a", "c"}, false},
		{"order matters", []string{"a", "b"}, []string{"b", "a"}, false},
	}
	for _, c := range cases {
		if got := CompareStringSlices(c.a, c.b); got != c.want {
			t.Errorf("%s: CompareStringSlices = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPtr(t *testing.T) {
	p := Ptr(42)
	if p == nil || *p != 42 {
		t.Fatalf("Ptr(42) = %v, want pointer to 42", p)
	}
	s := Ptr("hello")
	if s == nil || *s != "hello" {
		t.Errorf("Ptr(\"hello\") = %v, want pointer to hello", s)
	}
	// Each call returns a distinct pointer.
	if Ptr(1) == Ptr(1) {
		t.Error("Ptr should return a fresh pointer each call")
	}
}

func TestThrottle(t *testing.T) {
	var n int32
	inc := func() { atomic.AddInt32(&n, 1) }

	th := NewThrottle(50 * time.Millisecond)

	// First call runs synchronously.
	th.Do(inc)
	if atomic.LoadInt32(&n) != 1 {
		t.Fatalf("first Do should run immediately; n=%d", atomic.LoadInt32(&n))
	}

	// A burst within the limit must NOT run synchronously (it's deferred to
	// a single trailing call).
	th.Do(inc)
	th.Do(inc)
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Fatalf("calls within the limit should defer, not run now; n=%d", got)
	}

	// After the limit window, the trailing call has fired exactly once.
	time.Sleep(120 * time.Millisecond)
	if got := atomic.LoadInt32(&n); got != 2 {
		t.Errorf("after the window the trailing call should have run once; n=%d (want 2)", got)
	}
}
