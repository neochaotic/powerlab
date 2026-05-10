package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/neochaotic/powerlab/backend/app-management/codegen/message_bus"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/model"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/utils/envHelper"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	timeutils "github.com/neochaotic/powerlab/backend/common/utils/time"
	"go.uber.org/zap"
)

// wrapContainerEvents fires a begin/end PublishEventWrapper pair
// around fn, and on fn error fires errType with the error message
// merged into props. Replaces the IIFE-with-events pattern that
// was repeated 6+ times in RecreateContainer (Sprint 7 #5).
//
// All event publishes are async (`go PublishEventWrapper`) — the
// helper preserves the original goroutine semantics, just without
// the boilerplate.
func wrapContainerEvents(
	ctx context.Context,
	beginType, endType, errType message_bus.EventType,
	props map[string]string,
	fn func() error,
) error {
	go PublishEventWrapper(ctx, beginType, props)

	defer PublishEventWrapper(ctx, endType, props)

	if err := fn(); err != nil {
		errProps := map[string]string{}
		for k, v := range props {
			errProps[k] = v
		}
		errProps[common.PropertyTypeMessage.Name] = err.Error()
		go PublishEventWrapper(ctx, errType, errProps)
		return err
	}
	return nil
}

// buildPortBindings translates the V1 install form's PortMap list
// into docker's nat.PortSet (exposed) + nat.PortMap (host bindings).
// Protocol "both" expands to ["tcp","udp"]. Host bindings are
// skipped when the network mode is "host" (the host already
// publishes everything in that mode).
//
// Returns an error when an unknown protocol is encountered — caller
// surfaces this as a 400 to the user.
func buildPortBindings(ports []model.PortMap, networkMode string) (nat.PortSet, nat.PortMap, error) {
	exposed := make(nat.PortSet)
	bindings := make(nat.PortMap)

	for _, portMap := range ports {
		protocol := strings.ToLower(portMap.Protocol)

		switch protocol {
		case "tcp", "udp", "both":
		default:
			logger.Error("unknown protocol", zap.String("protocol", protocol))
			return nil, nil, errors.New("unknown protocol")
		}

		protocols := strings.Replace(protocol, "both", "tcp,udp", -1)
		for _, p := range strings.Split(protocols, ",") {
			tContainer, _ := strconv.Atoi(portMap.ContainerPort)
			if tContainer > 0 {
				exposed[nat.Port(portMap.ContainerPort+"/"+p)] = struct{}{}
				if networkMode != "host" {
					bindings[nat.Port(portMap.ContainerPort+"/"+p)] = []nat.PortBinding{{HostPort: portMap.CommendPort}}
				}
			}
		}
	}

	return exposed, bindings, nil
}

// buildEnvVars renders the V1 install form's Env list into the
// docker env-var slice + the comma-joined "show env" label list
// the UI uses to hide system-managed vars from the env-edit panel.
//
// $-prefixed values get the system timezone substituted via the
// envHelper; "port_map" sentinel gets replaced with the form's
// PortMap value.
func buildEnvVars(envs []model.Env, portMap string) (envArr []string, showENV []string) {
	showENV = []string{"casaos"}

	for _, e := range envs {
		showENV = append(showENV, e.Name)
		if strings.HasPrefix(e.Value, "$") {
			systemTimeZoneName := timeutils.GetSystemTimeZoneName()
			envArr = append(envArr, e.Name+"="+envHelper.ReplaceDefaultENV(e.Value, systemTimeZoneName))
			continue
		}
		if len(e.Value) > 0 {
			if e.Value == "port_map" {
				envArr = append(envArr, e.Name+"="+portMap)
				continue
			}
			envArr = append(envArr, e.Name+"="+e.Value)
		}
	}
	return envArr, showENV
}

// buildContainerResources translates the V1 install form's CPU +
// memory + device limits into docker's container.Resources.
// Memory is shifted left 20 (MiB → bytes) to match the form's UI
// unit.
func buildContainerResources(m model.CustomizationPostData) container.Resources {
	res := container.Resources{}
	if m.CPUShares > 0 {
		res.CPUShares = m.CPUShares
	}
	if m.Memory > 0 {
		res.Memory = m.Memory << 20
	}
	for _, p := range m.Devices {
		if len(p.Path) > 0 {
			res.Devices = append(res.Devices, container.DeviceMapping{PathOnHost: p.Path, PathInContainer: p.ContainerPath, CgroupPermissions: "rwm"})
		}
	}
	return res
}

// buildVolumeMounts walks the V1 install form's Volumes list and
// returns docker bind-mount specs + the legacy host-config bind-
// strings. Auto-creates missing host directories (like
// `mkdir -p`) so the install doesn't fail on a fresh /DATA tree.
// Per-volume errors are logged + skipped; the install continues
// with the surviving volumes (matches the pre-extract behaviour).
//
// $AppID in the host path is substituted with label.
func buildVolumeMounts(volumes []model.PathMap, label string) ([]mount.Mount, []string) {
	mounts := []mount.Mount{}
	binds := []string{}
	for _, v := range volumes {
		path := v.Path
		if len(path) == 0 {
			path = docker.GetDir(label, v.Path)
			if len(path) == 0 {
				continue
			}
		}
		path = strings.ReplaceAll(path, "$AppID", label)
		if err := file.IsNotExistMkDir(path); err != nil {
			logger.Error("Failed to create a folder", zap.Any("err", err))
			continue
		}

		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: path,
			Target: v.ContainerPath,
		})
		binds = append(binds, v.Path+":"+v.ContainerPath)
	}
	return mounts, binds
}
