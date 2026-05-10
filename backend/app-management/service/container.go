package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/model"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/utils/envHelper"
	v1 "github.com/neochaotic/powerlab/backend/app-management/service/v1"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/neochaotic/powerlab/backend/common/utils/random"
	timeutils "github.com/neochaotic/powerlab/backend/common/utils/time"

	//"github.com/containerd/containerd/oci"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	client2 "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

var (
	dataStats = &sync.Map{}
	isFinish  bool

	// NewVersionApp is the in-memory map of "app ID → newer version
	// available" populated by a background goroutine. Read by the
	// container-list code path to set the `latest` flag per app.
	NewVersionApp map[string]string
)

type DockerService interface {
	// image
	IsExistImage(imageName string) bool
	PullImage(ctx context.Context, imageName string) error
	PullLatestImage(ctx context.Context, imageName string) (bool, error)
	RemoveImage(name string) error

	// container
	CheckContainerHealth(id string) (bool, error)
	CreateContainer(m model.CustomizationPostData, id string) (containerID string, err error)
	CreateContainerShellSession(container, row, col string) (hr types.HijackedResponse, err error)
	DescribeContainer(ctx context.Context, name string) (*types.ContainerJSON, error)
	GetContainer(id string) (types.Container, error)
	GetContainerAppList(name, image, state *string) (*[]model.MyAppList, *[]model.MyAppList)
	GetContainerByName(name string) (*types.Container, error)
	GetContainerLog(name string) ([]byte, error)
	GetContainerStats() []model.DockerStatsModel
	RecreateContainer(ctx context.Context, id string, pull bool, force bool) error
	RemoveContainer(name string, update bool) error
	RenameContainer(name, id string) (err error)
	StartContainer(name string) error
	StopContainer(id string) error

	// network
	GetNetworkList() []types.NetworkResource

	// docker server
	GetServerInfo() (types.Info, error)
}

type dockerService struct{}

// FIXME - should use WebSocket or SocketIO instead of HTTP polling (tiger)
func getContainerStats() {
	cli, err := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		logger.Error("Failed to get container_list", zap.Any("err", err))
	}
	for i := 0; i < 100; i++ {
		if i%10 == 0 {
			containers, err = cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
			if err != nil {
				logger.Error("Failed to get container_list", zap.Any("err", err))
				continue
			}
		}
		if config.AppLifecycleFlags.AppChange {
			config.AppLifecycleFlags.AppChange = false
			dataStats.Range(func(key, value interface{}) bool {
				dataStats.Delete(key)
				return true
			})
		}

		var temp sync.Map
		var wg sync.WaitGroup
		for _, v := range containers {
			if v.State != "running" {
				continue
			}
			wg.Add(1)
			go func(v types.Container, i int) {
				defer wg.Done()
				stats, err := cli.ContainerStatsOneShot(ctx, v.ID)
				if err != nil {
					return
				}
				decoder := json.NewDecoder(stats.Body)

				// data
				var data interface{}
				if err := decoder.Decode(&data); err == io.EOF {
					return
				}
				m, _ := dataStats.Load(v.ID)
				dockerStats := model.DockerStatsModel{}
				if m != nil {
					dockerStats.Previous = m.(model.DockerStatsModel).Data
				}

				// icon
				if icon, ok := v.Labels[v1.V1LabelIcon]; ok {
					dockerStats.Icon = icon
				}

				dockerStats.Data = data
				dockerStats.Title = strings.ReplaceAll(v.Names[0], "/", "")

				// @tiger - 不建议直接把依赖的数据结构封装返回。
				//          如果依赖的数据结构有变化，应该在这里适配或者保存，这样更加对客户端负责
				temp.Store(v.ID, dockerStats)
				if i == 99 {
					stats.Body.Close()
				}
			}(v, i)
		}
		wg.Wait()
		dataStats = &temp
		isFinish = true

		time.Sleep(time.Second * 1)
	}
	isFinish = false
	cancel()
}

func (ds *dockerService) GetContainerStats() []model.DockerStatsModel {
	stream := true
	for !isFinish {
		if stream {
			stream = false
			go getContainerStats()
		}
		runtime.Gosched()
	}
	list := []model.DockerStatsModel{}

	dataStats.Range(func(key, value interface{}) bool {
		list = append(list, value.(model.DockerStatsModel))
		return true
	})
	return list
}

