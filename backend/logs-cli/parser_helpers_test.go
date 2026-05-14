package main

import (
	"testing"
)

// Direct unit tests for the helpers that the higher-level
// parseJournalEntry exercises indirectly. Filling these gaps brings
// coverage above the >=95% target in ADR-0027 §"Testing discipline".

func TestParseSlogLevel_AllKnown(t *testing.T) {
	cases := []struct {
		in   string
		want Level
		ok   bool
	}{
		{"DEBUG", LevelDebug, true},
		{"INFO", LevelInfo, true},
		{"WARN", LevelWarn, true},
		{"WARNING", LevelWarn, true},
		{"ERROR", LevelError, true},
		{"FATAL", LevelFatal, true},
		{"", LevelInfo, false},
		{"NOTICE", LevelInfo, false},
	}
	for _, c := range cases {
		got, ok := parseSlogLevel(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseSlogLevel(%q) = (%v, %v); want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestPriorityToLevel_EmptyInput(t *testing.T) {
	if got := priorityToLevel(""); got != LevelInfo {
		t.Errorf("priorityToLevel(\"\") = %v, want INFO", got)
	}
}

func TestPriorityToLevel_NonNumericInput(t *testing.T) {
	// Atoi-fail case: a non-empty but non-numeric PRIORITY string
	// (e.g., journal driver bug) should fall back to INFO rather
	// than panic. Real-world: never observed, but the safety net
	// matters when running powerlab-logs against arbitrary journal
	// dumps from CI investigations.
	if got := priorityToLevel("not-a-number"); got != LevelInfo {
		t.Errorf("priorityToLevel(\"not-a-number\") = %v, want INFO", got)
	}
}

func TestParseRealtimeMicros_EmptyInput(t *testing.T) {
	if got := parseRealtimeMicros(""); !got.IsZero() {
		t.Errorf("parseRealtimeMicros(\"\") = %v, want zero time", got)
	}
}

func TestLevelString_Default(t *testing.T) {
	// Construct a Level outside the iota range to exercise the
	// default branch in String(). This guards against accidental
	// drift when someone adds a new level without updating String.
	weird := Level(999)
	if got := weird.String(); got != "INFO" {
		t.Errorf("Level(999).String() = %q, want INFO (default fallback)", got)
	}
}
