package main

import (
	"strings"
	"testing"
	"time"
)

// TestParseJournalEntry — TDD foundation for the journal subcommand.
// `journalctl -o json` outputs one JSON object per line; we want a
// normalized Entry struct that surfaces:
//   - Unit (e.g. "powerlab-core.service")
//   - Time (wall-clock from __REALTIME_TIMESTAMP, which is unix
//     microseconds as a string)
//   - Level (mapped from PRIORITY syslog levels, with slog override
//     when MESSAGE is itself JSON with a "level" key)
//   - Message (the human-readable line — innermost slog "msg" when
//     applicable, raw MESSAGE otherwise)
//
// Coverage matrix:
//   1. plain text MESSAGE — PRIORITY drives the level
//   2. slog-structured MESSAGE — inner level + msg take precedence
//   3. malformed timestamp — falls back to zero time but does not panic
//   4. unknown PRIORITY (e.g. "99") — defaults to INFO
//   5. missing MESSAGE — empty message, no panic
//   6. PRIORITY missing — defaults to INFO

func TestParseJournalEntry_PlainText(t *testing.T) {
	line := `{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"3","MESSAGE":"connection refused"}`
	e, err := parseJournalEntry([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Unit != "powerlab-core.service" {
		t.Errorf("unit: got %q, want powerlab-core.service", e.Unit)
	}
	want := time.UnixMicro(1715619600000000).UTC()
	if !e.Time.Equal(want) {
		t.Errorf("time: got %v, want %v", e.Time, want)
	}
	if e.Level != LevelError {
		t.Errorf("level: got %v, want ERROR", e.Level)
	}
	if e.Message != "connection refused" {
		t.Errorf("message: got %q, want 'connection refused'", e.Message)
	}
}

func TestParseJournalEntry_SlogStructured(t *testing.T) {
	inner := `{"level":"WARN","msg":"slow query","duration_ms":1234}`
	line := `{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"6","MESSAGE":` + jsonEncode(inner) + `}`
	e, err := parseJournalEntry([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// PRIORITY=6 (info) is overridden by the inner slog WARN.
	if e.Level != LevelWarn {
		t.Errorf("level: got %v, want WARN (inner slog overrides PRIORITY)", e.Level)
	}
	if e.Message != "slow query" {
		t.Errorf("message: got %q, want 'slow query' (inner slog msg)", e.Message)
	}
}

func TestParseJournalEntry_MalformedTimestamp_NoPanic(t *testing.T) {
	line := `{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"not-a-number","PRIORITY":"6","MESSAGE":"hello"}`
	e, err := parseJournalEntry([]byte(line))
	if err != nil {
		t.Fatalf("malformed ts should not surface an error: %v", err)
	}
	if !e.Time.IsZero() {
		t.Errorf("expected zero time for malformed ts, got %v", e.Time)
	}
	if e.Message != "hello" {
		t.Errorf("message: got %q, want 'hello'", e.Message)
	}
}

func TestParseJournalEntry_UnknownPriority_DefaultsInfo(t *testing.T) {
	line := `{"_SYSTEMD_UNIT":"x.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"99","MESSAGE":"weird"}`
	e, err := parseJournalEntry([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Level != LevelInfo {
		t.Errorf("unknown PRIORITY should default to INFO, got %v", e.Level)
	}
}

func TestParseJournalEntry_MissingMessage(t *testing.T) {
	line := `{"_SYSTEMD_UNIT":"x.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"6"}`
	e, err := parseJournalEntry([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Message != "" {
		t.Errorf("missing MESSAGE should yield empty string, got %q", e.Message)
	}
}

func TestParseJournalEntry_MissingPriority_DefaultsInfo(t *testing.T) {
	line := `{"_SYSTEMD_UNIT":"x.service","__REALTIME_TIMESTAMP":"1715619600000000","MESSAGE":"no priority"}`
	e, err := parseJournalEntry([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Level != LevelInfo {
		t.Errorf("missing PRIORITY should default to INFO, got %v", e.Level)
	}
}

func TestParseJournalEntry_PrioritySyslogMapping(t *testing.T) {
	cases := []struct {
		priority string
		want     Level
	}{
		{"0", LevelFatal},
		{"1", LevelFatal},
		{"2", LevelFatal},
		{"3", LevelError},
		{"4", LevelWarn},
		{"5", LevelInfo},
		{"6", LevelInfo},
		{"7", LevelDebug},
	}
	for _, c := range cases {
		line := `{"_SYSTEMD_UNIT":"x.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"` + c.priority + `","MESSAGE":"m"}`
		e, err := parseJournalEntry([]byte(line))
		if err != nil {
			t.Fatalf("PRIORITY=%q: %v", c.priority, err)
		}
		if e.Level != c.want {
			t.Errorf("PRIORITY=%q: got level %v, want %v", c.priority, e.Level, c.want)
		}
	}
}

func TestParseJournalEntry_MalformedJSON_ReturnsError(t *testing.T) {
	_, err := parseJournalEntry([]byte("not json"))
	if err == nil {
		t.Fatal("expected error on non-JSON input")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected error to mention 'parse', got %q", err.Error())
	}
}

// jsonEncode quotes + escapes a string for embedding inside a JSON
// document. Used by the slog-nested test to keep the fixture literal
// readable. (No third-party fixture library to avoid pulling deps
// into a CLI binary.)
func jsonEncode(s string) string {
	b := strings.Builder{}
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
