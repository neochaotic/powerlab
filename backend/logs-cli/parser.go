package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Level is the severity bucket the CLI + UI use to color and filter
// log entries. Comes from one of two places: the slog "level" key
// when the source line is a structured slog record (PowerLab
// services), or the systemd PRIORITY field otherwise (which maps to
// the syslog level integer).
type Level int

const (
	LevelInfo Level = iota
	LevelDebug
	LevelWarn
	LevelError
	LevelFatal
)

// String renders the level as its uppercase tag. Used by the text
// formatter + the --json output. ANSI coloring lives in
// format.go (TTY-aware).
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "INFO"
	}
}

// Entry is the normalized log record produced by parseJournalEntry
// and consumed by the formatters. Stable across log sources —
// docker_logs.go and install_logs.go produce the same shape so the
// CLI's --json output is schema-clean regardless of which source
// fed it.
type Entry struct {
	Time    time.Time `json:"time"`
	Unit    string    `json:"unit"`
	Level   Level     `json:"-"`
	LevelS  string    `json:"level"`
	Message string    `json:"message"`
}

// journalRecord mirrors only the journalctl -o json fields we
// actually read. Anything else in the record is preserved by the
// systemd journal itself; we do not need to surface it.
type journalRecord struct {
	Unit      string `json:"_SYSTEMD_UNIT"`
	Timestamp string `json:"__REALTIME_TIMESTAMP"` // unix microseconds as a string
	Priority  string `json:"PRIORITY"`
	Message   string `json:"MESSAGE"`
}

// slogRecord captures the inner JSON when the PowerLab services
// (which all use pkg/logging on top of stdlib slog — see ADR-0026)
// emit structured records. The level key is upper-case ("INFO",
// "WARN", "ERROR") per slog convention. We only read level + msg;
// extra attribute fields are preserved when --json output mode is
// used.
type slogRecord struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// parseJournalEntry takes one line of `journalctl -o json` output
// and returns the normalized Entry. The function is intentionally
// forgiving: malformed timestamps fall back to zero time, unknown
// PRIORITY values fall back to INFO, and slog-shaped MESSAGE
// content overrides the outer level/message pair.
//
// The only hard error is JSON parse failure on the OUTER record.
// (If the inner slog parse fails we silently treat MESSAGE as
// plain text.)
func parseJournalEntry(line []byte) (Entry, error) {
	var r journalRecord
	if err := json.Unmarshal(line, &r); err != nil {
		return Entry{}, fmt.Errorf("parse journal record: %w", err)
	}

	e := Entry{
		Unit:    r.Unit,
		Time:    parseRealtimeMicros(r.Timestamp),
		Level:   priorityToLevel(r.Priority),
		Message: r.Message,
	}

	// Try to peel off an inner slog record. If MESSAGE itself parses
	// as a JSON object with a "level" key, prefer those values —
	// PowerLab services emit structured slog records which are
	// strictly more informative than the outer PRIORITY/MESSAGE.
	if r.Message != "" {
		var inner slogRecord
		if err := json.Unmarshal([]byte(r.Message), &inner); err == nil && inner.Level != "" {
			if lvl, ok := parseSlogLevel(inner.Level); ok {
				e.Level = lvl
			}
			if inner.Msg != "" {
				e.Message = inner.Msg
			}
		}
	}

	e.LevelS = e.Level.String()
	return e, nil
}

// priorityToLevel maps a syslog PRIORITY string to our Level enum.
// Unknown values default to INFO. The mapping follows RFC 5424
// severity codes:
//
//	0 emerg, 1 alert, 2 crit, 3 err, 4 warning, 5 notice, 6 info, 7 debug
//
// We collapse 0/1/2 to FATAL (the operator's "wake me up at 3 AM"
// bucket), 3 to ERROR, 4 to WARN, 5/6 to INFO, 7 to DEBUG.
func priorityToLevel(s string) Level {
	if s == "" {
		return LevelInfo
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return LevelInfo
	}
	switch n {
	case 0, 1, 2:
		return LevelFatal
	case 3:
		return LevelError
	case 4:
		return LevelWarn
	case 5, 6:
		return LevelInfo
	case 7:
		return LevelDebug
	default:
		return LevelInfo
	}
}

// parseSlogLevel reads the slog-format level string ("INFO" /
// "DEBUG" / "WARN" / "ERROR") and returns the Level enum. The
// second return is false when the input is not a known slog level
// — caller falls back to the outer PRIORITY mapping.
func parseSlogLevel(s string) (Level, bool) {
	switch s {
	case "DEBUG":
		return LevelDebug, true
	case "INFO":
		return LevelInfo, true
	case "WARN", "WARNING":
		return LevelWarn, true
	case "ERROR":
		return LevelError, true
	case "FATAL":
		return LevelFatal, true
	default:
		return LevelInfo, false
	}
}

// parseRealtimeMicros converts the journalctl __REALTIME_TIMESTAMP
// string (unix microseconds) into a time.Time. Malformed values
// return zero time — the caller decides whether that is renderable
// (the text formatter prints "(no timestamp)" rather than crash).
func parseRealtimeMicros(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMicro(n).UTC()
}
