package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ANSI escape sequences for severity colouring (ADR-0027 §"Severity
// coloring"). Applied only when the destination is a TTY — pipe /
// redirect / `--no-color` fall through to the no-color path.
const (
	ansiReset    = "\x1b[0m"
	ansiBoldRed  = "\x1b[1;31m"
	ansiRed      = "\x1b[31m"
	ansiYellow   = "\x1b[33m"
	ansiDim      = "\x1b[2m"
	noTimestamp  = "(no timestamp)"
	timestampFmt = time.RFC3339
)

// writeTextLine emits one human-readable line for the entry to w.
// When color is true, severity tokens get wrapped in ANSI escape
// codes (and the trailing reset). When color is false, output is
// plain ASCII so logs can be piped through grep/awk cleanly.
//
// Format (concrete example):
//
//	2026-05-13T20:15:30Z  ERROR  powerlab-core.service  connection refused
//
// Columns separated by two spaces — easy to read, easy to grep.
// Zero-time entries render the literal "(no timestamp)" rather than
// the misleading 1970-01-01.
func writeTextLine(w io.Writer, e Entry, color bool) {
	ts := noTimestamp
	if !e.Time.IsZero() {
		ts = e.Time.UTC().Format(timestampFmt)
	}

	lvl := e.Level.String()
	if color {
		lvl = colorize(e.Level, lvl)
	}

	fmt.Fprintf(w, "%s  %-5s  %s  %s\n", ts, lvl, e.Unit, e.Message)
}

// writeJSONLine emits the entry as one compact JSON object per line
// (newline-delimited JSON, a.k.a. ndjson). Never colored — the
// `--json` flag is meant for piping through `jq` / log shippers,
// where ANSI would corrupt the parser.
func writeJSONLine(w io.Writer, e Entry) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(e)
}

// colorize wraps a token in the ANSI escape pair appropriate for
// the level. INFO is intentionally left untouched (default fg) —
// terminals already render it readably and we want the eye drawn
// to the WARN/ERROR/FATAL rows. DEBUG is dimmed to push it visually
// into the background.
//
// Splitting this out (vs. inlining) makes the format_test cases
// easy: each severity-coloring assertion is a one-liner against
// the substrings here.
func colorize(level Level, token string) string {
	switch level {
	case LevelFatal:
		return ansiBoldRed + token + ansiReset
	case LevelError:
		return ansiRed + token + ansiReset
	case LevelWarn:
		return ansiYellow + token + ansiReset
	case LevelDebug:
		return ansiDim + token + ansiReset
	default:
		// INFO — no escape sequence, return as-is.
		return token
	}
}
