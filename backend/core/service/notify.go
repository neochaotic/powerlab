package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/neochaotic/powerlab/backend/core/common"
	model2 "github.com/neochaotic/powerlab/backend/core/model"
	"github.com/neochaotic/powerlab/backend/core/model/notify"
	"github.com/neochaotic/powerlab/backend/core/service/model"
	"github.com/neochaotic/powerlab/backend/core/types"
	"go.uber.org/zap"
	"golang.org/x/sync/syncmap"

	socketio "github.com/googollee/go-socket.io"
	"gorm.io/gorm"
)

// NotifyServer brokers system notifications: persists per-app
// notification log rows + emits live events via the message-bus.
// SystemTempMap holds in-memory aggregated metric snapshots
// (CPU/memory/disk) used by the homepage dashboard widgets.
type NotifyServer interface {
	GetLog(id string) model.AppNotify
	AddLog(log model.AppNotify)
	UpdateLog(log model.AppNotify)
	UpdateLogByCustomID(log model.AppNotify)
	DelLog(id string)
	GetList(c int) (list []model.AppNotify)
	MarkRead(id string, state int)

	// SendFileOperateNotify publishes the buffered file-operation
	// events on the message-bus. nowSend bypasses the throttle.
	SendFileOperateNotify(nowSend bool)
	// SendNotify publishes a custom event on the message-bus.
	SendNotify(name string, message map[string]interface{})
	// SettingSystemTempData merges keys into the in-memory metric
	// snapshot map.
	SettingSystemTempData(message map[string]interface{})
	// GetSystemTempMap returns the live metric snapshot map.
	GetSystemTempMap() *syncmap.Map
}

type notifyServer struct {
	db            *gorm.DB
	SystemTempMap syncmap.Map //[string]interface{}
}

func (i *notifyServer) SettingSystemTempData(message map[string]interface{}) {
	for k, v := range message {
		i.SystemTempMap.Store(k, v)
		//i.SystemTempMap[k] = v
	}
}

func (i *notifyServer) SendNotify(name string, message map[string]interface{}) {
	msg := make(map[string]string)
	for k, v := range message {
		bt, _ := json.Marshal(v)
		msg[k] = string(bt)
	}
	response, err := MyService.MessageBus().PublishEventWithResponse(context.Background(), common.SERVICENAME, name, msg)
	if err != nil {
		logger.Error("failed to publish event to message bus", zap.Error(err), zap.Any("event", msg))
		return
	}
	if response.StatusCode() != http.StatusOK {
		logger.Error("failed to publish event to message bus", zap.String("status", response.Status()), zap.Any("response", response))
	}
	// SocketServer.BroadcastToRoom("/", "public", path, message)
}

// SendFileOperateNotify publishes the in-flight file-operation
// queue snapshot to the message-bus. nowSend=true publishes once
// (and emits an empty-Data snapshot when the queue is empty —
// drives the UI's "ops complete" toast). nowSend=false loops
// every 3s until the queue drains, used as a goroutine on first
// op enqueue.
//
// Sprint 7 #6 (per #227 §F): the build-snapshot-and-publish body
// was duplicated between the two branches; extracted as
// publishFileOperateSnapshot for shared use.
func (i *notifyServer) SendFileOperateNotify(nowSend bool) {
	if nowSend {
		publishFileOperateSnapshot(true)
		return
	}
	for {
		if !publishFileOperateSnapshot(false) {
			return
		}
		time.Sleep(time.Second * 3)
	}
}

