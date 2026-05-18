package service

import (
	"errors"
	"fmt"
	net2 "net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/command"
	"github.com/neochaotic/powerlab/backend/common/utils/constants"
	exec2 "github.com/neochaotic/powerlab/backend/common/utils/exec"

	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/neochaotic/powerlab/backend/core/model"
	"github.com/neochaotic/powerlab/backend/core/pkg/config"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/ip_helper"
	"go.uber.org/zap"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// SystemService is the largest service in core — owns
// system-info aggregation (CPU, memory, disk, network, sensors)
// for the homepage stats widgets, file/directory traversal, the
// reboot/shutdown actions, timezone management, and the legacy
// CasaOS log + system-entry endpoints.
//
// Method count is large (35+); per-method docs would add little
// signal here since the names are self-describing wrappers around
// gopsutil + os calls. The interface serves as the contract surface
// the route layer binds to — implementations live in systemService
// below.
type SystemService interface {
	GetSystemConfigDebug() []string
	GetSystemLogs(lineNumber int) string
	UpdateAssist()
	UpSystemPort(port string)
	GetTimeZone() string
	SetTimeZone(timezone string) error
	UpAppOrderFile(str, id string)
	GetAppOrderFile(id string) []byte
	GetNet(physics bool) []string
	GetNetInfo() []net.IOCountersStat
	GetCpuCoreNum() int
	GetCpuPercent() float64
	GetMemInfo() map[string]interface{}
	GetCpuInfo() []cpu.InfoStat
	GetDirPath(path string) ([]model.Path, error)
	GetDirPathOne(path string) (m model.Path)
	GetNetState(name string) string
	GetDiskInfo() *disk.UsageStat
	GetSysInfo() host.InfoStat
	GetDeviceTree() string
	CreateFile(path string) (int, error)
	RenameFile(oldF, newF string) (int, error)
	MkdirAll(path string) (int, error)
	GetCPUTemperature() int
	GetCPUPower() map[string]string
	GetMacAddress() (string, error)
	SystemReboot() error
	SystemShutdown() error
	GetSystemEntry() string
	GenreateSystemEntry()
	GetNetworkInterfaces() []model.NetworkInterface
	GetSystemUsers() []model.SystemUser
}
type systemService struct{}

func (c *systemService) GenreateSystemEntry() {
	// constants.DefaultDataPath resolves per-platform: /var/lib/powerlab
	// on Linux, /opt/powerlab/lib on darwin. Sprint 3 Phase 3 rebrand:
	// was hardcoded /var/lib/casaos/www/modules.
	modelsPath := filepath.Join(constants.DefaultDataPath, "www", "modules")
	entryFileName := "entry.json"
	entryFilePath := filepath.Join(config.AppInfo.DBPath, "db", entryFileName)
	file.IsNotExistCreateFile(entryFilePath)

	dir, err := os.ReadDir(modelsPath)
	if err != nil {
		logger.Error("read dir error", zap.Error(err))
		return
	}
	json := "["
	for _, v := range dir {
		data, err := os.ReadFile(filepath.Join(modelsPath, v.Name(), entryFileName))
		if err != nil {
			logger.Error("read entry file error", zap.Error(err))
			continue
		}
		json += string(data) + ","
	}
	json = strings.TrimRight(json, ",")
	json += "]"
	err = os.WriteFile(entryFilePath, []byte(json), 0o666)
	if err != nil {
		logger.Error("write entry file error", zap.Error(err))
		return
	}
}

func (c *systemService) GetSystemEntry() string {
	// constants.DefaultDataPath resolves per-platform: /var/lib/powerlab
	// on Linux, /opt/powerlab/lib on darwin. Sprint 3 Phase 3 rebrand:
	// was hardcoded /var/lib/casaos/www/modules.
	modelsPath := filepath.Join(constants.DefaultDataPath, "www", "modules")
	entryFileName := "entry.json"
	dir, err := os.ReadDir(modelsPath)
	if err != nil {
		logger.Error("read dir error", zap.Error(err))
		return ""
	}
	json := "["
	for _, v := range dir {
		data, err := os.ReadFile(filepath.Join(modelsPath, v.Name(), entryFileName))
		if err != nil {
			logger.Error("read entry file error", zap.Error(err))
			continue
		}
		json += string(data) + ","
	}
	json = strings.TrimRight(json, ",")
	json += "]"
	if err != nil {
		logger.Error("write entry file error", zap.Error(err))
		return ""
	}
	return json
}

func (c *systemService) GetMacAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	nets := MyService.System().GetNet(true)
	for _, v := range interfaces {
		for _, n := range nets {
			if v.Name == n {
				return v.HardwareAddr, nil
			}
		}
	}
	return "", errors.New("not found")
}

