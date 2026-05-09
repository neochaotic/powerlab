package v1

import (
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/neochaotic/powerlab/backend/core/pkg/utils"
	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/neochaotic/powerlab/backend/common/utils/logger"
)

// WsShell opens a local pseudo-terminal on the host PowerLab is running on
// and bridges it to a websocket. No SSH, no credentials, no Remote-Login
// requirement — the user is already authenticated via JWT and PowerLab is
// already running with the correct user's privileges.
//
// This is the default terminal for "open a shell on this server". It works
// on macOS dev (no Remote Login needed) and Linux production (no PAM
// dependency). For SSH-to-a-different-host, use WsSsh.
//
// GET /v1/sys/wsshell?cols=200&rows=32
func WsShell(ctx echo.Context) error {
	wsConn, upgradeErr := upgrader.Upgrade(ctx.Response().Writer, ctx.Request(), nil)
	if upgradeErr != nil {
		return upgradeErr
	}
	defer wsConn.Close()

	cols, _ := strconv.Atoi(utils.DefaultQuery(ctx, "cols", "200"))
	rows, _ := strconv.Atoi(utils.DefaultQuery(ctx, "rows", "32"))

	// Pick the user's preferred shell, falling back to bash then sh.
	shell := os.Getenv("SHELL")
	if shell == "" {
		if _, err := exec.LookPath("bash"); err == nil {
			shell = "bash"
		} else {
			shell = "sh"
		}
	}

	cmd := exec.Command(shell, "-l")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		_ = wsConn.WriteMessage(websocket.TextMessage,
			[]byte("\x1b[31mFailed to allocate pty: "+err.Error()+"\x1b[0m\r\n"))
		return nil
	}
	defer func() {
		_ = ptmx.Close()
		// Best-effort cleanup of the child shell so we don't leak on disconnect.
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	done := make(chan struct{})

	// pty → websocket
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if writeErr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					logger.Info("pty read ended", zap.Error(err))
				}
				return
			}
		}
	}()

	// websocket → pty (also handles resize messages encoded as JSON)
	for {
		select {
		case <-done:
			return nil
		default:
		}
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			return nil
		}
		// Resize protocol: "\x04{cols},{rows}" (Ctrl-D-prefixed). Plain typing
		// goes straight to pty stdin.
		if len(msg) > 0 && msg[0] == 0x04 {
			parts := splitResize(string(msg[1:]))
			if len(parts) == 2 {
				if c, err1 := strconv.Atoi(parts[0]); err1 == nil {
					if r, err2 := strconv.Atoi(parts[1]); err2 == nil {
						_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(c), Rows: uint16(r)})
						continue
					}
				}
			}
		}
		if _, err := ptmx.Write(msg); err != nil {
			return nil
		}
	}
}

func splitResize(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}
