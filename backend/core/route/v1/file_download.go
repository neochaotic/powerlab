package v1

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/h2non/filetype"
	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/core/model"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/common_err"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
)

// GetDownloadFile streams either a single file or a multi-file
// archive (zip/tar/targz, picked via ?format). For multi-file the
// archive is built on-the-fly to the response writer; per-file
// archive errors are logged but don't abort the stream.
//
// @Summary download
// @Produce  application/json
// @Accept application/json
// @Tags file
// @Security ApiKeyAuth
// @Param format query string false "Compression format" Enums(zip,tar,targz)
// @Param files query string true "file list eg: filename1,filename2,filename3 "
// @Success 200 {string} string "ok"
// @Router /file/download [get]
func GetDownloadFile(ctx echo.Context) error {
	t := ctx.QueryParam("format")

	files := ctx.QueryParam("files")

	if len(files) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: common_err.GetMsg(common_err.INVALID_PARAMS),
		})
	}
	list := strings.Split(files, ",")
	for _, v := range list {
		if !file.Exists(v) {
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
				Success: common_err.FILE_DOES_NOT_EXIST,
				Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
			})
		}
	}
	ctx.Request().Header.Add("Content-Type", "application/octet-stream")
	ctx.Request().Header.Add("Content-Transfer-Encoding", "binary")
	ctx.Request().Header.Add("Cache-Control", "no-cache")
	// handles only single files not folders and multiple files
	if len(list) == 1 {

		filePath := list[0]
		info, err := os.Stat(filePath)
		if err != nil {
			return ctx.JSON(http.StatusOK, model.Result{
				Success: common_err.FILE_DOES_NOT_EXIST,
				Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
			})
		}
		if !info.IsDir() {

			// 打开文件
			fileTmp, _ := os.Open(filePath)
			defer fileTmp.Close()

			// 获取文件的名称
			fileName := path.Base(filePath)
			setUntrustedContentHeaders(ctx.Response().Header(), true)
			ctx.Response().Header().Set(echo.HeaderContentDisposition, contentDisposition("attachment", fileName))
			return ctx.File(filePath)
		}
	}

	extension, format, err := file.GetCompressionAlgorithm(t)
	if err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: common_err.GetMsg(common_err.INVALID_PARAMS),
		})
	}

	commonDir := file.CommonPrefix(filepath.Separator, list...)

	currentPath := filepath.Base(commonDir)

	name := "_" + currentPath
	name += extension
	setUntrustedContentHeaders(ctx.Response().Header(), true)
	ctx.Response().Header().Set(echo.HeaderContentDisposition, contentDisposition("attachment", name))
	if err := file.ArchiveFiles(ctx.Request().Context(), ctx.Response().Writer, format, list, commonDir); err != nil {
		log.Printf("Failed to archive: %v", err)
	}
	return nil
}

// GetDownloadSingleFile streams a single file with full content-
// type detection (filetype magic-byte sniff on first 261 bytes)
// + Last-Modified + Content-Length headers. Used by the file-
// preview drawer for inline-renderable files (images, videos).
func GetDownloadSingleFile(ctx echo.Context) error {
	filePath := ctx.QueryParam("path")
	if len(filePath) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{
			Success: common_err.INVALID_PARAMS,
			Message: common_err.GetMsg(common_err.INVALID_PARAMS),
		})
	}
	fileName := path.Base(filePath)

	fi, err := os.Open(filePath)
	if err != nil {
		// Audit #216 §C item 2: was `panic(err)` — converted to a
		// graceful error response matching the existing 267-style
		// pattern further down. The pkg/lifecycle recover middleware
		// still catches process-restart panics, but a missing file
		// is a CLIENT-shaped error (404-shaped semantically) and
		// should not look like a backend crash to the caller.
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
			Success: common_err.FILE_DOES_NOT_EXIST,
			Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
		})
	}

	// We only have to pass the file header = first 261 bytes
	buffer := make([]byte, 261)

	_, _ = fi.Read(buffer)

	kind, _ := filetype.Match(buffer)
	if kind != filetype.Unknown {
		ctx.Request().Header.Add("Content-Type", kind.MIME.Value)
	}

	// #39: Content-Disposition used to be added to ctx.Request(), so it
	// never reached the browser. Inline because this route backs the
	// preview drawer (<img>/<video>/<audio>/<embed>) and "Open in new
	// tab". PDFs skip the CSP sandbox: browsers refuse to run their
	// built-in PDF viewer inside a sandboxed document.
	isPDF := strings.EqualFold(filepath.Ext(fileName), ".pdf") || kind.MIME.Value == "application/pdf"
	setUntrustedContentHeaders(ctx.Response().Header(), !isPDF)
	ctx.Response().Header().Set(echo.HeaderContentDisposition, contentDisposition("inline", fileName))
	node, err := os.Stat(filePath)
	// Set the Last-Modified header to the timestamp
	ctx.Request().Header.Add("Last-Modified", node.ModTime().UTC().Format(http.TimeFormat))

	knownSize := node.Size() >= 0
	if knownSize {
		ctx.Request().Header.Add("Content-Length", strconv.FormatInt(node.Size(), 10))
	}
	http.ServeContent(ctx.Response().Writer, ctx.Request(), fileName, node.ModTime(), fi)
	defer fi.Close()
	fileTmp, err := os.Open(filePath)
	if err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
			Success: common_err.FILE_DOES_NOT_EXIST,
			Message: common_err.GetMsg(common_err.FILE_DOES_NOT_EXIST),
		})
	}
	defer fileTmp.Close()

	return nil
}

// Content-Security-Policy values for user-controlled file bodies (#39).
// A user can upload an .html or .svg; served from the panel origin it
// would otherwise run script with the panel's session. `sandbox` turns
// the document into an opaque origin, `script-src 'none'` blocks script
// outright. Subresource loads (<img>, <video>, <audio>) do not apply a
// response's CSP, so previews are unaffected.
const (
	cspUntrustedSandboxed = "sandbox; script-src 'none'"
	cspUntrustedNoSandbox = "script-src 'none'"
)

// setUntrustedContentHeaders marks a response that streams a file from
// disk as untrusted content. sandbox=false is for PDFs, whose built-in
// browser viewers do not load inside a sandboxed document.
func setUntrustedContentHeaders(h http.Header, sandbox bool) {
	if sandbox {
		h.Set(echo.HeaderContentSecurityPolicy, cspUntrustedSandboxed)
	} else {
		h.Set(echo.HeaderContentSecurityPolicy, cspUntrustedNoSandbox)
	}
	h.Set(echo.HeaderXContentTypeOptions, "nosniff")
}

// contentDisposition builds an RFC 6266 Content-Disposition value with
// an ASCII `filename=` fallback for old clients and an RFC 5987
// `filename*=` (UTF-8, percent-encoded) parameter carrying the real (possibly
// non-ASCII) name.
func contentDisposition(dispType, name string) string {
	return fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", dispType, asciiFilename(name), rfc5987Escape(name))
}

// asciiFilename replaces anything that cannot sit inside the quoted
// fallback (non-ASCII, control chars, quote, backslash) with '_'.
func asciiFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// rfc5987Escape percent-encodes every byte outside RFC 5987 attr-char.
// url.PathEscape is not enough: it leaves ';' and ',' unescaped, which
// would split the header parameter.
func rfc5987Escape(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') ||
			strings.IndexByte("!#$&+-.^_`|~", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}
