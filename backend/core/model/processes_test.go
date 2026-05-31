package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// SECURITY LOCK — ProcessSummary must NEVER expose argv / cmdline /
// command-line. The whole point of /v1/sys/processes (and the
// powerlab-mcp system://processes proxy that consumes it) is "name
// only" because argv routinely carries secrets: passwords passed as
// flags, signed URLs, JWT tokens via env expansion.
//
// If a future refactor adds a Cmdline / Args / CommandLine field to
// ProcessSummary, this test fails loud BEFORE the leak ships.
func TestProcessSummary_NeverLeaksCmdline(t *testing.T) {
	forbidden := []string{"cmdline", "args", "argv", "commandline", "command_line"}

	rt := reflect.TypeOf(ProcessSummary{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		// Check both Go field name and JSON tag — defence in depth in
		// case someone renames the field but keeps the dangerous tag.
		jsonTag := strings.Split(f.Tag.Get("json"), ",")[0]
		for _, bad := range forbidden {
			if strings.EqualFold(f.Name, bad) {
				t.Errorf("ProcessSummary has forbidden field %q — argv leaks secrets, /v1/sys/processes is name-only by design", f.Name)
			}
			if strings.EqualFold(jsonTag, bad) {
				t.Errorf("ProcessSummary field %q has forbidden JSON tag %q", f.Name, jsonTag)
			}
		}
	}

	// Belt + braces: marshal an empty ProcessSummary and assert the
	// forbidden tokens never appear in the wire output either.
	out, err := json.Marshal(ProcessSummary{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	low := strings.ToLower(string(out))
	for _, bad := range forbidden {
		if strings.Contains(low, bad) {
			t.Errorf("ProcessSummary JSON contains forbidden token %q: %s", bad, string(out))
		}
	}
}

// ProcessSummary's wire shape — agents + UI parse on these keys; if
// any rename, this test fails so the contract change is intentional.
func TestProcessSummary_StableWireKeys(t *testing.T) {
	out, err := json.Marshal(ProcessSummary{PID: 1, Name: "x", CPUPct: 1, MemPct: 2, RSSKB: 3, User: "u"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"pid"`, `"name"`, `"cpu_percent"`, `"mem_percent"`, `"rss_kb"`, `"user"`} {
		if !strings.Contains(string(out), key) {
			t.Errorf("ProcessSummary JSON missing wire key %s: %s", key, string(out))
		}
	}
}

// ProcessesSummary wire keys — top_by_cpu / top_by_mem are the
// agent's primary parse path; locking them here.
func TestProcessesSummary_StableWireKeys(t *testing.T) {
	out, err := json.Marshal(ProcessesSummary{Total: 42, TopByCPU: []ProcessSummary{}, TopByMem: []ProcessSummary{}, Truncated: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"total"`, `"top_by_cpu"`, `"top_by_mem"`, `"truncated"`} {
		if !strings.Contains(string(out), key) {
			t.Errorf("ProcessesSummary JSON missing wire key %s: %s", key, string(out))
		}
	}
}