func (ds *dockerService) CheckContainerHealth(id string) (bool, error) {
	container, err := ds.GetContainer(id)
	if err != nil {
		logger.Error("failed to get container by id", zap.Error(err), zap.String("id", id))
		return false, err
	}

	if webUIPort := common.LabelValue(container.Labels, common.LabelWebPortKey); webUIPort != "" {
		index := common.LabelValue(container.Labels, common.LabelWebIndexKey)
		url := fmt.Sprintf("http://%s:%s/%s", common.Localhost, webUIPort, strings.TrimLeft(index, "/"))

		logger.Info("checking container health at the specified web port...", zap.Any("name", container.Names), zap.String("id", id), zap.Any("url", url))
		client := resty.New()
		client.SetTimeout(30 * time.Second)
		client.SetHeader("Accept", "text/html")
		response, err := client.R().Get(url)
		if err != nil {
			logger.Error("failed to check container health", zap.Error(err), zap.Any("name", container.Names), zap.String("id", id))
			return false, err
		}
		if response.StatusCode() == http.StatusOK || response.StatusCode() == http.StatusUnauthorized {
			return true, nil
		}
		// response, err := httpUtil.GetWithHeader(url, 30*time.Second, map[string]string{
		// 	echo.HeaderAccept: echo.MIMETextHTML, // emulate a browser
		// })
		// if err != nil {
		// 	logger.Error("failed to check container health", zap.Error(err), zap.Any("name", container.Names), zap.String("id", id))
		// 	return false, err
		// }

		// if (response.StatusCode == http.StatusUnauthorized) || // we treat Unauthorized as a success because it means the container is up and running
		// 	(response.StatusCode >= 200 && response.StatusCode < 300) {
		// 	logger.Info("container health check passed at the specified web port", zap.Any("name", container.Names), zap.String("id", id), zap.Any("url", url))
		// 	return true, nil
		// }

		logger.Error("container health check failed at the specified web port", zap.Any("name", container.Names), zap.String("id", id), zap.Any("url", url), zap.String("status", fmt.Sprint(response.StatusCode())))
		return false, errors.New(fmt.Sprint(response.StatusCode()))
	}

	logger.Error("container health check failed, no web port specified", zap.Any("name", container.Names), zap.String("id", id))
	return false, errors.New("no web port")
}

// 获取我的应用列表
func (ds *dockerService) GetContainer(id string) (types.Container, error) {
	// 获取docker应用
	cli, err := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation())
	if err != nil {
		logger.Error("Failed to init client", zap.Any("err", err))
		return types.Container{}, err
	}
	defer cli.Close()

	filters := filters.NewArgs()
	filters.Add("id", id)
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true, Filters: filters})
	if err != nil {
		logger.Error("Failed to get container_list", zap.Any("err", err))
		return types.Container{}, err
	}

	if len(containers) > 0 {
		return containers[0], nil
	}
	return types.Container{}, nil
}

// 获取我的应用列表
func (ds *dockerService) GetContainerAppList(name, image, state *string) (*[]model.MyAppList, *[]model.MyAppList) {
	cli, err := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation(), client2.WithTimeout(time.Second*5))
	if err != nil {
		logger.Error("Failed to init client", zap.Any("err", err))
	}
	defer cli.Close()
	// fts := filters.NewArgs()
	// fts.Add("label", "casaos=casaos")
	// fts.Add("label", "casaos")
	// fts.Add("casaos", "casaos")
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		logger.Error("Failed to get container_list", zap.Any("err", err))
	}
	// 获取本地数据库应用

	localApps := []model.MyAppList{}

	managedApps := []model.MyAppList{}

	for i, m := range containers {

		if name != nil && len(*name) > 0 {
			if !lo.ContainsBy(m.Names, func(n string) bool { return strings.Contains(n, *name) }) {
				continue
			}
		}

		if image != nil && len(*image) > 0 {
			if !strings.HasPrefix(m.Image, *image) {
				continue
			}
		}

		if state != nil && len(*state) > 0 {
			if m.State != *state {
				continue
			}
		}

		if common.IsPowerLabApp(m.Labels) {

			_, newVersion := NewVersionApp[m.ID]
			name := strings.ReplaceAll(m.Names[0], "/", "")
			icon := common.LabelValue(m.Labels, common.LabelIconKey)
			if customName := common.LabelValue(m.Labels, common.LabelNameKey); customName != "" {
				name = customName
			}
			if common.LabelValue(m.Labels, common.LabelOriginKey) == "system" {
				name = strings.Split(m.Image, ":")[0]
				// icon synthesis used to call
				// https://icon.casaos.io/main/all/<image>.png for
				// system-origin containers. Removed in Sprint 5 #203
				// kill #9 — per ADR-0022, no runtime dependencies on
				// CasaOS infra. System-origin containers now fall
				// through to whatever icon the container itself
				// supplies (or the UI's MyAppList fallback if
				// nothing). When PowerLab ships its own icon CDN
				// or embedded SVG library, this is the place to
				// wire it in.
			}

			managedApp := model.MyAppList{
				Name:       name,
				Icon:       icon,
				State:      m.State,
				CustomID:   common.LabelValue(m.Labels, common.LabelCustomIDKey),
				ID:         m.ID,
				Port:       common.LabelValue(m.Labels, common.LabelWebPortKey),
				Index:      common.LabelValue(m.Labels, common.LabelWebIndexKey),
				Image:      m.Image,
				Latest:     newVersion,
				Host:       common.LabelValue(m.Labels, common.LabelHostKey),
				Protocol:   common.LabelValue(m.Labels, common.LabelProtocolKey),
				Created:    m.Created,
				AppStoreID: getV1AppStoreID(&containers[i]),
			}

			managedApps = append(managedApps, managedApp)
		} else {
			localApp := model.MyAppList{
				Name:     strings.ReplaceAll(m.Names[0], "/", ""),
				Icon:     "",
				State:    m.State,
				CustomID: m.ID,
				ID:       m.ID,
				Port:     "",
				Latest:   false,
				Host:     "",
				Protocol: "",
				Image:    m.Image,
				Created:  m.Created,
			}

			localApps = append(localApps, localApp)
		}
	}

	return &managedApps, &localApps
}

