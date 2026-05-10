package service

import (
	"context"
	"encoding/json"
	json2 "encoding/json"
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
	"github.com/gorilla/websocket"
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

// Send periodic broadcast messages
func (i *notifyServer) SendFileOperateNotify(nowSend bool) {
	if nowSend {
		len := 0
		FileQueue.Range(func(k, v interface{}) bool {
			len++
			return true
		})

		model := notify.NotifyModel{}
		listMsg := make(map[string]interface{})
		if len == 0 {
			model.Data = []string{}
			listMsg["file_operate"] = model
			msg := make(map[string]string)
			for k, v := range listMsg {
				bt, _ := json.Marshal(v)
				msg[k] = string(bt)
			}
			response, err := MyService.MessageBus().PublishEventWithResponse(context.Background(), common.SERVICENAME, common.EventFileOperate, msg)
			if err != nil {
				logger.Error("failed to publish event to message bus", zap.Error(err), zap.Any("event", msg))
			}
			if response.StatusCode() != http.StatusOK {
				logger.Error("failed to publish event to message bus", zap.String("status", response.Status()), zap.Any("response", response))
			}
			return
		}

		model.State = "NORMAL"
		list := []notify.File{}
		OpStrArrbak := OpStrArr

		for _, v := range OpStrArrbak {
			tempItem, ok := FileQueue.Load(v)
			temp := tempItem.(model2.FileOperate)
			if !ok {
				continue
			}
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
		model.Data = list

		listMsg["file_operate"] = model
		msg := make(map[string]string)
		for k, v := range listMsg {
			bt, _ := json.Marshal(v)
			msg[k] = string(bt)
		}
		response, err := MyService.MessageBus().PublishEventWithResponse(context.Background(), common.SERVICENAME, common.EventFileOperate, msg)
		if err != nil {
			logger.Error("failed to publish event to message bus", zap.Error(err), zap.Any("event", msg))
		}
		if response.StatusCode() != http.StatusOK {
			logger.Error("failed to publish event to message bus", zap.String("status", response.Status()), zap.Any("response", response))
		}

	} else {
		for {

			len := 0
			FileQueue.Range(func(k, v interface{}) bool {
				len++
				return true
			})
			if len == 0 {
				return
			}
			listMsg := make(map[string]interface{})
			model := notify.NotifyModel{}
			model.State = "NORMAL"
			list := []notify.File{}
			OpStrArrbak := OpStrArr

			for _, v := range OpStrArrbak {
				tempItem, ok := FileQueue.Load(v)
				temp := tempItem.(model2.FileOperate)
				if !ok {
					continue
				}
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
			model.Data = list

			listMsg["file_operate"] = model
			msg := make(map[string]string)
			for k, v := range listMsg {
				bt, _ := json.Marshal(v)
				msg[k] = string(bt)
			}
			response, err := MyService.MessageBus().PublishEventWithResponse(context.Background(), common.SERVICENAME, common.EventFileOperate, msg)
			if err != nil {
				logger.Error("failed to publish event to message bus", zap.Error(err), zap.Any("event", msg))
			}
			if response.StatusCode() != http.StatusOK {
				logger.Error("failed to publish event to message bus", zap.String("status", response.Status()), zap.Any("response", response))
			}
			time.Sleep(time.Second * 3)
		}
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

// SendMeg (typo preserved as it's the live function name) is the
// legacy WebSocket broadcaster — fans every queued message out to
// every active connection.
func SendMeg() {
	// for {
	// 	mt, message, err := ws.ReadMessage()
	// 	if err != nil {
	// 		break
	// 	}
	// 	notify := model.NotifyMssage{}
	// 	json2.Unmarshal(message, &notify)
	// 	if notify.Type == "read" {
	// 		service.MyService.Notify().MarkRead(notify.Data, types.NOTIFY_READ)
	// 	}
	// 	if notify.Type == "app" {
	//		go func(ws *websocket.Conn) {

	for {
		list := MyService.Notify().GetList(types.NOTIFY_APP)
		json, _ := json2.Marshal(list)

		if len(list) > 0 {
			var temp []*websocket.Conn
			for _, v := range WebSocketConns {

				err := v.WriteMessage(1, json)
				if err == nil {
					temp = append(temp, v)
				}
			}
			WebSocketConns = temp
			for _, v := range list {
				MyService.Notify().MarkRead(v.Id, types.NOTIFY_READ)
			}
		}

		if len(WebSocketConns) == 0 {
			SocketRun = false
		}
		time.Sleep(time.Second * 2)
	}
	// 	}(ws)
	// }
	//	}
}

// func (i notifyServer) SendText(m model.AppNotify) {
// 	list := []model.AppNotify{}
// 	list = append(list, m)
// 	json, _ := json2.Marshal(list)
// 	var temp []*websocket.Conn
// 	for _, v := range WebSocketConns {

// 		err := v.WriteMessage(1, json)
// 		if err == nil {
// 			temp = append(temp, v)
// 		}
// 	}
// 	WebSocketConns = temp

// 	if len(WebSocketConns) == 0 {
// 		SocketRun = false
// 	}

// }
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
