package external

import (
	"os/exec"
	"strconv"
	"strings"
)

type GPUInfo struct {
	Index         int
	UUID          string
	DriverVersion string
	Name          string
	GPUSerial     string
}

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

func GPUInfoList() ([]GPUInfo, error) {
	gpusInfo, err := GPUInfoListWithSMI()
	if err != nil {
		return nil, err
	}
	return gpusInfo, nil
}

// Aliases for backward compatibility if needed
type NvidiaGPUInfo = GPUInfo

func NvidiaGPUInfoList() ([]NvidiaGPUInfo, error) {
	return GPUInfoList()
}

type GPUUtilization struct {
	Percent     float64 `json:"percent"`
	MemoryUsed  int64   `json:"memoryUsed"`
	Model       string  `json:"model"`
	Temperature int     `json:"temperature"`
}

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