func (ds *dockerService) CreateContainerShellSession(container, row, col string) (types.HijackedResponse, error) {
	cli, err := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation())
	if err != nil {
		return types.HijackedResponse{}, err
	}

	ctx := context.Background()
	ir, err := cli.ContainerExecCreate(ctx, container, types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Env:          []string{"COLUMNS=" + col, "LINES=" + row},
		Cmd:          []string{"/bin/sh"},
		Tty:          true,
	})
	if err != nil {
		return types.HijackedResponse{}, err
	}

	return cli.ContainerExecAttach(ctx, ir.ID, types.ExecStartCheck{Detach: false, Tty: true})
}

// 正式内容

// param imageName 镜像名称
// param containerDbId 数据库的id
// param port 容器内部主端口
// param mapPort 容器主端口映射到外部的端口
// param tcp 容器其他tcp端口
// param udp 容器其他udp端口
// CreateContainer builds a docker container spec from the V1 install
// form's CustomizationPostData and creates the container. Spec
// building is delegated to the `build*` helpers in
// container_helpers.go (Sprint 7 #5 extraction); this function is
// the orchestration body.
//
// When id refers to an existing PowerLab-managed container, the
// existing host config + non-app fields are reused (preserves
// hand-tweaks on edit); the app-managed fields are always
// overwritten from m.
func (ds *dockerService) CreateContainer(m model.CustomizationPostData, id string) (containerID string, err error) {
	if len(m.NetworkModel) == 0 {
		m.NetworkModel = "bridge"
	}

	cli, err := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer cli.Close()

	ports, portMaps, err := buildPortBindings(m.Ports, m.NetworkModel)
	if err != nil {
		return "", err
	}

	envArr, showENV := buildEnvVars(m.Envs, m.PortMap)
	res := buildContainerResources(m)
	volumes, _ := buildVolumeMounts(m.Volumes, m.Label)

	rp := container.RestartPolicy{}
	if len(m.Restart) > 0 {
		rp.Name = m.Restart
	}
	if len(m.HostName) == 0 {
		m.HostName = m.Label
	}

	info, err := cli.ContainerInspect(context.Background(), id)
	hostConfig := &container.HostConfig{}
	config := &container.Config{}
	config.Labels = map[string]string{}
	if err == nil {
		hostConfig = info.HostConfig
		config = info.Config
		if common.IsPowerLabApp(config.Labels) {
			config.Cmd = m.Cmd
			config.Image = m.Image
			config.Env = envArr
			config.Hostname = m.HostName
			config.ExposedPorts = ports
		}
	} else {
		config.Cmd = m.Cmd
		config.Image = m.Image
		config.Env = envArr
		config.Hostname = m.HostName
		config.ExposedPorts = ports
	}

	// Per ADR-0021: dual-write canonical io.powerlab.v1.* + legacy
	// unnamespaced labels for one release window. common.BuildLabels
	// is the single source of truth — never write labels by hand here.
	for k, v := range common.BuildLabels(common.AppLabels{
		Origin:      m.Origin,
		WebPort:     m.PortMap,
		Icon:        m.Icon,
		Description: m.Description,
		WebIndex:    m.Index,
		CustomID:    m.CustomID,
		ShowEnv:     strings.Join(showENV, ","),
		Protocol:    m.Protocol,
		Host:        m.Host,
		Name:        m.Label,
		AppStoreID:  strconv.Itoa((int)(m.AppStoreID)),
	}) {
		config.Labels[k] = v
	}

	hostConfig.Mounts = volumes
	hostConfig.Binds = []string{}
	hostConfig.Privileged = m.Privileged
	hostConfig.CapAdd = m.CapAdd
	hostConfig.NetworkMode = container.NetworkMode(m.NetworkModel)
	hostConfig.RestartPolicy = rp
	hostConfig.Resources = res
	hostConfig.PortBindings = portMaps

	containerDb, err := cli.ContainerCreate(context.Background(),
		config,
		hostConfig,
		&network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{m.NetworkModel: {NetworkID: "", Aliases: []string{}}}},
		nil,
		m.ContainerName)
	if err != nil {
		return "", err
	}
	return containerDb.ID, err
}

