package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
	http2 "github.com/neochaotic/powerlab/backend/common/utils/http"
	"github.com/neochaotic/powerlab/backend/common/utils/port"
	"github.com/neochaotic/powerlab/backend/core/common"
	"github.com/neochaotic/powerlab/backend/core/model"
	"github.com/neochaotic/powerlab/backend/core/pkg/config"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils"
	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/version"
	"github.com/neochaotic/powerlab/backend/core/service"
	model2 "github.com/neochaotic/powerlab/backend/core/service/model"
	"github.com/neochaotic/powerlab/backend/core/types"
	"github.com/labstack/echo/v4"
	"github.com/tidwall/gjson"
)

// @Summary check version
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/version/check [get]
func GetSystemCheckVersion(ctx echo.Context) error {
	need, version := version.IsNeedUpdate(service.MyService.Casa().GetCasaosVersion())
	if need {
		installLog := model2.AppNotify{}
		installLog.State = 0
		installLog.Message = "New version " + version.Version + " is ready, ready to upgrade"
		installLog.Type = types.NOTIFY_TYPE_NEED_CONFIRM
		installLog.CreatedAt = strconv.FormatInt(time.Now().Unix(), 10)
		installLog.UpdatedAt = strconv.FormatInt(time.Now().Unix(), 10)
		installLog.Name = "CasaOS System"
		service.MyService.Notify().AddLog(installLog)
	}
	data := make(map[string]interface{}, 3)
	data["need_update"] = need
	data["version"] = version
	data["current_version"] = common.VERSION
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: data})
}

// @Summary 系统信息
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/update [post]
func SystemUpdate(ctx echo.Context) error {
	need, version := version.IsNeedUpdate(service.MyService.Casa().GetCasaosVersion())
	if need {
		service.MyService.System().UpdateSystemVersion(version.Version)
	}
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// @Summary  get logs
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/error/logs [get]
func GetCasaOSErrorLogs(ctx echo.Context) error {
	line, _ := strconv.Atoi(utils.DefaultQuery(ctx, "line", "100"))
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: service.MyService.System().GetCasaOSLogs(line)})
}

// 系统配置
func GetSystemConfigDebug(ctx echo.Context) error {
	array := service.MyService.System().GetSystemConfigDebug()
	disk := service.MyService.System().GetDiskInfo()
	sys := service.MyService.System().GetSysInfo()
	version := service.MyService.Casa().GetCasaosVersion()
	var bugContent string = fmt.Sprintf(`
	 - OS: %s
	 - CasaOS Version: %s
	 - Disk Total: %v 
	 - Disk Used: %v 
	 - System Info: %s
	 - Remote Version: %s
	 - Browser: $Browser$ 
	 - Version: $Version$
`, sys.OS, common.VERSION, disk.Total>>20, disk.Used>>20, array, version.Version)

	//	array = append(array, fmt.Sprintf("disk,total:%v,used:%v,UsedPercent:%v", disk.Total>>20, disk.Used>>20, disk.UsedPercent))

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: bugContent})
}

// @Summary get casaos server port
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/port [get]
func GetCasaOSPort(ctx echo.Context) error {
	return ctx.JSON(common_err.SUCCESS,
		model.Result{
			Success: common_err.SUCCESS,
			Message: common_err.GetMsg(common_err.SUCCESS),
			Data:    config.ServerInfo.HttpPort,
		})
}

// @Summary edit casaos server port
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Param port json string true "port"
// @Success 200 {string} string "ok"
// @Router /sys/port [put]
func PutCasaOSPort(ctx echo.Context) error {
	json := make(map[string]string)
	ctx.Bind(&json)
	portStr := json["port"]
	portNumber, err := strconv.Atoi(portStr)
	if err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR,
			model.Result{
				Success: common_err.SERVICE_ERROR,
				Message: err.Error(),
			})
	}

	isAvailable := port.IsPortAvailable(portNumber, "tcp")
	if !isAvailable {
		return ctx.JSON(common_err.SERVICE_ERROR,
			model.Result{
				Success: common_err.PORT_IS_OCCUPIED,
				Message: common_err.GetMsg(common_err.PORT_IS_OCCUPIED),
			})
	}
	service.MyService.System().UpSystemPort(strconv.Itoa(portNumber))
	return ctx.JSON(common_err.SUCCESS,
		model.Result{
			Success: common_err.SUCCESS,
			Message: common_err.GetMsg(common_err.SUCCESS),
		})
}

