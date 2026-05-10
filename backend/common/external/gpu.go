package external

import (
	"os/exec"
	"strconv"
	"strings"
)

// GPUInfo is one nvidia-smi --query-gpu row. Index is the local GPU
// number (0-based); UUID is stable across reboots and is the safe
// key for per-GPU config. PowerLab's GPU monitoring (the homepage
// category differentiator vs. CasaOS) reads these to populate the
// dashboard widget.
type GPUInfo struct {
	Index         int
	UUID          string
	DriverVersion string
	Name          string
	GPUSerial     string
}

// GPUInfoListWithSMI shells out to nvidia-smi and returns one
// GPUInfo per detected GPU. Returns an error when nvidia-smi is
// unavailable — callers should treat that as "no GPU" rather than
// an outright failure.
func GPUInfoListWithSMI() ([]GPUInfo, error) {
	GPUInfos := []GPUInfo{}

	output, err := exec.Command("nvidia-smi", "--query-gpu=index,uuid,driver_version,name,gpu_serial", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		value := strings.Split(line, ", ")
		if len(value) == 5 {
			index, _ := strconv.Atoi(value[0])

			GPUInfos = append(GPUInfos, GPUInfo{
				Index:         index,
				UUID:          value[1],
				DriverVersion: value[2],
				Name:          value[3],
				GPUSerial:     value[4],
			})
		} else {
			continue
		}
	}
	return GPUInfos, nil
}

// GPUInfoList is the public entry point — currently a thin
// wrapper over the SMI implementation. The indirection exists so
// AMD/Intel detection can land here without changing call sites.
func GPUInfoList() ([]GPUInfo, error) {
	gpusInfo, err := GPUInfoListWithSMI()
	if err != nil {
		return nil, err
	}
	return gpusInfo, nil
}

// NvidiaGPUInfo is the legacy name for GPUInfo — kept as an alias
// for code written before the multi-vendor rename.
type NvidiaGPUInfo = GPUInfo

// NvidiaGPUInfoList is the legacy name for GPUInfoList.
func NvidiaGPUInfoList() ([]NvidiaGPUInfo, error) {
	return GPUInfoList()
}

// GPUUtilization is the live performance snapshot used by the
// dashboard widget. MemoryUsed is in bytes (the SMI/ioreg readers
// normalise to bytes before populating). Temperature is unset on
// macOS where IOAccelerator does not surface it.
type GPUUtilization struct {
	Percent     float64 `json:"percent"`
	MemoryUsed  int64   `json:"memoryUsed"`
	Model       string  `json:"model"`
	Temperature int     `json:"temperature"`
}

// GetGPUUtilization returns a single live snapshot of the
// primary GPU's utilization. Tries macOS Apple Silicon first
// (system_profiler + ioreg parsing), then falls back to
// nvidia-smi on Linux. Returns nil when no supported GPU
// readout is available — caller should treat as "no GPU stats"
// rather than an error.
func GetGPUUtilization() *GPUUtilization {
	// macOS Apple Silicon (M1/M2/M3/M4/M5)
	if out, err := exec.Command("uname").Output(); err == nil && strings.TrimSpace(string(out)) == "Darwin" {
		util := &GPUUtilization{Model: "Apple Silicon GPU"}
		
		// Attempt to get exact model
		if prof, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
			for _, line := range strings.Split(string(prof), "\n") {
				if strings.Contains(line, "Chipset Model:") {
					util.Model = strings.TrimSpace(strings.Split(line, ":")[1])
					break
				}
			}
		}

		// Read performance statistics from ioreg (targeted at IOAccelerator for speed)
		if ioreg, err := exec.Command("ioreg", "-n", "IOAccelerator", "-w0", "-l").Output(); err == nil {
			output := string(ioreg)
			
			// Extract values using string searching to avoid regex complexities with ioreg output
			extractValue := func(key string) string {
				searchKey := "\"" + key + "\"="
				idx := strings.Index(output, searchKey)
				if idx == -1 {
					return ""
				}
				start := idx + len(searchKey)
				end := start
				for end < len(output) && (output[end] >= '0' && output[end] <= '9') {
					end++
				}
				return output[start:end]
			}

			if valStr := extractValue("Device Utilization %"); valStr != "" {
				val, _ := strconv.ParseFloat(valStr, 64)
				util.Percent = val
			} else if valStr := extractValue("Renderer Utilization %"); valStr != "" {
				// Fallback to Renderer utilization if Device is not present
				val, _ := strconv.ParseFloat(valStr, 64)
				util.Percent = val
			}

			if valStr := extractValue("In use system memory"); valStr != "" {
				val, _ := strconv.ParseInt(valStr, 10, 64)
				util.MemoryUsed = val
			}
		}
		// If we actually read something, return it
		if util.Model != "Apple Silicon GPU" || util.MemoryUsed > 0 {
			return util
		}
	}

	// Linux (Nvidia)
	out, err := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,memory.used,temperature.gpu,name", "--format=csv,noheader,nounits").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 {
			parts := strings.Split(lines[0], ", ")
			if len(parts) >= 4 {
				percent, _ := strconv.ParseFloat(parts[0], 64)
				memMB, _ := strconv.ParseInt(parts[1], 10, 64)
				temp, _ := strconv.Atoi(parts[2])
				model := parts[3]

				return &GPUUtilization{
					Percent:     percent,
					MemoryUsed:  memMB * 1024 * 1024, // Convert MB to Bytes
					Temperature: temp,
					Model:       model,
				}
			}
		}
	}

	return nil
}