// RecreateContainer is the in-place upgrade for a single container:
// pull latest image (when pull=true), clone the running container
// under a temp name, stop the old, start the new, remove the old,
// rename the new to the old's name. On any failure mid-flight,
// the old container is restarted to preserve uptime.
//
// force=false short-circuits when no image update was found —
// callers use force=true to re-create with the same image (e.g.
// after a config change).
//
// Each phase is wrapped via wrapContainerEvents (Sprint 7 #5
// extraction) — eliminates the IIFE-with-events boilerplate that
// repeated 6 times in the original.
func (ds *dockerService) RecreateContainer(ctx context.Context, id string, pull bool, force bool) error {
	containerInfo, err := docker.Container(ctx, id)
	if err != nil {
		return err
	}

	isImageUpdated := false
	if pull {
		imageName := docker.ImageName(containerInfo)
		if imageName != "" {
			_isImageUpdated, err := ds.PullLatestImage(ctx, imageName) // image update result will be included in ctx properties
			if err != nil {
				logger.Error("pull new image failed", zap.Error(err), zap.String("image", imageName))
			}
			isImageUpdated = _isImageUpdated
		}
	}

	if !force && !isImageUpdated {
		return nil
	}

	// Clone the old container under a temp name
	var newID string
	tempName := fmt.Sprintf("%s-%s", containerInfo.Name, random.RandomString(4, false))
	if err := wrapContainerEvents(ctx,
		common.EventTypeContainerCreateBegin,
		common.EventTypeContainerCreateEnd,
		common.EventTypeContainerCreateError,
		map[string]string{common.PropertyTypeContainerName.Name: tempName},
		func() error {
			_newID, err := docker.CloneContainer(ctx, id, tempName)
			if err != nil {
				return err
			}
			newID = _newID
			return nil
		}); err != nil {
		return err
	}

	// Stop old container if it is running
	if containerInfo.State.Running {
		if err := wrapContainerEvents(ctx,
			common.EventTypeContainerStopBegin,
			common.EventTypeContainerStopEnd,
			common.EventTypeContainerStopError,
			map[string]string{common.PropertyTypeContainerID.Name: id},
			func() error { return docker.StopContainer(ctx, id) }); err != nil {
			return err
		}
	}

	// Start new container
	startErr := wrapContainerEvents(ctx,
		common.EventTypeContainerStartBegin,
		common.EventTypeContainerStartEnd,
		common.EventTypeContainerStartError,
		map[string]string{common.PropertyTypeContainerID.Name: newID},
		func() error { return docker.StartContainer(ctx, newID) })

	if startErr != nil && containerInfo.State.Running {
		// Roll back: restart the old container, then remove the new one.
		if err := wrapContainerEvents(ctx,
			common.EventTypeContainerStartBegin,
			common.EventTypeContainerStartEnd,
			common.EventTypeContainerStartError,
			map[string]string{common.PropertyTypeContainerID.Name: id},
			func() error { return docker.StartContainer(ctx, id) }); err != nil {
			return err
		}
		if err := wrapContainerEvents(ctx,
			common.EventTypeContainerRemoveBegin,
			common.EventTypeContainerRemoveEnd,
			common.EventTypeContainerRemoveError,
			map[string]string{common.PropertyTypeContainerID.Name: newID},
			func() error { return docker.RemoveContainer(ctx, newID) }); err != nil {
			return err
		}
	}

	// Remove the old container (new one started successfully)
	if err := wrapContainerEvents(ctx,
		common.EventTypeContainerRemoveBegin,
		common.EventTypeContainerRemoveEnd,
		common.EventTypeContainerRemoveError,
		map[string]string{common.PropertyTypeContainerID.Name: containerInfo.ID},
		func() error { return docker.RemoveContainer(ctx, containerInfo.ID) }); err != nil {
		return err
	}

	// Rename the new container to the old name
	return wrapContainerEvents(ctx,
		common.EventTypeContainerRenameBegin,
		common.EventTypeContainerRenameEnd,
		common.EventTypeContainerRenameError,
		map[string]string{
			common.PropertyTypeContainerID.Name:   newID,
			common.PropertyTypeContainerName.Name: containerInfo.Name,
		},
		func() error { return docker.RenameContainer(ctx, newID, containerInfo.Name) })
}