func (c *systemService) MkdirAll(path string) (int, error) {
	_, err := os.Stat(path)
	if err == nil {
		return common_err.DIR_ALREADY_EXISTS, nil
	} else {
		if os.IsNotExist(err) {
			os.MkdirAll(path, os.ModePerm)
			return common_err.SUCCESS, nil
		} else if strings.Contains(err.Error(), ": not a directory") {
			return common_err.FILE_OR_DIR_EXISTS, err
		}
	}
	return common_err.SERVICE_ERROR, err
}

func (c *systemService) RenameFile(oldF, newF string) (int, error) {
	_, err := os.Stat(newF)
	if err == nil {
		return common_err.DIR_ALREADY_EXISTS, nil
	} else {
		if os.IsNotExist(err) {
			err := os.Rename(oldF, newF)
			if err != nil {
				return common_err.SERVICE_ERROR, err
			}
			return common_err.SUCCESS, nil
		}
	}
	return common_err.SERVICE_ERROR, err
}

func (c *systemService) CreateFile(path string) (int, error) {
	_, err := os.Stat(path)
	if err == nil {
		return common_err.FILE_OR_DIR_EXISTS, nil
	} else {
		if os.IsNotExist(err) {
			file.CreateFile(path)
			return common_err.SUCCESS, nil
		}
	}
	return common_err.SERVICE_ERROR, err
}

func (c *systemService) GetDeviceTree() string {
	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;GetDeviceTree"); err != nil {
		return ""
	} else {
		return output
	}
}

func (c *systemService) GetSysInfo() host.InfoStat {
	info, _ := host.Info()
	return *info
}

func (c *systemService) GetDiskInfo() *disk.UsageStat {
	path := "/"
	if runtime.GOOS == "windows" {
		path = "C:"
	}
	diskInfo, _ := disk.Usage(path)
	diskInfo.UsedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.1f", diskInfo.UsedPercent), 64)
	diskInfo.InodesUsedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.1f", diskInfo.InodesUsedPercent), 64)
	return diskInfo
}

func (c *systemService) GetNetState(name string) string {
	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;CatNetCardState " + name); err != nil {
		return ""
	} else {
		return output
	}
}

func (c *systemService) GetDirPathOne(path string) (m model.Path) {
	f, err := os.Stat(path)
	if err != nil {
		return
	}
	m.IsDir = f.IsDir()
	m.Name = f.Name()
	m.Path = path
	m.Size = f.Size()
	m.Date = f.ModTime()
	return
}

func (c *systemService) GetDirPath(path string) ([]model.Path, error) {
	if path == "/DATA" {
		sysType := runtime.GOOS
		if sysType == "windows" {
			path = "C:\\CasaOS\\DATA"
		}
		if sysType == "darwin" {
			path = "./local_data/DATA"
		}
	}
	// Create local data dir if not exists (development mode)
	if _, err := os.Stat(path); os.IsNotExist(err) && (path == "./local_data/DATA") {
		os.MkdirAll(path, 0755)
	}

	ls, err := os.ReadDir(path)
	if err != nil {
		logger.Error("when read dir", zap.Error(err))
		return []model.Path{}, err
	}
	dirs := []model.Path{}
	if len(path) > 0 {
		for _, l := range ls {
			filePath := filepath.Join(path, l.Name())
			link, err := filepath.EvalSymlinks(filePath)
			if err != nil {
				link = filePath
			}
			tempFile, err := l.Info()
			if err != nil {
				logger.Error("when read dir", zap.Error(err))
				return []model.Path{}, err
			}
			temp := model.Path{Name: l.Name(), Path: filePath, IsDir: l.IsDir(), Date: tempFile.ModTime(), Size: tempFile.Size()}
			if filePath != link {
				file, _ := os.Stat(link)
				temp.IsDir = file.IsDir()
			}
			dirs = append(dirs, temp)
		}
	} else {
		dirs = append(dirs, model.Path{Name: "DATA", Path: "/DATA/", IsDir: true, Date: time.Now()})
	}
	return dirs, nil
}

func (c *systemService) GetCpuInfo() []cpu.InfoStat {
	info, _ := cpu.Info()
	return info
}

