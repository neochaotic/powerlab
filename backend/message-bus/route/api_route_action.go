package route

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/common"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
	"github.com/neochaotic/powerlab/backend/message-bus/route/adapter/in"
	"github.com/neochaotic/powerlab/backend/message-bus/route/adapter/out"
)

func (r *APIRoute) GetActionTypes(c echo.Context) error {
	actionType, err := r.services.ActionTypeService.GetActionTypes()
	if err != nil {
		message := err.Error()
		return c.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{Message: &message})
	}

	results := make([]codegen.ActionType, 0)

	for _, actionType := range actionType {
		results = append(results, out.ActionTypeAdapter(actionType))
	}

	return c.JSON(http.StatusOK, results)
}

func (r *APIRoute) RegisterActionTypes(c echo.Context) error {
	var actionTypes []codegen.ActionType
	if err := c.Bind(&actionTypes); err != nil {
		message := err.Error()
		return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
	}

	for _, actionType := range actionTypes {
		_, err := r.services.ActionTypeService.RegisterActionType(in.ActionTypeAdapter(actionType))
		if err != nil {
			message := err.Error()
			return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
		}
	}

	return c.JSON(http.StatusOK, codegen.ResponseOK{})
}

func (r *APIRoute) GetActionTypesBySourceID(c echo.Context, sourceID codegen.SourceID) error {
	results, err := r.services.ActionTypeService.GetActionTypesBySourceID(sourceID)
	if err != nil {
		message := err.Error()
		return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
	}

	return c.JSON(http.StatusOK, results)
}

func (r *APIRoute) GetActionType(c echo.Context, sourceID codegen.SourceID, name codegen.EventName) error {
	result, err := r.services.ActionTypeService.GetActionType(sourceID, name)
	if err != nil {
		message := err.Error()
		return c.JSON(http.StatusNotFound, codegen.ResponseNotFound{Message: &message})
	}

	if result == nil {
		return c.JSON(http.StatusNotFound, codegen.ResponseNotFound{Message: utils.Ptr("not found")})
	}

	return c.JSON(http.StatusOK, result)
}

func (r *APIRoute) TriggerAction(c echo.Context, sourceID codegen.SourceID, name codegen.EventName) error {
	actionType, err := r.services.ActionTypeService.GetActionType(sourceID, name)
	if err != nil {
		message := err.Error()
		return c.JSON(http.StatusNotFound, codegen.ResponseNotFound{Message: &message})
	}

	if actionType == nil {
		return c.JSON(http.StatusNotFound, codegen.ResponseNotFound{Message: utils.Ptr("not found")})
	}

	var properties map[string]string
	if err := c.Bind(&properties); err != nil {
		message := err.Error()
		return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
	}

	action := in.ActionAdapter(codegen.Action{
		SourceID:   sourceID,
		Name:       name,
		Properties: properties,
		Timestamp:  utils.Ptr(time.Now()),
	})

	go r.services.SocketIOService.Publish(action)
	go r.services.ActionServiceWS.Trigger(action)

	return c.JSON(http.StatusOK, out.ActionAdapter(action))
}

func (r *APIRoute) SubscribeActionWS(c echo.Context, sourceID codegen.SourceID, params codegen.SubscribeActionWSParams) error {
	var actionNames []string
	if params.Names != nil {
		for _, actionName := range *params.Names {
			actionType, err := r.services.ActionTypeService.GetActionType(sourceID, actionName)
			if err != nil {
				message := err.Error()
				return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
			}

			if actionType == nil {
				return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: utils.Ptr("action type not found")})
			}

			actionNames = append(actionNames, actionName)
		}
	} else {
		actionTypes, err := r.services.ActionTypeService.GetActionTypesBySourceID(sourceID)
		if err != nil {
			message := err.Error()
			return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
		}

		for _, actionType := range actionTypes {
			actionNames = append(actionNames, actionType.Name)
		}
	}

	conn, _, _, err := ws.UpgradeHTTP(c.Request(), c.Response())
	if err != nil {
		message := err.Error()
		return c.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{Message: &message})
	}

	channel, err := r.services.ActionServiceWS.Subscribe(sourceID, actionNames)
	if err != nil {
		conn.Close() // need to close connection here, instead of defer, because of the goroutine
		message := err.Error()
		return c.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{Message: &message})
	}

	go func(conn net.Conn, channel chan model.Action, actionNames []string) {
		defer conn.Close()
		defer close(channel)
		defer func(actionNames []string) {
			for _, name := range actionNames {
				if err := r.services.ActionServiceWS.Unsubscribe(sourceID, name, channel); err != nil {
					_log.Error(context.Background(), "error when trying to unsubscribe an action type via websocket", err, slog.String("source_id", sourceID), slog.String("name", name))
				}
			}
		}(actionNames)

		_log.Info(context.Background(), "a websocket connection has started for actions", slog.String("remote_addr", conn.RemoteAddr().String()))

		for {
			action, ok := <-channel
			if !ok {
				_log.Info(context.Background(), "websocket channel for events is closed")
				return
			}

			if action.SourceID == common.MessageBusSourceID && action.Name == common.MessageBusHeartbeatName {
				if err := wsutil.WriteServerMessage(conn, ws.OpPing, []byte{}); err != nil {
					_log.Error(context.Background(), "error when trying to send ping message via websocket", err)
					return
				}
				continue
			}

			message, err := json.Marshal(out.ActionAdapter(action))
			if err != nil {
				_log.Error(context.Background(), "error when trying to marshal action for websocket", err)
				continue
			}

			_log.Info(context.Background(), "sending action via websocket", slog.String("remote_addr", conn.RemoteAddr().String()), slog.String("message", string(message)))

			if err := wsutil.WriteServerBinary(conn, message); err != nil {
				if _, ok := err.(*net.OpError); ok {
					_log.Info(context.Background(), "websocket connection ended", slog.String("error", err.Error()))
				} else {
					_log.Error(context.Background(), "error when sending event via websocket", err)
				}
				return
			}
		}
	}(conn, channel, actionNames)

	return nil
}
