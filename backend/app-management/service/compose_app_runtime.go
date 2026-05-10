package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/types"
	"github.com/docker/compose/v2/cmd/formatter"
	"github.com/docker/compose/v2/pkg/api"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/go-resty/resty/v2"
	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// Containers returns the live docker-compose container summaries,
// keyed by service name. Used by the per-app stats card.
func (a *ComposeApp) Containers(ctx context.Context) (map[string][]api.ContainerSummary, error) {
	service, dockerClient, err := apiService()
	if err != nil {
		return nil, err
	}
	defer dockerClient.Close()

	containers, err := service.Ps(ctx, a.Name, api.PsOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	// it is possible a `service` contains multiple containers.
	// See https://docs.docker.com/compose/compose-file/deploy/#replicas
	return lo.GroupBy(containers, func(container api.ContainerSummary) string {
		return container.Service
	}), nil
}

// Pull pulls every image in the compose project. Streams docker
// pull progress to logWriter so the UI install-log view sees
// per-layer updates in real time.
func (a *ComposeApp) Pull(ctx context.Context, logWriter io.Writer) error {
	if logWriter == nil {
		logWriter = io.Discard
	}
	// pull
	serviceNum := len(a.Services)

	for i, app := range a.Services {
		if err := func() error {
			go PublishEventWrapper(ctx, common.EventTypeImagePullBegin, map[string]string{
				common.PropertyTypeImageName.Name: app.Image,
			})

			defer PublishEventWrapper(ctx, common.EventTypeImagePullEnd, map[string]string{
				common.PropertyTypeImageName.Name: app.Image,
			})

			fmt.Fprintf(logWriter, "Pulling %s (%d/%d)...\n", app.Image, i+1, serviceNum)

			if err := docker.PullImage(ctx, app.Image, func(out io.ReadCloser) {
				// We still want the percentage progress in message bus
				// But we also want the raw output in our logWriter

				// Create a pipe or just copy?
				// docker.PullImage output is JSON messages.
				// pullImageProgress handles the decoding.

				// For the logWriter, we'll write the raw JSON decoded messages for now,
				// or just a "Pulling..." message.

				// Actually, let's wrap pullImageProgress to also write to our logWriter.
				pullImageProgress(ctx, out, "INSTALL", serviceNum, i+1, logWriter)
			}); err != nil {
				fmt.Fprintf(logWriter, "Error pulling %s: %v\n", app.Image, err)
				go PublishEventWrapper(ctx, common.EventTypeImagePullError, map[string]string{
					common.PropertyTypeImageName.Name: app.Image,
					common.PropertyTypeMessage.Name:   err.Error(),
				})
			}

			return nil
		}(); err != nil {
			return err
		}
	}

	return nil
}

// Up runs `compose up` on the project — assumes images are
// already pulled. Use UpWithCheckRequire for the full lifecycle
// hook (memory + disk preflight checks).
func (a *ComposeApp) Up(ctx context.Context, service api.Service) error {
	a.injectEnvVariableToComposeApp()

	if err := service.Up(ctx, (*codegen.ComposeApp)(a), api.UpOptions{
		Start: api.StartOptions{
			CascadeStop: true,
			Wait:        true,
		},
	}); err != nil {
		logger.Error("failed to start original compose app", zap.Error(err), zap.String("name", a.Name))
		return err
	}
	return nil
}

// UpWithCheckRequire runs Up after the catalog's min-memory /
// min-disk preflight checks. Returns an error before pulling if
// the host can't satisfy the requirements.
func (a *ComposeApp) UpWithCheckRequire(ctx context.Context, service api.Service) error {
	// prepare source path for volumes if not exist
	for i, app := range a.Services {
		for _, volume := range app.Volumes {
			if _, ok := a.Volumes[volume.Source]; ok {
				// this is a internal volume, so skip.
				continue
			}

			path := volume.Source
			if err := file.IsNotExistMkDir(path); err != nil {
				go PublishEventWrapper(ctx, common.EventTypeContainerStartError, map[string]string{
					common.PropertyTypeMessage.Name: err.Error(),
				})
				return err
			}
		}

		// check if each required device exists
		deviceMapFiltered := []string{}
		for _, deviceMap := range app.Devices {
			devicePath := strings.SplitN(deviceMap, ":", 2)[0]
			if file.CheckNotExist(devicePath) {
				logger.Info("device not found", zap.String("device", devicePath))
				continue
			}
			deviceMapFiltered = append(deviceMapFiltered, deviceMap)
		}
		a.Services[i].Devices = deviceMapFiltered
	}

	if err := a.Up(ctx, service); err != nil {
		go PublishEventWrapper(ctx, common.EventTypeContainerStartError, map[string]string{
			common.PropertyTypeMessage.Name: err.Error(),
		})
		return err
	}
	return nil
}

// Create wraps compose-go's Create on the project — used by Up
// + UpWithCheckRequire as their first phase.
func (a *ComposeApp) Create(ctx context.Context, options api.CreateOptions, service api.Service) error {
	a.injectEnvVariableToComposeApp()
	return service.Create(ctx, (*codegen.ComposeApp)(a), api.CreateOptions{})
}

// Logs returns the last `lines` of combined service logs as raw
// bytes. lines < 0 returns the entire log buffer ("all"). Used by
// the per-app log viewer.
func (a *ComposeApp) Logs(ctx context.Context, lines int) ([]byte, error) {
	service, dockerClient, err := apiService()
	if err != nil {
		return nil, err
	}
	defer dockerClient.Close()

	var buf bytes.Buffer

	consumer := formatter.NewLogConsumer(ctx, &buf, &buf, false, true, false)

	if err := service.Logs(ctx, a.Name, consumer, api.LogOptions{
		Project:  (*codegen.ComposeApp)(a),
		Services: lo.Map(a.Services, func(s types.ServiceConfig, i int) string { return s.Name }),
		Follow:   false,
		Tail:     lo.If(lines < 0, "all").Else(strconv.Itoa(lines)),
	}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// HealthCheck issues an HTTP GET against the app's declared web
// port + index path and reports whether the response is 200 or
// 401 (auth-protected apps still count as healthy).
func (a *ComposeApp) HealthCheck() (bool, error) {
	storeInfo, err := a.StoreInfo(false)
	if err != nil {
		return false, err
	}

	scheme := "http"
	if storeInfo.Scheme != nil && *storeInfo.Scheme != "" {
		scheme = string(*storeInfo.Scheme)
	}

	hostname := common.Localhost
	if storeInfo.Hostname != nil && *storeInfo.Hostname != "" {
		hostname = *storeInfo.Hostname
	}

	url := fmt.Sprintf(
		"%s://%s:%s/%s",
		scheme,
		hostname,
		storeInfo.PortMap,
		strings.TrimLeft(storeInfo.Index, "/"),
	)

	logger.Info("checking compose app health at the specified web port...", zap.String("name", a.Name), zap.Any("url", url))

	client := resty.New()
	client.SetTimeout(30 * time.Second)
	client.SetHeader("Accept", "text/html")
	// ignore ssl error
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	response, err := client.R().Get(url)
	if err != nil {
		logger.Error("failed to check container health", zap.Error(err), zap.String("name", a.Name))
		return false, err
	}
	if response.StatusCode() == http.StatusOK || response.StatusCode() == http.StatusUnauthorized {
		return true, nil
	}

	logger.Error("compose app health check failed at the specified web port", zap.Any("name", a.Name), zap.Any("url", url), zap.String("status", fmt.Sprint(response.StatusCode())))
	return false, nil
}

// Stats aggregates per-container CPU/memory/network into a single
// compose-app-level snapshot. Skips containers not in the
// "running" state. Returned struct matches codegen.ComposeAppStats.
func (a *ComposeApp) Stats(ctx context.Context) (*codegen.ComposeAppStats, error) {
	_, dockerClient, err := apiService()
	if err != nil {
		return nil, err
	}
	defer dockerClient.Close()

	containers, err := a.Containers(ctx)
	if err != nil {
		return nil, err
	}

	var totalCPU float64
	var totalMemUsed int64
	var totalMemLimit int64
	var totalNetRx int64
	var totalNetTx int64

	for _, containerList := range containers {
		for _, c := range containerList {
			if c.State != "running" {
				continue
			}

			stats, err := dockerClient.ContainerStatsOneShot(ctx, c.ID)
			if err != nil {
				continue
			}

			var v dockerTypes.StatsJSON
			if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
				stats.Body.Close()
				continue
			}
			stats.Body.Close()

			// CPU %
			cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
			systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
			onlineCPUs := float64(v.CPUStats.OnlineCPUs)
			if onlineCPUs == 0.0 {
				onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
			}
			if systemDelta > 0.0 && cpuDelta > 0.0 {
				totalCPU += (cpuDelta / systemDelta) * onlineCPUs * 100.0
			}

			// Memory
			// For memory usage, we use (usage - cache) to match Docker CLI behavior
			cache := v.MemoryStats.Stats["inactive_file"]
			if cache == 0 {
				cache = v.MemoryStats.Stats["cache"]
			}
			totalMemUsed += int64(v.MemoryStats.Usage - cache)
			totalMemLimit += int64(v.MemoryStats.Limit)

			// Network
			for _, n := range v.Networks {
				totalNetRx += int64(n.RxBytes)
				totalNetTx += int64(n.TxBytes)
			}
		}
	}

	return &codegen.ComposeAppStats{
		CPUPercent:       totalCPU,
		MemoryUsedBytes:  totalMemUsed,
		MemoryLimitBytes: totalMemLimit,
		NetRxBytes:       totalNetRx,
		NetTxBytes:       totalNetTx,
	}, nil
}