// publishFileOperateSnapshot builds one notify.NotifyModel from
// the FileQueue + OpStrArr state and publishes it to the
// message-bus. emitWhenEmpty=true publishes an empty-Data
// snapshot when the queue is empty (the once-and-done
// "ops complete" path used by SendFileOperateNotify(true) and
// the Delete handler); emitWhenEmpty=false short-circuits and
// returns false (signal to the polling caller to stop looping).
//
// Returns true when the snapshot was non-empty + published;
// false when the queue was empty + emitWhenEmpty was false.
//
// Side effects: mutates FileQueue + OpStrArr — finished tasks
// are removed and ExecOpFile is fired for each (matches the
// pre-extract behaviour exactly; see the original git history
// for the per-line commentary).
func publishFileOperateSnapshot(emitWhenEmpty bool) bool {
	queueLen := 0
	FileQueue.Range(func(k, v interface{}) bool {
		queueLen++
		return true
	})

	listMsg := make(map[string]interface{})
	m := notify.NotifyModel{}

	if queueLen == 0 {
		if !emitWhenEmpty {
			return false
		}
		m.Data = []string{}
		listMsg["file_operate"] = m
		publishFileOperateMessage(listMsg)
		return true
	}

	m.State = "NORMAL"
	list := []notify.File{}
	OpStrArrbak := OpStrArr

	for _, v := range OpStrArrbak {
		tempItem, ok := FileQueue.Load(v)
		if !ok {
			continue
		}
		temp := tempItem.(model2.FileOperate)
		task := notify.File{}
		task.Id = v
		task.ProcessedSize = temp.ProcessedSize
		task.TotalSize = temp.TotalSize
		task.To = temp.To
		task.Type = temp.Type
		if task.ProcessedSize == 0 {
			task.Status = "STARTING"
		} else {
			task.Status = "PROCESSING"
		}

		if temp.Finished || temp.ProcessedSize >= temp.TotalSize {
			task.Finished = true
			task.Status = "FINISHED"
			FileQueue.Delete(v)
			OpStrArr = OpStrArr[1:]
			go ExecOpFile()
			list = append(list, task)
			continue
		}
		for _, v := range temp.Item {
			if v.Size != v.ProcessedSize {
				task.ProcessingPath = v.From
				break
			}
		}

		list = append(list, task)
	}
	m.Data = list

	listMsg["file_operate"] = m
	publishFileOperateMessage(listMsg)
	return true
}

// publishFileOperateMessage marshals + publishes the file-operate
// envelope to the message-bus. Errors are logged but not returned —
// the publish is best-effort.
func publishFileOperateMessage(listMsg map[string]interface{}) {
	msg := make(map[string]string)
	for k, v := range listMsg {
		bt, _ := json.Marshal(v)
		msg[k] = string(bt)
	}
	response, err := MyService.MessageBus().PublishEventWithResponse(context.Background(), common.SERVICENAME, common.EventFileOperate, msg)
	if err != nil {
		logger.Error("failed to publish event to message bus", zap.Error(err), zap.Any("event", msg))
		return
	}
	if response.StatusCode() != http.StatusOK {
		logger.Error("failed to publish event to message bus", zap.String("status", response.Status()), zap.Any("response", response))
	}
}

// func (i *notifyServer) SendInstallAppBySocket(app notifyCommon.Application) {
// 	SocketServer.BroadcastToRoom("/", "public", "app_install", app)
// }

// func (i *notifyServer) SendUninstallAppBySocket(app notifyCommon.Application) {
// 	SocketServer.BroadcastToRoom("/", "public", "app_uninstall", app)
// }

func (i *notifyServer) SSR() {
	server := socketio.NewServer(nil)
	fmt.Println(server)
}

func (i *notifyServer) GetList(c int) (list []model.AppNotify) {
	i.db.Where("class = ?", c).Where(i.db.Where("state = ?", types.NOTIFY_DYNAMICE).Or("state = ?", types.NOTIFY_UNREAD)).Find(&list)
	return
}

func (i *notifyServer) AddLog(log model.AppNotify) {
	i.db.Create(&log)
}

func (i *notifyServer) UpdateLog(log model.AppNotify) {
	i.db.Save(&log)
}

func (i *notifyServer) UpdateLogByCustomID(log model.AppNotify) {
	if len(log.CustomId) == 0 {
		return
	}
	i.db.Model(&model.AppNotify{}).Select("*").Where("custom_id = ? ", log.CustomId).Updates(log)
}

func (i *notifyServer) GetLog(id string) model.AppNotify {
	var log model.AppNotify
	i.db.Where("custom_id = ? ", id).First(&log)
	return log
}

func (i *notifyServer) MarkRead(id string, state int) {
	if id == "0" {
		i.db.Model(&model.AppNotify{}).Where("1 = ?", 1).Update("state", state)
		return
	}
	i.db.Model(&model.AppNotify{}).Where("id = ? ", id).Update("state", state)
}

func (i *notifyServer) DelLog(id string) {
	var log model.AppNotify
	i.db.Where("custom_id = ?", id).Delete(&log)
}

// GetSystemTempMap returns a pointer to the in-memory system-temperature
// map. Returning a pointer (not a copy) is required because syncmap.Map
// embeds a sync.Mutex — copying it loses synchronisation, which Go's
// 1.25 vet promoted to a hard build error.
func (i *notifyServer) GetSystemTempMap() *syncmap.Map {
	return &i.SystemTempMap
}

// NewNotifyService returns a NotifyServer backed by db.
func NewNotifyService(db *gorm.DB) NotifyServer {
	return &notifyServer{db: db, SystemTempMap: syncmap.Map{}}
}
