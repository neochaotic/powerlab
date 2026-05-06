package codegen

type ComposeAppStats struct {
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsedBytes  int64   `json:"mem_used_bytes"`
	MemoryLimitBytes int64   `json:"mem_limit_bytes"`
	NetRxBytes       int64   `json:"net_rx"`
	NetTxBytes       int64   `json:"net_tx"`
}

type ComposeAppStatsOK struct {
	Data    *ComposeAppStats `json:"data,omitempty"`
	Message *string          `json:"message,omitempty"`
}

type ErrorResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type Response struct {
	Success int         `json:"success,omitempty"`
	Message *string     `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type AppManagementConfig struct {
	StoragePath string `json:"storage_path"`
	AppsPath    string `json:"apps_path"`
}

type DiskUsage struct {
	Bytes int64 `json:"bytes"`
}
