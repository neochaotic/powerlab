package main

import (
	"context"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/local-storage/common"
	"github.com/neochaotic/powerlab/backend/local-storage/model"
	"github.com/neochaotic/powerlab/backend/local-storage/service"
	"github.com/pilebones/go-udev/netlink"
)

func sendDiskBySocket() {
	blkList := service.MyService.Disk().LSBLK(true)

	status := model.DiskStatus{}
	healthy := true

	for _, currentDisk := range blkList {
		if !service.IsDiskSupported(currentDisk) {
			continue
		}
		temp := service.MyService.Disk().SmartCTL(currentDisk.Path)
		if reflect.DeepEqual(temp, model.SmartctlA{}) {
			healthy = true
		} else {
			if len(temp.ModelName) > 0 {
				healthy = temp.SmartStatus.Passed
			} else {
				healthy = true
			}
		}
		if len(currentDisk.Children) > 0 {
			for _, v := range currentDisk.Children {
				if len(v.MountPoint) > 0 {
					s, _ := strconv.ParseUint(v.FSSize.String(), 10, 64)
					a, _ := strconv.ParseUint(v.FSAvail.String(), 10, 64)
					u, _ := strconv.ParseUint(v.FSUsed.String(), 10, 64)
					status.Size += s
					status.Avail += a
					status.Used += u
				}
			}
		} else {
			if len(currentDisk.MountPoint) > 0 {
				s, _ := strconv.ParseUint(currentDisk.FSSize.String(), 10, 64)
				a, _ := strconv.ParseUint(currentDisk.FSAvail.String(), 10, 64)
				u, _ := strconv.ParseUint(currentDisk.FSUsed.String(), 10, 64)
				status.Size += s
				status.Avail += a
				status.Used += u
			}
		}
	}

	status.Health = healthy
	message := make(map[string]interface{})
	message["sys_disk"] = status
	if err := service.MyService.NotifySystem().SendSystemStatusNotify(message); err != nil {
		_log.Error(context.Background(), "failed to send notify", err, slog.Any("message", message))
	}
}

func sendUSBBySocket() {
	message := map[string]interface{}{
		"sys_usb": service.MyService.Disk().GetUSBDriveStatusList(),
	}

	if err := service.MyService.NotifySystem().SendSystemStatusNotify(message); err != nil {
		_log.Error(context.Background(), "failed to send notify", err, slog.Any("message", message))
	}
}

func monitorUEvent(ctx context.Context) {
	var matcher netlink.Matcher

	conn := new(netlink.UEventConn)
	if err := conn.Connect(netlink.UdevEvent); err != nil {
		_log.Error(ctx, "udev err: unable to connect to Netlink Kobject UEvent socket", err)
	}
	defer conn.Close()

	queue := make(chan netlink.UEvent)
	defer close(queue)

	errors := make(chan error)
	defer close(errors)

	quit := conn.Monitor(queue, errors, matcher)
	defer close(quit)

	for {
		select {

		case <-ctx.Done():
			return

		case uevent := <-queue:

			if event := common.EventAdapter(uevent); event != nil {

				// add UI properties to applicable events so that PowerLab UI can render it
				event := common.EventAdapterWithUIProperties(event)

				if v, ok := event.Properties["local-storage:path"]; ok && strings.Contains(event.Name, "disk") {

					diskModel := service.MyService.Disk().GetDiskInfo(v)
					if !reflect.DeepEqual(diskModel, model.LSBLKModel{}) {

						properties := common.AdditionalProperties(diskModel)
						for k, v := range properties {
							event.Properties[k] = v
						}
					}
				}
				_log.Info(ctx, "disk model", slog.String("diskModel", event.Name))
				response, err := service.MyService.MessageBus().PublishEventWithResponse(ctx, event.SourceID, event.Name, event.Properties)
				if err != nil {
					_log.Error(ctx, "failed to publish event to message bus", err, slog.Any("event", event))
				}

				if response.StatusCode() != http.StatusOK {
					_log.Error(ctx, "failed to publish event to message bus", nil,
						slog.String("status", response.Status()),
						slog.Any("response", response))
				}
			}

			switch uevent.Env["DEVTYPE"] {
			case "partition":

				switch uevent.Env["ID_BUS"] {
				case "usb":
					time.Sleep(1 * time.Second)
					sendUSBBySocket()
					continue
				}
			}

		case err := <-errors:
			_log.Error(ctx, "udev err", err)
		}
	}
}

func sendStorageStats() {
	sendDiskBySocket()
	sendUSBBySocket()
}