// @Summary active killing casaos
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/restart [post]
func PostKillCasaOS(ctx echo.Context) error {
	os.Exit(0)
	return nil
}

// @Summary get system hardware info
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/hardware/info [get]
func GetSystemHardwareInfo(ctx echo.Context) error {
	data := make(map[string]string, 1)
	data["drive_model"] = service.MyService.System().GetDeviceTree()
	data["arch"] = runtime.GOARCH

	if cpu := service.MyService.System().GetCpuInfo(); len(cpu) > 0 {
		return ctx.JSON(common_err.SUCCESS,
			model.Result{
				Success: common_err.SUCCESS,
				Message: common_err.GetMsg(common_err.SUCCESS),
				Data:    data,
			})
	}
	return nil
}

// @Summary system utilization
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/utilization [get]
func GetSystemUtilization(ctx echo.Context) error {
	data := make(map[string]interface{})
	cpu := service.MyService.System().GetCpuPercent()
	num := service.MyService.System().GetCpuCoreNum()
	cpuModel := "arm"
	if cpu := service.MyService.System().GetCpuInfo(); len(cpu) > 0 {
		if strings.Count(strings.ToLower(strings.TrimSpace(cpu[0].ModelName)), "intel") > 0 {
			cpuModel = "intel"
		} else if strings.Count(strings.ToLower(strings.TrimSpace(cpu[0].ModelName)), "amd") > 0 {
			cpuModel = "amd"
		}
	}
	cpuData := make(map[string]interface{})
	cpuData["percent"] = cpu
	cpuData["num"] = num
	cpuData["temperature"] = service.MyService.System().GetCPUTemperature()
	cpuData["power"] = service.MyService.System().GetCPUPower()
	cpuData["model"] = cpuModel

	data["cpu"] = cpuData
	data["mem"] = service.MyService.System().GetMemInfo()

	// 拼装网络信息
	netList := service.MyService.System().GetNetInfo()
	newNet := []model.IOCountersStat{}
	nets := service.MyService.System().GetNet(true)
	for _, n := range netList {
		for _, netCardName := range nets {
			if n.Name == netCardName {
				item := *(*model.IOCountersStat)(unsafe.Pointer(&n))
				item.State = strings.TrimSpace(service.MyService.System().GetNetState(n.Name))
				item.Time = time.Now().Unix()
				newNet = append(newNet, item)
				break
			}
		}
	}

	if len(newNet) == 0 {
		for _, n := range netList {
			item := *(*model.IOCountersStat)(unsafe.Pointer(&n))
			newNet = append(newNet, item)
		}
	}
	data["net"] = newNet

	// Add OS Info
	osInfo := make(map[string]interface{})
	if out, err := exec.Command("hostname").Output(); err == nil {
		osInfo["hostname"] = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("uname", "-sr").Output(); err == nil {
		osInfo["kernel"] = strings.TrimSpace(string(out))
	}
	// Simple uptime (seconds)
	if out, err := exec.Command("uptime").Output(); err == nil {
		osInfo["uptime_str"] = strings.TrimSpace(string(out))
	}
	data["os"] = osInfo

	if gpuUtil := external.GetGPUUtilization(); gpuUtil != nil {
		data["gpu"] = gpuUtil
	}

	systemMap := service.MyService.Notify().GetSystemTempMap()
	systemMap.Range(func(key, value interface{}) bool {
		data[key.(string)] = value
		return true
	})
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: data})
}

// @Summary get cpu info
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/cpu [get]
func GetSystemCupInfo(ctx echo.Context) error {
	cpu := service.MyService.System().GetCpuPercent()
	num := service.MyService.System().GetCpuCoreNum()
	data := make(map[string]interface{})
	data["percent"] = cpu
	data["num"] = num
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: data})
}

// @Summary get mem info
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/mem [get]
func GetSystemMemInfo(ctx echo.Context) error {
	mem := service.MyService.System().GetMemInfo()
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: mem})
}

