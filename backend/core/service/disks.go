package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"

	"github.com/neochaotic/powerlab/backend/core/model"
)

// GetDisks returns the rich shape {physical, mounts} the
// /v1/sys/disk route promises (see route/v1/system.go) and that
// powerlab-mcp's system://disk description advertises (see
// resources_system.go). Pre-bug the route returned a single
// disk.UsageStat for the root mount only — agents querying
// "show me my disks" got back one object with no SMART data and
// no awareness of additional mounts.
//
// Physical inventory is best-effort: smartctl is not a hard dep,
// and macOS / containerised CI sandboxes typically can't run it
// (no /dev/sd*, no CAP_SYS_RAWIO). When smartctl is absent or any
// individual probe fails the entry is reported with empty Model /
// Serial / HealthStatus and TemperatureC=0 — same graceful-degrade
// pattern as system://gpu's empty Model = "no GPU detected".
//
// Mounts is the gopsutil disk.Partitions() snapshot filtered to
// physical/visible filesystems (skips synthetic /proc, /sys, etc.
// via the gopsutil "all=false" flag) then enriched with the
// per-mount disk.Usage() readout.
func (c *systemService) GetDisks() model.DisksInfo {
	return model.DisksInfo{
		Physical: collectPhysicalDisks(),
		Mounts:   collectMounts(),
	}
}

// collectMounts walks gopsutil's filtered partition list and
// returns one MountInfo per usable filesystem. Always returns a
// non-nil slice (so JSON marshals to `[]` not `null` even if the
// host is somehow degenerate).
func collectMounts() []model.MountInfo {
	out := make([]model.MountInfo, 0, 4)

	// all=false skips pseudo-filesystems (proc, sysfs, cgroup,
	// overlay-on-overlay container layers, etc.). gopsutil also
	// handles the Windows drive-letter enumeration so the route
	// stays portable for dev.
	parts, err := disk.Partitions(false)
	if err != nil {
		// On macOS dev / containers gopsutil sometimes returns
		// ENOENT for /etc/mtab — fall back to the legacy single-
		// root mount via disk.Usage so the agent still gets
		// something meaningful.
		return appendRootFallback(out)
	}

	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		// Some hosts list the same device under several mount
		// paths (bind mounts, snap loop devices). De-duplicate by
		// mount path — the agent cares about distinct mounts,
		// not how many bind targets share a backing fs.
		if _, ok := seen[p.Mountpoint]; ok {
			continue
		}
		seen[p.Mountpoint] = struct{}{}

		u, err := disk.Usage(p.Mountpoint)
		if err != nil || u == nil {
			continue
		}
		out = append(out, model.MountInfo{
			Path:        p.Mountpoint,
			FSType:      p.Fstype,
			Total:       u.Total,
			Used:        u.Used,
			Free:        u.Free,
			UsedPercent: roundPercent(u.UsedPercent),
		})
	}

	if len(out) == 0 {
		return appendRootFallback(out)
	}
	return out
}

// appendRootFallback emits a single root-mount entry when
// disk.Partitions returns nothing useful — preserves the
// pre-bug guarantee that `/v1/sys/disk` always carries at
// least one entry.
func appendRootFallback(out []model.MountInfo) []model.MountInfo {
	rootPath := "/"
	if runtime.GOOS == "windows" {
		rootPath = "C:"
	}
	if u, err := disk.Usage(rootPath); err == nil && u != nil {
		out = append(out, model.MountInfo{
			Path:        u.Path,
			FSType:      u.Fstype,
			Total:       u.Total,
			Used:        u.Used,
			Free:        u.Free,
			UsedPercent: roundPercent(u.UsedPercent),
		})
	}
	return out
}

