package model

// MountInfo is one filesystem mount in /v1/sys/disk's `mounts` array.
// Mirrors the subset of gopsutil's disk.UsageStat the dashboard
// actually consumes — total/used/free/used_percent for the storage
// bar widget, plus path + fs_type so the operator (or an MCP agent)
// can disambiguate when multiple mounts are present.
type MountInfo struct {
	Path        string  `json:"path"`
	FSType      string  `json:"fs_type"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// PhysicalDisk is one block device in /v1/sys/disk's `physical`
// array. Fields beyond Name+SizeBytes are best-effort: when smartctl
// is not installed (typical dev/macOS) or the operator lacks the
// CAP_SYS_RAWIO permission the disk is reported with empty Model /
// Serial / HealthStatus and TemperatureC = 0. An empty Model means
// "no SMART data available" — not a failure (same graceful-degrade
// pattern as system://gpu's empty model = no GPU).
type PhysicalDisk struct {
	Name          string `json:"name"`
	Model         string `json:"model"`
	Serial        string `json:"serial"`
	SizeBytes     uint64 `json:"size_bytes"`
	TemperatureC  int    `json:"temperature_c"`
	HealthStatus  string `json:"health_status"`
}

// DisksInfo is the /v1/sys/disk response shape — physical block
// devices (with optional SMART metadata) PLUS per-mount usage. Pre-
// release the route returned a single root-mount disk.UsageStat;
// MCP's system://disk description promised the richer surface (and
// the dashboard widget shows SMART health) so the shape was widened
// to match the documented contract.
type DisksInfo struct {
	Physical []PhysicalDisk `json:"physical"`
	Mounts   []MountInfo    `json:"mounts"`
}
