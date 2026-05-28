// Package metrics reads point-in-time host metrics straight from the
// kernel's /proc filesystem — no gateway, no external collector — so the
// system:// MCP resource keeps working even when the rest of PowerLab is
// down (ADR-0034 independence).
//
// Everything is a single read of a /proc text file; CPU-utilisation
// deltas (which need two samples over time) are deliberately out of
// scope here — load average is the honest single-shot CPU signal.
package metrics

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Metrics is a point-in-time snapshot of host health.
type Metrics struct {
	MemTotalKB     int64   `json:"mem_total_kb"`
	MemAvailableKB int64   `json:"mem_available_kb"`
	MemUsedPercent float64 `json:"mem_used_percent"`
	Load1          float64 `json:"load1"`
	Load5          float64 `json:"load5"`
	Load15         float64 `json:"load15"`
	UptimeSeconds  float64 `json:"uptime_seconds"`
}

// Collect reads and parses the relevant /proc files under procRoot
// (normally "/proc"; tests point it at a fixture dir) and derives the
// snapshot. Any read or parse failure is returned — the caller must
// surface it, never hand out a zero-valued snapshot that an agent would
// misread as "idle / out of memory".
func Collect(procRoot string) (Metrics, error) {
	var m Metrics

	mem, err := os.ReadFile(filepath.Join(procRoot, "meminfo"))
	if err != nil {
		return m, fmt.Errorf("read meminfo: %w", err)
	}
	m.MemTotalKB, m.MemAvailableKB, err = parseMeminfo(mem)
	if err != nil {
		return m, err
	}
	if m.MemTotalKB > 0 {
		m.MemUsedPercent = round2(float64(m.MemTotalKB-m.MemAvailableKB) / float64(m.MemTotalKB) * 100)
	}

	load, err := os.ReadFile(filepath.Join(procRoot, "loadavg"))
	if err != nil {
		return m, fmt.Errorf("read loadavg: %w", err)
	}
	m.Load1, m.Load5, m.Load15, err = parseLoadavg(load)
	if err != nil {
		return m, err
	}

	up, err := os.ReadFile(filepath.Join(procRoot, "uptime"))
	if err != nil {
		return m, fmt.Errorf("read uptime: %w", err)
	}
	m.UptimeSeconds, err = parseUptime(up)
	if err != nil {
		return m, err
	}

	return m, nil
}

// parseMeminfo extracts MemTotal and MemAvailable (both in kB) from the
// /proc/meminfo body. MemAvailable is required: a kernel too old to
// report it is an error rather than a silent zero.
func parseMeminfo(b []byte) (total, available int64, err error) {
	var haveTotal, haveAvail bool
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		kb, perr := parseKB(val)
		if perr != nil {
			continue
		}
		switch key {
		case "MemTotal":
			total, haveTotal = kb, true
		case "MemAvailable":
			available, haveAvail = kb, true
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, fmt.Errorf("scan meminfo: %w", err)
	}
	if !haveTotal {
		return 0, 0, fmt.Errorf("meminfo: MemTotal not found")
	}
	if !haveAvail {
		return 0, 0, fmt.Errorf("meminfo: MemAvailable not found (kernel too old?)")
	}
	return total, available, nil
}

// parseKB turns " 16331524 kB" into 16331524.
func parseKB(field string) (int64, error) {
	f := strings.Fields(field)
	if len(f) == 0 {
		return 0, fmt.Errorf("empty field")
	}
	return strconv.ParseInt(f[0], 10, 64)
}

// parseLoadavg reads the 1/5/15-minute load averages from /proc/loadavg.
func parseLoadavg(b []byte) (l1, l5, l15 float64, err error) {
	f := strings.Fields(string(b))
	if len(f) < 3 {
		return 0, 0, 0, fmt.Errorf("loadavg: expected at least 3 fields, got %d", len(f))
	}
	if l1, err = strconv.ParseFloat(f[0], 64); err != nil {
		return 0, 0, 0, fmt.Errorf("loadavg load1: %w", err)
	}
	if l5, err = strconv.ParseFloat(f[1], 64); err != nil {
		return 0, 0, 0, fmt.Errorf("loadavg load5: %w", err)
	}
	if l15, err = strconv.ParseFloat(f[2], 64); err != nil {
		return 0, 0, 0, fmt.Errorf("loadavg load15: %w", err)
	}
	return l1, l5, l15, nil
}

// parseUptime reads the first field (seconds since boot) of /proc/uptime.
func parseUptime(b []byte) (float64, error) {
	f := strings.Fields(string(b))
	if len(f) < 1 {
		return 0, fmt.Errorf("uptime: empty")
	}
	return strconv.ParseFloat(f[0], 64)
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
