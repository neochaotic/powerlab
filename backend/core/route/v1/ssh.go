package v1

import (
	"bytes"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/common_err"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	sshHelper "github.com/IceWhaleTech/CasaOS-Common/utils/ssh"
	"github.com/IceWhaleTech/CasaOS/pkg/utils"
	"github.com/labstack/echo/v4"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	modelCommon "github.com/IceWhaleTech/CasaOS-Common/model"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	CheckOrigin:      func(r *http.Request) bool { return true },
	HandshakeTimeout: time.Duration(time.Second * 5),
}

func PostSshLogin(ctx echo.Context) error {
	j := make(map[string]string)
	ctx.Bind(&j)
	userName := j["username"]
	password := j["password"]
	port := j["port"]
	if userName == "" || password == "" || port == "" {
		return ctx.JSON(common_err.CLIENT_ERROR, modelCommon.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS), Data: "Username or password or port is empty"})
	}
	_, err := sshHelper.NewSshClient(userName, password, port)
	if err != nil {
		logger.Error("connect ssh error", zap.Any("error", err))
		return ctx.JSON(common_err.CLIENT_ERROR, modelCommon.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: "Please check if the username and port are correct, and make sure that ssh server is installed."})
	}
	return ctx.JSON(common_err.SUCCESS, modelCommon.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

func WsSsh(ctx echo.Context) error {
	if _, err := exec.LookPath("ssh"); err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, modelCommon.Result{
			Success: common_err.SERVICE_ERROR,
			Message: common_err.GetMsg(common_err.SERVICE_ERROR),
			Data:    "ssh client not found on this host",
		})
	}

	userName := ctx.QueryParam("username")
	password := ctx.QueryParam("password")
	port := ctx.QueryParam("port")

	wsConn, upgradeErr := upgrader.Upgrade(ctx.Response().Writer, ctx.Request(), nil)
	if upgradeErr != nil {
		return upgradeErr
	}
	defer wsConn.Close()

	// Validate inputs once. The original implementation looped forever
	// retrying on every failed handshake — including missing credentials,
	// bad password, and SSH server unreachable — burning CPU and never
	// reporting back. We now do a single attempt and surface the error.
	if userName == "" || password == "" || port == "" {
		_ = wsConn.WriteMessage(websocket.TextMessage,
			[]byte("\x1b[31mssh: missing username, password, or port\x1b[0m\r\n"))
		return nil
	}

	client, err := sshHelper.NewSshClient(userName, password, port)
	if err != nil || client == nil {
		msg := "ssh: connection refused"
		if err != nil {
			msg = err.Error()
		}
		_ = wsConn.WriteMessage(websocket.TextMessage,
			[]byte("\x1b[31m"+msg+"\x1b[0m\r\n\r\n"))
		_ = wsConn.WriteMessage(websocket.TextMessage,
			[]byte("\x1b[33mIs the SSH server enabled on this host? On macOS:\r\n"+
				"  System Settings → General → Sharing → enable Remote Login\r\n"+
				"On Linux:\r\n"+
				"  sudo systemctl enable --now ssh\x1b[0m\r\n"))
		return nil
	}
	defer client.Close()

	cols, _ := strconv.Atoi(utils.DefaultQuery(ctx, "cols", "200"))
	rows, _ := strconv.Atoi(utils.DefaultQuery(ctx, "rows", "32"))

	ssConn, sshErr := sshHelper.NewSshConn(cols, rows, client)
	if sshErr != nil || ssConn == nil {
		_ = wsConn.WriteMessage(websocket.TextMessage,
			[]byte("\x1b[31mssh: failed to allocate pty: "+sshErr.Error()+"\x1b[0m\r\n"))
		return nil
	}
	defer ssConn.Close()

	logBuff := new(bytes.Buffer)
	quitChan := make(chan bool, 3)
	go ssConn.ReceiveWsMsg(wsConn, logBuff, quitChan)
	go ssConn.SendComboOutput(wsConn, quitChan)
	go ssConn.SessionWait(quitChan)

	<-quitChan
	return nil
}