func (c *systemService) GetMemInfo() map[string]interface{} {
	memInfo, _ := mem.VirtualMemory()
	memInfo.UsedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.1f", memInfo.UsedPercent), 64)
	memData := make(map[string]interface{})
	memData["total"] = memInfo.Total
	memData["available"] = memInfo.Available
	memData["used"] = memInfo.Used
	memData["free"] = memInfo.Free
	memData["usedPercent"] = memInfo.UsedPercent
	return memData
}

func (c *systemService) GetCpuPercent() float64 {
	percent, _ := cpu.Percent(0, false)
	value, _ := strconv.ParseFloat(fmt.Sprintf("%.1f", percent[0]), 64)
	return value
}

func (c *systemService) GetCpuCoreNum() int {
	count, _ := cpu.Counts(false)
	return count
}

func (c *systemService) GetNetInfo() []net.IOCountersStat {
	parts, _ := net.IOCounters(true)
	return parts
}

func (c *systemService) GetNet(physics bool) []string {
	t := "1"
	if physics {
		t = "2"
	}

	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;GetNetCard " + t); err != nil {
		return []string{}
	} else {
		return strings.Split(output, "\n")
	}
}

// UpdateSystemVersion was the inherited CasaOS self-update path.
// Removed in Sprint 5 #203 kill #1 — it `curl … | bash`'d from
// get.casaos.io/update (a real curl-pipe-bash from upstream
// CasaOS infra). PowerLab's in-app updater under
// /v1/powerlab-update/ uses the manifest.json + signed-tarball
// pipeline; the old path was dead AND dangerous.

func (s *systemService) UpdateAssist() {
	command.ExecResultStrArray("source " + config.AppInfo.ShellPath + "/assist.sh")
}

func (s *systemService) GetTimeZone() string {
	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;GetTimeZone"); err != nil {
		return ""
	} else {
		return strings.TrimSpace(output)
	}
}

func (s *systemService) SetTimeZone(timezone string) error {
	_, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;SetTimeZone " + timezone)
	return err
}

func (s *systemService) GetSystemConfigDebug() []string {
	if output, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/helper.sh ;GetSysInfo"); err != nil {
		return []string{}
	} else {
		return strings.Split(output, "\n")
	}
}

func (s *systemService) UpAppOrderFile(str, id string) {
	file.WriteToPath([]byte(str), config.AppInfo.DBPath+"/"+id, "app_order.json")
}

func (s *systemService) GetAppOrderFile(id string) []byte {
	return file.ReadFullFile(config.AppInfo.UserDataPath + "/" + id + "/app_order.json")
}

func (s *systemService) UpSystemPort(port string) {
	if len(port) > 0 && port != config.ServerInfo.HttpPort {
		config.Cfg.Section("server").Key("HttpPort").SetValue(port)
		config.ServerInfo.HttpPort = port
	}
	config.Cfg.SaveTo(config.SystemConfigInfo.ConfigPath)
}

func (s *systemService) GetSystemLogs(lineNumber int) string {
	if lineNumber <= 0 {
		lineNumber = 100
	}
	logPath := filepath.Join(config.AppInfo.LogPath, fmt.Sprintf("%s.%s",
		config.AppInfo.LogSaveName,
		config.AppInfo.LogFileExt,
	))
	file, err := os.Open(logPath)
	if err != nil {
		return err.Error()
	}
	defer file.Close()

	// Original implementation read the whole file and returned it,
	// which on a long-running server meant /v1/sys/logs returned MBs
	// of historical noise (including stack traces from panics that
	// happened on an earlier boot, long since recovered) and made the
	// UI show stale errors as if they were current. Tail the last N
	// lines instead — the parameter was already plumbed through, just
	// never honoured.
	stat, err := file.Stat()
	if err != nil {
		return err.Error()
	}
	const chunkSize int64 = 16 * 1024
	var (
		buf       []byte
		offset    = stat.Size()
		newlines  = 0
		startFrom int64
	)
	for offset > 0 && newlines <= lineNumber {
		read := chunkSize
		if offset < chunkSize {
			read = offset
		}
		offset -= read
		chunk := make([]byte, read)
		if _, err := file.ReadAt(chunk, offset); err != nil {
			return err.Error()
		}
		buf = append(chunk, buf...)
		newlines = 0
		for _, b := range buf {
			if b == '\n' {
				newlines++
			}
		}
		startFrom = offset
		if startFrom == 0 {
			break
		}
	}
	// Drop everything before the (lineNumber)th-last newline so the
	// caller gets exactly the requested tail.
	if newlines > lineNumber {
		extra := newlines - lineNumber
		for i, b := range buf {
			if b == '\n' {
				extra--
				if extra == 0 {
					buf = buf[i+1:]
					break
				}
			}
		}
	}
	return string(buf)
}

