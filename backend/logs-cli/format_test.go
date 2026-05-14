package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestFormatTextLine — render an Entry to a single text line for
// the operator's terminal. Severity coloring uses ANSI escape codes
// per ADR-0027, only emitted when the writer is a TTY.
//
// Format: `2026-05-13T20:15:30Z  ERROR  powerlab-core.service  connection refused`
// Coloring (TTY mode):
//   - FATAL  \e[1;31m bold red
//   - ERROR  \e[31m   red
//   - WARN   \e[33m   yellow
//   - INFO   default (no escape)
//   - DEBUG  \e[2m    dim
// Plus a trailing \e[0m reset wherever a non-default color was used.

func makeEntry() Entry {
	return Entry{
		Time:    time.Date(2026, 5, 13, 20, 15, 30, 0, time.UTC),
		Unit:    "powerlab-core.service",
		Level:   LevelError,
		LevelS:  "ERROR",
		Message: "connection refused",
	}
}

func TestFormatTextLine_NoColor(t *testing.T) {
	buf := &bytes.Buffer{}
	writeTextLine(buf, makeEntry(), false)
	got := buf.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("no-color mode should not contain ANSI escape codes, got: %q", got)
	}
	if !strings.Contains(got, "ERROR") {
		t.Errorf("expected ERROR token in output, got: %q", got)
	}
	if !strings.Contains(got, "powerlab-core.service") {
		t.Errorf("expected unit name in output, got: %q", got)
	}
	if !strings.Contains(got, "connection refused") {
		t.Errorf("expected message in output, got: %q", got)
	}
	if !strings.Contains(got, "2026-05-13T20:15:30Z") {
		t.Errorf("expected RFC3339-formatted timestamp in output, got: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("output should end with newline, got: %q", got)
	}
}

func TestFormatTextLine_ErrorColored(t *testing.T) {
	buf := &bytes.Buffer{}
	writeTextLine(buf, makeEntry(), true)
	got := buf.String()
	if !strings.Contains(got, "\x1b[31m") {
		t.Errorf("ERROR in color mode should contain red ANSI, got: %q", got)
	}
	if !strings.Contains(got, "\x1b[0m") {
		t.Errorf("color mode should reset ANSI at end of token, got: %q", got)
	}
}

func TestFormatTextLine_FatalBoldRed(t *testing.T) {
	e := makeEntry()
	e.Level = LevelFatal
	e.LevelS = "FATAL"
	buf := &bytes.Buffer{}
	writeTextLine(buf, e, true)
	got := buf.String()
	if !strings.Contains(got, "\x1b[1;31m") {
		t.Errorf("FATAL should use bold red, got: %q", got)
	}
}

func TestFormatTextLine_WarnYellow(t *testing.T) {
	e := makeEntry()
	e.Level = LevelWarn
	e.LevelS = "WARN"
	buf := &bytes.Buffer{}
	writeTextLine(buf, e, true)
	got := buf.String()
	if !strings.Contains(got, "\x1b[33m") {
		t.Errorf("WARN should use yellow, got: %q", got)
	}
}

func TestFormatTextLine_InfoNoEscape(t *testing.T) {
	e := makeEntry()
	e.Level = LevelInfo
	e.LevelS = "INFO"
	buf := &bytes.Buffer{}
	writeTextLine(buf, e, true)
	got := buf.String()
	// INFO in color mode is intentionally uncolored (default fg).
	// Verify no ANSI sequence wraps the level token specifically.
	if strings.Contains(got, "\x1b[31m") || strings.Contains(got, "\x1b[33m") || strings.Contains(got, "\x1b[1;31m") {
		t.Errorf("INFO should not emit error/warn/fatal ANSI, got: %q", got)
	}
}

func TestFormatTextLine_DebugDim(t *testing.T) {
	e := makeEntry()
	e.Level = LevelDebug
	e.LevelS = "DEBUG"
	buf := &bytes.Buffer{}
	writeTextLine(buf, e, true)
	got := buf.String()
	if !strings.Contains(got, "\x1b[2m") {
		t.Errorf("DEBUG should use dim, got: %q", got)
	}
}

func TestFormatTextLine_ZeroTime(t *testing.T) {
	e := makeEntry()
	e.Time = time.Time{}
	buf := &bytes.Buffer{}
	writeTextLine(buf, e, false)
	got := buf.String()
	if !strings.Contains(got, "(no timestamp)") {
		t.Errorf("zero time should render '(no timestamp)', got: %q", got)
	}
}

// TestFormatJSONLine — when --json is set, every entry renders as
// one compact JSON line (no trailing newline-of-newlines, no ANSI).
// This is the machine-consumable mode for piping through jq.
func TestFormatJSONLine(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := writeJSONLine(buf, makeEntry()); err != nil {
		t.Fatalf("write json: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("JSON mode should never emit ANSI, got: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("JSON line should end with newline, got: %q", got)
	}
	// Round-trip: must be parseable.
	var back Entry
	if err := json.Unmarshal([]byte(strings.TrimRight(got, "\n")), &back); err != nil {
		t.Fatalf("emitted JSON does not parse: %v", err)
	}
	if back.LevelS != "ERROR" {
		t.Errorf("round-trip level: got %q, want ERROR", back.LevelS)
	}
	if back.Message != "connection refused" {
		t.Errorf("round-trip message: got %q", back.Message)
	}
}
