package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// JournalOpts shapes the runtime formatting choices for the journal
// subcommand. Color is TTY-detected by the caller (cobra layer);
// JSON forces NDJSON output regardless.
type JournalOpts struct {
	Color bool
	JSON  bool
}

// JournalArgs models the user-facing flag surface. Translated into
// `journalctl` command-line args by buildJournalArgs.
//
//	powerlab-logs journal --service core --follow --lines 200
type JournalArgs struct {
	Service string // bare service name, "core" → powerlab-core.service
	Follow  bool   // --follow → journalctl -f
	Lines   int    // --lines N → journalctl -n N; 0 means default
}

// runJournal reads `journalctl -o json` output line-by-line from r,
// parses each into an Entry, and formats it to w per opts. Designed
// to be unit-testable: production code wires a real journalctl
// subprocess's stdout in as r; tests pass any io.Reader.
//
// Skip semantics:
//   - blank lines are silently ignored (journalctl rotation gaps,
//     etc.)
//   - lines that fail to parse as journal JSON are silently dropped
//     (journalctl shouldn't emit them; if it does, we'd rather
//     surface the parseable lines than abort the stream).
func runJournal(r io.Reader, w io.Writer, opts JournalOpts) error {
	scanner := bufio.NewScanner(r)
	// journalctl lines can be long when MESSAGE carries a multi-line
	// stack trace; bump the buffer ceiling to 1 MiB (bufio default
	// is 64 KiB, which truncates legitimately-long records).
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		entry, err := parseJournalEntry(line)
		if err != nil {
			// Malformed → skip silently.
			continue
		}
		if opts.JSON {
			if err := writeJSONLine(w, entry); err != nil {
				return fmt.Errorf("emit JSON line: %w", err)
			}
		} else {
			writeTextLine(w, entry, opts.Color)
		}
	}
	return scanner.Err()
}

// buildJournalArgs translates the user-facing JournalArgs into the
// argv slice passed to `journalctl`. Pure function.
//
// Defaults applied:
//   - `-o json` for every invocation (parser depends on it)
//   - --service unset → `-u powerlab-*.service` wildcard
//   - --follow → trailing `-f`
//   - --lines N (N>0) → `-n N`
func buildJournalArgs(args JournalArgs) []string {
	out := []string{"-o", "json"}
	if args.Service != "" {
		out = append(out, "-u", "powerlab-"+args.Service+".service")
	} else {
		out = append(out, "-u", "powerlab-*.service")
	}
	if args.Lines > 0 {
		out = append(out, "-n", strconv.Itoa(args.Lines))
	}
	if args.Follow {
		out = append(out, "-f")
	}
	return out
}