// find thermal_zone of cpu.
// assertions:
//   - thermal_zone "type" and "temp" are required fields
//     (https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-class-thermal)
//
// GetCPUThermalZone reads /sys/class/thermal/thermal_zone* and
// returns the first zone whose type matches the CPU. Linux-only;
// returns "" on macOS dev installs.
func GetCPUThermalZone() string {
	keyName := "cpu_thermal_zone"

	var path string
	if result, ok := Cache.Get(keyName); ok {
		path, ok = result.(string)
		if ok {
			return path
		}
	}

	var name string
	cpu_types := []string{"x86_pkg_temp", "cpu", "CPU", "soc"}
	stub := "/sys/devices/virtual/thermal/thermal_zone"
	for i := 0; i < 100; i++ {
		path = stub + strconv.Itoa(i)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			name = strings.TrimSuffix(string(file.ReadFullFile(path+"/type")), "\n")
			for _, s := range cpu_types {
				if strings.HasPrefix(name, s) {
					//logger.Info(fmt.Sprintf("CPU thermal zone found: %s, path: %s.", name, path))
					Cache.SetDefault(keyName, path)
					return path
				}
			}
		} else {
			if len(name) > 0 { // proves at least one zone
				path = stub + "0"
			} else {
				path = ""
			}
			break
		}
	}

	Cache.SetDefault(keyName, path)
	return path
}

func (s *systemService) GetCPUTemperature() int {
	outPut := ""
	path := GetCPUThermalZone()
	if len(path) > 0 {
		outPut = string(file.ReadFullFile(path + "/temp"))
	} else {
		outPut = string(file.ReadFullFile("/sys/class/hwmon/hwmon0/temp1_input"))
		if len(outPut) == 0 {
			outPut = "0"
		}
	}

	celsius, _ := strconv.Atoi(strings.TrimSpace(outPut))

	if celsius > 1000 {
		celsius = celsius / 1000
	}
	return celsius
}

func (s *systemService) GetCPUPower() map[string]string {
	data := make(map[string]string, 2)
	data["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)
	if file.Exists("/sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj") {
		data["value"] = strings.TrimSpace(string(file.ReadFullFile("/sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj")))
	} else {
		data["value"] = "0"
	}
	return data
}

func (s *systemService) SystemReboot() error {
	arg := []string{"6"}
	cmd := exec2.Command("init", arg...)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}

func (s *systemService) SystemShutdown() error {
	arg := []string{"0"}
	cmd := exec2.Command("init", arg...)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}

func (s *systemService) GetNetworkInterfaces() []model.NetworkInterface {
	interfaces, err := net2.Interfaces()
	if err != nil {
		return []model.NetworkInterface{}
	}

	allIpv4 := ip_helper.GetDeviceAllIPv4()
	physicalNets := s.GetNet(true)
	virtualNets := s.GetNet(false)

	var result []model.NetworkInterface
	for _, v := range interfaces {
		// Skip loopback
		if v.Flags&net2.FlagLoopback != 0 {
			continue
		}

		iface := model.NetworkInterface{
			Name: v.Name,
			MAC:  v.HardwareAddr.String(),
			IP:   allIpv4[v.Name],
		}

		// Determine type
		iface.Type = "unknown"
		for _, n := range physicalNets {
			if v.Name == n {
				iface.Type = "physical"
				break
			}
		}
		if iface.Type == "unknown" {
			for _, n := range virtualNets {
				if v.Name == n {
					iface.Type = "virtual"
					break
				}
			}
		}

		// Get state
		iface.State = s.GetNetState(v.Name)
		if iface.State == "" {
			if v.Flags&net2.FlagUp != 0 {
				iface.State = "up"
			} else {
				iface.State = "down"
			}
		}

		result = append(result, iface)
	}

	return result
}

func (s *systemService) GetSystemUsers() []model.SystemUser {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return []model.SystemUser{}
	}

	var result []model.SystemUser
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			uid, _ := strconv.Atoi(parts[2])
			// Only include real users (UID >= 1000) or special ones we care about
			if uid >= 1000 || parts[0] == "root" {
				result = append(result, model.SystemUser{
					Username: parts[0],
					UID:      parts[2],
					GID:      parts[3],
					HomeDir:  parts[5],
					Shell:    parts[6],
				})
			}
		}
	}
	return result
}

// NewSystemService returns a stateless SystemService.
func NewSystemService() SystemService {
	return &systemService{}
}
