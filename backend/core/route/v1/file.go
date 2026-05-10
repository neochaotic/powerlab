// File-router shared types + globals. Sprint 7 #2 split (per #227):
// the original 1166-LOC file.go was split into 5 companion files
// based on responsibility. This file keeps the cross-handler types
// (ListReq, ObjResp, FsListResp) and the package-level WebSocket
// upgrader + connection state.
//
// Companion files:
//   - file_browse.go    — read paths (GetFilerContent, GetLocalFile,
//     DirPath, GetSize, GetFileCount, GetFileImage)
//   - file_mutate.go    — write paths (RenamePath, MkdirAll, DeleteFile,
//     PostOperateFileOrDir, DeleteOperateFileOrDir,
//     PutFileContent, PostFileContent)
//   - file_router_upload.go — multipart + chunked upload (GetFileUpload,
//     PostFileUpload, PostFileOctet)
//   - file_download.go  — download paths (GetDownloadFile,
//     GetDownloadSingleFile — panic-on-open fixed
//     per audit #216 §C item 2)
//   - file_websocket.go — legacy peer-broadcast subsystem
//     (CenterHandler, Client, PeerModel,
//     ConnectWebSocket, init, writePump,
//     readPump, monitoring, GetPeers)
package v1

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/neochaotic/powerlab/backend/core/model"
)

// ListReq is the standard pagination + path request used by the
// directory-listing endpoint. Embeds model.PageReq for Index/Size.
type ListReq struct {
	model.PageReq
	Path string `json:"path" form:"path"`
	// Refresh bool   `json:"refresh"`
}

// ObjResp is one row of a directory listing — name, size, dir
// flag, modified time, type, full path, plus driver-specific
// extension metadata (share state, mount state, etc.).
type ObjResp struct {
	Name       string                 `json:"name"`
	Size       int64                  `json:"size"`
	IsDir      bool                   `json:"is_dir"`
	Modified   time.Time              `json:"modified"`
	Sign       string                 `json:"sign"`
	Thumb      string                 `json:"thumb"`
	Type       int                    `json:"type"`
	Path       string                 `json:"path"`
	Date       time.Time              `json:"date"`
	Extensions map[string]interface{} `json:"extensions"`
}

// FsListResp is the response envelope for DirPath — paginated
// content rows + total count + the readme/write/provider hints
// used by the V2 file-explorer (currently unused in V1 but kept
// for shape compatibility).
type FsListResp struct {
	Content  []ObjResp `json:"content"`
	Total    int64     `json:"total"`
	Readme   string    `json:"readme,omitempty"`
	Write    bool      `json:"write,omitempty"`
	Provider string    `json:"provider,omitempty"`
	Index    int       `json:"index"`
	Size     int       `json:"size"`
}

// upgraderFile is the package-level WebSocket upgrader used by the
// peer-broadcast subsystem (file_websocket.go). CheckOrigin
// returns true so any origin can upgrade — caller relies on the
// gateway's JWT middleware as the auth gate.
//
// conn + err are package-level state shared with the upgrade path
// in file_websocket.go.ConnectWebSocket.
var (
	// 升级成 WebSocket 协议
	upgraderFile = websocket.Upgrader{
		// 允许CORS跨域请求
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn *websocket.Conn
	err  error
)
