package model

// ProcessSummary is one entry in /v1/sys/processes — the executable
// NAME of a running process plus the resource it's consuming. Cmdline
// is deliberately omitted at the API layer: argv routinely carries
// secrets (passwords passed as flags, signed URLs, JWT tokens via env
// expansion). The agent surface that consumes this (powerlab-mcp
// system://processes per ADR-0044) needs "what's eating my CPU" not
// "what arguments was it launched with".
type ProcessSummary struct {
	PID    int32   `json:"pid"`
	Name   string  `json:"name"`
	CPUPct float64 `json:"cpu_percent"`      // 0..100*cores
	MemPct float64 `json:"mem_percent"`      // 0..100
	RSSKB  uint64  `json:"rss_kb,omitempty"` // resident set size in KiB
	User   string  `json:"user,omitempty"`
}

// ProcessesSummary is the /v1/sys/processes response shape — total
// process count + the top-N by CPU and by memory.
type ProcessesSummary struct {
	Total     int              `json:"total"`
	TopByCPU  []ProcessSummary `json:"top_by_cpu"`
	TopByMem  []ProcessSummary `json:"top_by_mem"`
	Truncated bool             `json:"truncated"` // true when Total > len(TopByCPU)|len(TopByMem)
}
