package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

// The parsers read the kernel's /proc text format. They're tested
// against captured real-world fixtures (not mocks) so a kernel format we
// don't expect fails loudly here rather than silently producing zeros an
// agent would misread as "box is idle / out of memory".

func TestParseMeminfo(t *testing.T) {
	const sample = `MemTotal:       16331524 kB
MemFree:          240112 kB
MemAvailable:    9876544 kB
Buffers:          123456 kB
Cached:          4567890 kB
`
	total, avail, err := parseMeminfo([]byte(sample))
	if err != nil {
		t.Fatalf("parseMeminfo: %v", err)
	}
	if total != 16331524 {
		t.Fatalf("MemTotal = %d; want 16331524", total)
	}
	if avail != 9876544 {
		t.Fatalf("MemAvailable = %d; want 9876544", avail)
	}
}

// MemAvailable is the field that matters (free + reclaimable); a kernel
// old enough to lack it must be an explicit error, not a 0 that reads as
// "no memory available".
func TestParseMeminfo_MissingAvailableIsError(t *testing.T) {
	const sample = "MemTotal:       16331524 kB\nMemFree:          240112 kB\n"
	if _, _, err := parseMeminfo([]byte(sample)); err == nil {
		t.Fatal("parseMeminfo with no MemAvailable returned nil error; want an error")
	}
}

func TestParseLoadavg(t *testing.T) {
	l1, l5, l15, err := parseLoadavg([]byte("0.52 0.38 0.29 2/431 18923\n"))
	if err != nil {
		t.Fatalf("parseLoadavg: %v", err)
	}
	if l1 != 0.52 || l5 != 0.38 || l15 != 0.29 {
		t.Fatalf("load = %v/%v/%v; want 0.52/0.38/0.29", l1, l5, l15)
	}
}

func TestParseUptime(t *testing.T) {
	up, err := parseUptime([]byte("123456.78 987654.32\n"))
	if err != nil {
		t.Fatalf("parseUptime: %v", err)
	}
	if up != 123456.78 {
		t.Fatalf("uptime = %v; want 123456.78", up)
	}
}

// Collect reads real files under a procRoot. Pointing it at a fixture
// dir exercises the whole read+parse+derive path end-to-end (used %
// computed from total/available) without depending on the host's /proc,
// so assertions are deterministic on any OS (including the macOS dev box
// that has no /proc).
func TestCollect_FromFixtureProc(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "meminfo", "MemTotal:       1000 kB\nMemAvailable:     250 kB\n")
	writeFile(t, dir, "loadavg", "1.50 0.75 0.25 1/100 5\n")
	writeFile(t, dir, "uptime", "3600.00 7200.00\n")

	m, err := Collect(dir)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if m.MemTotalKB != 1000 || m.MemAvailableKB != 250 {
		t.Fatalf("mem = %d/%d; want 1000/250", m.MemTotalKB, m.MemAvailableKB)
	}
	// used% = (1000-250)/1000 = 75.0
	if m.MemUsedPercent != 75.0 {
		t.Fatalf("MemUsedPercent = %v; want 75.0", m.MemUsedPercent)
	}
	if m.Load1 != 1.50 {
		t.Fatalf("Load1 = %v; want 1.50", m.Load1)
	}
	if m.UptimeSeconds != 3600.0 {
		t.Fatalf("UptimeSeconds = %v; want 3600.0", m.UptimeSeconds)
	}
}

// A missing procRoot (or unreadable file) is a real error — the resource
// must report failure, not hand the agent a zero-valued struct.
func TestCollect_MissingProcIsError(t *testing.T) {
	if _, err := Collect(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("Collect on a missing procRoot returned nil error; want an error")
	}
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}