// @Summary get disk info
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/disk [get]
func GetSystemDiskInfo(ctx echo.Context) error {
	disk := service.MyService.System().GetDiskInfo()
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: disk})
}

// @Summary get Net info
// @Produce  application/json
// @Accept application/json
// @Tags sys
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /sys/net [get]
func GetSystemNetInfo(ctx echo.Context) error {
	netList := service.MyService.System().GetNetInfo()
	newNet := []model.IOCountersStat{}
	for _, n := range netList {
		for _, netCardName := range service.MyService.System().GetNet(true) {
			if n.Name == netCardName {
				item := *(*model.IOCountersStat)(unsafe.Pointer(&n))
				item.State = strings.TrimSpace(service.MyService.System().GetNetState(n.Name))
				item.Time = time.Now().Unix()
				newNet = append(newNet, item)
				break
			}
		}
	}

	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: newNet})
}

func GetSystemProxy(ctx echo.Context) error {
	url := ctx.QueryParam("url")
	resp, err := http2.Get(url, 30*time.Second)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	defer resp.Body.Close()
	for k, v := range ctx.Request().Header {
		ctx.Request().Header.Add(k, v[0])
	}
	rda, _ := ioutil.ReadAll(resp.Body)
	//	json.NewEncoder(c.Writer).Encode(json.RawMessage(string(rda)))
	// 响应状态码
	ctx.Response().Writer.WriteHeader(resp.StatusCode)
	// 复制转发的响应Body到响应Body
	io.Copy(ctx.Response().Writer, ioutil.NopCloser(bytes.NewBuffer(rda)))
	return nil
}

func PutSystemState(ctx echo.Context) error {
	state := ctx.Param("state")
	if strings.ToLower(state) == "off" {
		service.MyService.System().SystemShutdown()
	} else if strings.ToLower(state) == "restart" {
		service.MyService.System().SystemReboot()
	}
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: "The operation will be completed shortly."})
}

// @Summary 获取一个可用端口
// @Produce  application/json
// @Accept application/json
// @Tags app
// @Param  type query string true "端口类型 udp/tcp"
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /app/getport [get]
func GetPort(ctx echo.Context) error {
	t := utils.DefaultQuery(ctx, "type", "tcp")
	var p int
	ok := true
	for ok {
		p, _ = port.GetAvailablePort(t)
		ok = !port.IsPortAvailable(p, t)
	}
	// @tiger 这里最好封装成 {'port': ...} 的形式，来体现出参的上下文
	return ctx.JSON(common_err.SUCCESS, &model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: p})
}

// @Summary 检查端口是否可用
// @Produce  application/json
// @Accept application/json
// @Tags app
// @Param  port path int true "端口号"
// @Param  type query string true "端口类型 udp/tcp"
// @Security ApiKeyAuth
// @Success 200 {string} string "ok"
// @Router /app/check/{port} [get]
func PortCheck(ctx echo.Context) error {
	p, _ := strconv.Atoi(ctx.Param("port"))
	t := utils.DefaultQuery(ctx, "type", "tcp")
	return ctx.JSON(common_err.SUCCESS, &model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: port.IsPortAvailable(p, t)})
}

func GetSystemEntry(ctx echo.Context) error {
	entry := service.MyService.System().GetSystemEntry()
	str := json.RawMessage(entry)
	if !gjson.ValidBytes(str) {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: entry, Data: json.RawMessage("[]")})
	}
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: str})
}

func GetSystemTimeZone(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: service.MyService.System().GetTimeZone()})
}

func PutSystemTimeZone(ctx echo.Context) error {
	json := make(map[string]string)
	ctx.Bind(&json)
	timezone := json["timezone"]
	if timezone == "" {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.CLIENT_ERROR, Message: "timezone is required"})
	}
	if err := service.MyService.System().SetTimeZone(timezone); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}
func GetNetworkInterfaces(ctx echo.Context) error {
	interfaces := service.MyService.System().GetNetworkInterfaces()
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    interfaces,
	})
}

func GetSystemUsers(ctx echo.Context) error {
	users := service.MyService.System().GetSystemUsers()
	return ctx.JSON(http.StatusOK, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    users,
	})
}