// roundPercent matches the 1-decimal rounding the pre-existing
// GetDiskInfo applied. Kept as a free helper so the bug-report
// path (route/v1/system.go::GetSystemConfigDebug) and the new
// per-mount path produce numerically identical wire values.
func roundPercent(p float64) float64 {
	rounded := math.Round(p*10) / 10
	// strconv round-trip mirrors the pre-bug behaviour of
	// fmt.Sprintf("%.1f", p) + ParseFloat — keeps the contract
	// pinned even when math.Round disagrees with sprintf at the
	// .x5 midpoint.
	s := strconv.FormatFloat(rounded, 'f', 1, 64)
	parsed, _ := strconv.ParseFloat(s, 64)
	return parsed
}

// collectPhysicalDisks enumerates block devices via smartctl when
// available; on hosts without smartctl (or where the binary fails
// to enumerate) returns an empty non-nil slice. Never panics —
// smartctl output formats vary across distros and we'd rather
// degrade gracefully than crash the route.
func collectPhysicalDisks() []model.PhysicalDisk {
	out := make([]model.PhysicalDisk, 0)

	// `smartctl --scan -j` enumerates the visible devices in JSON.
	// 5-second budget — the typical host completes in <100ms; the
	// cap protects against a hung disk pinning the route.
	scan, err := runSmartctl(5*time.Second, "--scan", "-j")
	if err != nil || len(scan) == 0 {
		return out
	}

	var scanResult struct {
		Devices []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(scan, &scanResult); err != nil {
		return out
	}

	for _, dev := range scanResult.Devices {
		entry := model.PhysicalDisk{Name: dev.Name}
		// `-a -j` returns model + serial + size + temperature +
		// SMART overall health in one shot. 5-second budget per
		// device — same hung-disk protection as the scan call.
		buf, err := runSmartctl(5*time.Second, "-a", "-j", dev.Name)
		if err == nil && len(buf) > 0 {
			fillFromSmartctl(&entry, buf)
		}
		out = append(out, entry)
	}
	return out
}

// fillFromSmartctl decodes the subset of smartctl's -a -j output
// the dashboard + MCP description care about. Unknown / missing
// fields stay at the zero value rather than failing the whole
// physical-disk entry — partial SMART is better than none.
func fillFromSmartctl(d *model.PhysicalDisk, buf []byte) {
	var raw struct {
		ModelName    string `json:"model_name"`
		SerialNumber string `json:"serial_number"`
		UserCapacity struct {
			Bytes uint64 `json:"bytes"`
		} `json:"user_capacity"`
		Temperature struct {
			Current int `json:"current"`
		} `json:"temperature"`
		SMARTStatus struct {
			Passed bool `json:"passed"`
		} `json:"smart_status"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return
	}
	d.Model = strings.TrimSpace(raw.ModelName)
	d.Serial = strings.TrimSpace(raw.SerialNumber)
	d.SizeBytes = raw.UserCapacity.Bytes
	d.TemperatureC = raw.Temperature.Current
	// smartctl exposes overall-health as a boolean. Map to the
	// short token the dashboard widget already renders: "PASSED"
	// when the disk reports green, "FAILED" when red, empty when
	// the JSON didn't include a smart_status object at all.
	if raw.SMARTStatus.Passed {
		d.HealthStatus = "PASSED"
	} else if buf != nil && strings.Contains(string(buf), `"smart_status"`) {
		d.HealthStatus = "FAILED"
	}
}

// runSmartctl shells out to the smartctl binary with a bounded
// context. Returns (nil, err) when smartctl is not on $PATH —
// callers MUST treat that as "no SMART data available" rather
// than an error.
func runSmartctl(budget time.Duration, args ...string) ([]byte, error) {
	if _, err := exec.LookPath("smartctl"); err != nil {
		return nil, fmt.Errorf("smartctl not found: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	// smartctl exits non-zero when the device has SMART errors but
	// still prints the JSON we want; we ignore the exit code and
	// only care whether we got parseable output.
	out, _ := exec.CommandContext(ctx, "smartctl", args...).Output()
	return out, nil
}