// 删除容器
func (ds *dockerService) RemoveContainer(name string, update bool) error {
	ctx := context.Background()
	if err := docker.RemoveContainer(ctx, name); err != nil {
		return err
	}

	if update {
		return nil
	}

	// 路径处理
	if path := docker.GetDir(name, "/config"); !file.CheckNotExist(path) {
		return file.RMDir(path)
	}

	return nil
}

// 停止镜像
func (ds *dockerService) StopContainer(id string) error {
	ctx := context.Background()
	return docker.StopContainer(ctx, id)
}

// 启动容器
func (ds *dockerService) StartContainer(name string) error {
	ctx := context.Background()
	return docker.StartContainer(ctx, name)
}

// 查看日志
func (ds *dockerService) GetContainerLog(name string) ([]byte, error) {
	cli, err := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation())
	if err != nil {
		return []byte(""), err
	}
	defer cli.Close()
	// body, err := cli.ContainerAttach(context.Background(), name, types.ContainerAttachOptions{Logs: true, Stream: false, Stdin: false, Stdout: false, Stderr: false})
	body, err := cli.ContainerLogs(context.Background(), name, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return []byte(""), err
	}

	defer body.Close()
	content, err := io.ReadAll(body)
	if err != nil {
		return []byte(""), err
	}
	return content, nil
}

func (ds *dockerService) GetContainerByName(name string) (*types.Container, error) {
	cli, _ := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation())
	defer cli.Close()
	filter := filters.NewArgs()
	filter.Add("name", name)
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true, Filters: filter})
	if err != nil {
		return &types.Container{}, err
	}
	if len(containers) == 0 {
		return &types.Container{}, errors.New("not found")
	}
	return &containers[0], nil
}

// 获取容器详情
func (ds *dockerService) DescribeContainer(ctx context.Context, nameOrID string) (*types.ContainerJSON, error) {
	return docker.Container(ctx, nameOrID)
}

// 更新容器名称
// param name 容器名称
// param id 老的容器名称
func (ds *dockerService) RenameContainer(name, id string) (err error) {
	ctx := context.Background()
	return docker.RenameContainer(ctx, id, name)
}

// 获取网络列表
func (ds *dockerService) GetNetworkList() []types.NetworkResource {
	cli, _ := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation())
	defer cli.Close()
	networks, _ := cli.NetworkList(context.Background(), types.NetworkListOptions{})
	return networks
}

func (ds *dockerService) GetServerInfo() (types.Info, error) {
	cli, err := client2.NewClientWithOpts(client2.FromEnv, client2.WithAPIVersionNegotiation())
	if err != nil {
		return types.Info{}, err
	}
	defer cli.Close()

	return cli.Info(context.Background())
}

func NewDockerService() DockerService {
	return &dockerService{}
}

func getV1AppStoreID(m *types.Container) uint {
	if appStoreIDString := common.LabelValue(m.Labels, common.LabelAppStoreIDKey); appStoreIDString != "" {
		appStoreID, err := strconv.Atoi(appStoreIDString)
		if err != nil {
			logger.Info("failed to convert v1 app store id", zap.Error(err), zap.String("appStoreIDString", appStoreIDString), zap.String("containerID", m.ID), zap.String("containerName", m.Names[0]))
		}

		if appStoreID < 0 {
			appStoreID = 0
		}

		return uint(appStoreID)
	}

	logger.Info("the container does not have a v1 app store id", zap.String("containerID", m.ID), zap.String("containerName", m.Names[0]))
	return 0
}
