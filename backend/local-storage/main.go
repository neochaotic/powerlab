//go:generate bash -c "mkdir -p codegen && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -generate types,server,spec -package codegen api/local_storage/openapi.yaml > codegen/local_storage_api.go"
//go:generate bash -c "mkdir -p codegen/message_bus && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -generate types,client -package message_bus https://raw.githubusercontent.com/IceWhaleTech/CasaOS-MessageBus/main/api/message_bus/openapi.yaml > codegen/message_bus/api.go"

package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	util_http "github.com/neochaotic/powerlab/backend/common/utils/http"
	"github.com/neochaotic/powerlab/backend/common/utils/paths"
	"github.com/neochaotic/powerlab/backend/local-storage/codegen/message_bus"
	"github.com/neochaotic/powerlab/backend/local-storage/common"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/cache"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/config"
	mergerfspkg "github.com/neochaotic/powerlab/backend/local-storage/pkg/mergerfs"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/sqlite"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/utils/merge"
	"github.com/neochaotic/powerlab/backend/local-storage/route"
	v1route "github.com/neochaotic/powerlab/backend/local-storage/route/v1"
	v2route "github.com/neochaotic/powerlab/backend/local-storage/route/v2"
	"github.com/neochaotic/powerlab/backend/local-storage/service"
	v2service "github.com/neochaotic/powerlab/backend/local-storage/service/v2"
	"github.com/coreos/go-systemd/daemon"
	pkgfoundation "github.com/neochaotic/powerlab/backend/pkg/foundation"
	pkglifecycle "github.com/neochaotic/powerlab/backend/pkg/lifecycle"
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
)

// _log is the PowerLab-owned slog-based logger used by the foundation
// middleware (panic recovery + correlation-ID tracing) and by the
// init() goroutines launched before main() runs.
//
// init() can call _log via background goroutines (e.g. ensureDefault-
// Directories) before main() finishes constructing the configured
// foundation logger. To avoid a nil-pointer crash in that window we
// seed _log with a permissive default at process start; main() then
// replaces it with the env-configured logger and wires the same
// instance into every per-package _log via SetLogger calls.
var _log pkglogging.Logger = mustDefaultLogger()

func mustDefaultLogger() pkglogging.Logger {
	l, _ := pkglogging.New(pkglogging.Config{Level: "info", Format: "json"})
	return l
}

// wrapWithFoundation wraps any http.Handler with PowerLab's
// foundation middleware:
//
//  1. tracing.Middleware — outermost. Reads X-Request-Id (or mints
//     one), stores in context for log emission, echoes back.
//  2. lifecycle.RecoverMiddleware — catches panics in the handler
//     chain, logs with stack trace + correlation ID, writes 500
//     via pkg/errors.WriteHTTP.
//
// Apply to local-storage's single http.Server.Handler. Same pattern
// as gateway and message-bus (ADR-0011 strangler — see those services'
// part 2 PRs for the precedent). This is the structural close for
// the bug-#64 SIGSEGV class within the local-storage process: even
// if a handler dereferences a nil pointer or panics for any other
// reason, the process keeps running.
func wrapWithFoundation(h http.Handler) http.Handler {
	return pkgfoundation.Wrap(h, _log)
}

const localhost = "127.0.0.1"

var (
	commit = "private build"
	date   = "private build"

	//go:embed api/index.html
	_docHTML string

	//go:embed api/local_storage/openapi.yaml
	_docYAML string

	//go:embed build/sysroot/etc/powerlab/local-storage.conf.sample
	_confSample string
)

func init() {

	configFlag := flag.String("c", "", "config address")
	dbFlag := flag.String("db", "", "db path")

	versionFlag := flag.Bool("v", false, "version")
	initFlag := flag.Bool("init", false, "init local-storage config")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("v%s\n", common.Version)
		os.Exit(0)
	}

	println("git commit:", commit)
	println("build date:", date)

	config.InitSetup(*configFlag, _confSample)

	// pkg/logging is process-wide; no per-process file rotation init
	// is needed (the legacy CasaOS logger.LogInit was a noop after the
	// call-site migration in #104).

	if len(*dbFlag) == 0 {
		*dbFlag = config.AppInfo.DBPath
	}

	// Auto-resolve known-stale legacy duplicates BEFORE the strict
	// split-brain check. local-storage NEVER reads from
	// `<dbPath>/db/local-storage.db` — that path was only ever a
	// v0.5.4 hot-fix copy left behind by a buggy install.sh migration
	// (see #179). v0.5.9 (this PR) auto-cleans rather than locking
	// the operator out as v0.5.8 did.
	canonicalLSDB := paths.LocalStorageDBIn(*dbFlag)
	legacyLSDB := paths.LegacyLocalStorageDBIn(*dbFlag)
	bgCtx := context.Background()
	for _, bak := range paths.AutoMoveLegacyAside(bgCtx, nil, "local-storage", canonicalLSDB, legacyLSDB) {
		fmt.Fprintf(os.Stderr, "[local-storage] moved stale legacy DB aside: %s\n", bak)
	}

	if err := paths.AssertNoSplitBrain(bgCtx, nil, "local-storage",
		canonicalLSDB,
		legacyLSDB,
	); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	sqliteDB := sqlite.GetGlobalDB(*dbFlag)

	service.MyService = service.NewService(sqliteDB)
	service.Cache = cache.Init()
	if initFlag != nil && *initFlag {
		service.MyService.Disk().CheckSerialDiskMount()
		os.Exit(0)
		return
	}
	if strings.ToLower(config.ServerInfo.EnableMergerFS) == "true" {
		if !merge.IsMergerFSInstalled() {
			config.ServerInfo.EnableMergerFS = "false"
			println("mergerfs is disabled")
		}
	}

	if strings.ToLower(config.ServerInfo.EnableMergerFS) == "true" {
		if !service.MyService.Disk().EnsureDefaultMergePoint() {
			config.ServerInfo.EnableMergerFS = "false"
			println("mergerfs is disabled")
		}
	}

	if strings.ToLower(config.ServerInfo.EnableMergerFS) == "true" {
		go service.MyService.LocalStorage().CheckMergeMount()
	}

	checkToken2_11()
	go ensureDefaultDirectories()
	//service.MyService.Disk().EnsureDefaultMergePoint()

	// service.MountLists = make(map[string]*mountlib.MountPoint)
	// configfile.Install()

}

func checkToken2_11() {
	deviceTree, err := service.MyService.USB().GetDeviceTree()
	if err != nil {
		panic(err)
	}

	if service.MyService.USB().GetSysInfo().KernelArch == "aarch64" && strings.ToLower(config.ServerInfo.USBAutoMount) != "true" && strings.Contains(deviceTree, "Raspberry Pi") {
		service.MyService.USB().UpdateUSBAutoMount("False")
		service.MyService.USB().ExecUSBAutoMountShell("False")
	}
}

func ensureDefaultDirectories() {
	sysType := runtime.GOOS
	var dirArray []string
	if sysType == "linux" {
		dirArray = []string{"/DATA/AppData", "/DATA/Documents", "/DATA/Downloads", "/DATA/Gallery", "/DATA/Media/Movies", "/DATA/Media/TV Shows", "/DATA/Media/Music"}
	}

	if sysType == "windows" {
		dirArray = []string{"C:\\PowerLab\\DATA\\AppData", "C:\\PowerLab\\DATA\\Documents", "C:\\PowerLab\\DATA\\Downloads", "C:\\PowerLab\\DATA\\Gallery", "C:\\PowerLab\\DATA\\Media/Movies", "C:\\PowerLab\\DATA\\Media\\TV Shows", "C:\\PowerLab\\DATA\\Media\\Music"}
	}

	if sysType == "darwin" {
		dirArray = []string{"./PowerLab/DATA/AppData", "./PowerLab/DATA/Documents", "./PowerLab/DATA/Downloads", "./PowerLab/DATA/Gallery", "./PowerLab/DATA/Media/Movies", "./PowerLab/DATA/Media/TV Shows", "./PowerLab/DATA/Media/Music"}
	}

	for _, v := range dirArray {
		if err := file.IsNotExistMkDir(v); err != nil {
			_log.Error(context.Background(), "ensureDefaultDirectories", err)
		}
	}
}

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the PowerLab foundation logger before anything else
	// in main() — wrapWithFoundation and SafeGo both need _log to be
	// non-nil. Same env-var contract as gateway / message-bus.
	level := os.Getenv("POWERLAB_LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	format := os.Getenv("POWERLAB_LOG_FORMAT")
	if format == "" {
		format = "json"
	}
	fl, err := pkglogging.New(pkglogging.Config{Level: level, Format: format})
	if err != nil {
		// Fall back to a permissive default rather than crash main.
		fl, _ = pkglogging.New(pkglogging.Config{})
	}
	_log = fl

	// Wire the same instance into every package that owns a _log,
	// so all log lines in this process flow through one logger.
	// Mirrors the user-service main() pattern (ADR-0011).
	service.SetLogger(_log)
	v2service.SetLogger(_log)
	v1route.SetLogger(_log)
	v2route.SetLogger(_log)
	merge.SetLogger(_log)
	mergerfspkg.SetLogger(_log)

	pkglifecycle.SafeGo(ctx, _log, func() { monitorUEvent(ctx) })

	pkglifecycle.SafeGo(ctx, _log, sendStorageStats)

	crontab := cron.New(cron.WithSeconds())
	if _, err := crontab.AddFunc("@every 5s", sendStorageStats); err != nil {
		_log.Error(ctx, "crontab add func error", err)
	}

	crontab.Start()
	defer crontab.Stop()

	listener, err := net.Listen("tcp", net.JoinHostPort(localhost, "0"))
	if err != nil {
		// Listener bind is a hard startup failure — there is nothing
		// useful the process can do without a port. panic here is
		// equivalent to os.Exit(1) and runs deferred funcs.
		panic(err)
	}

	// register at gateway
	apiPaths := []string{
		"/v1/usb",
		"/v1/disks",
		"/v1/storage",
		// "/v1/cloud",
		// "/v1/recover",
		// "/v1/driver",
		route.V2APIPath,
		route.V2DocPath,
	}
	for _, apiPath := range apiPaths {
		err = service.MyService.Gateway().CreateRoute(&model.Route{
			Path:   apiPath,
			Target: "http://" + listener.Addr().String(),
		})

		if err != nil {
			panic(err)
		}
	}
	pkglifecycle.SafeGo(ctx, _log, RegMsg)
	pkglifecycle.SafeGo(ctx, _log, func() { service.MyService.Disk().InitCheck() })
	v1Router := route.InitV1Router()
	v2Router := route.InitV2Router()
	v2DocRouter := route.InitV2DocRouter(_docHTML, _docYAML)

	mux := &util_http.HandlerMultiplexer{
		HandlerMap: map[string]http.Handler{
			"v1":  v1Router,
			"v2":  v2Router,
			"doc": v2DocRouter,
		},
	}

	if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		_log.Error(ctx, "Failed to notify systemd that local storage service is ready", err)
	} else if supported {
		_log.Info(ctx, "Notified systemd that local storage service is ready")
	} else {
		_log.Info(ctx, "This process is not running as a systemd service.")
	}

	_log.Info(ctx, "LocalStorage service is listening...", slog.String("address", listener.Addr().String()))

	server := &http.Server{
		Handler:           wrapWithFoundation(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	err = server.Serve(listener)
	if err != nil {
		panic(err)
	}
}
// RegMsg registers the local-storage service's event types with the
// PowerLab message-bus. Two static event types ("merge_status" and
// "storage_status") are always registered; additionally every event
// type declared in common.EventTypes (per device kind) is registered.
//
// The first registration phase retries up to 10 times with a 1s
// backoff to handle the case where main() finishes booting before the
// message-bus service is ready to accept registrations. Subsequent
// per-devtype registrations run once and any errors are logged but
// not retried.
//
// Called as a background goroutine from main() under
// pkglifecycle.SafeGo; a panic during registration is recovered and
// logged.
func RegMsg() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events []message_bus.EventType
	events = append(events, message_bus.EventType{Name: common.ServiceName + ":merge_status", SourceID: common.ServiceName, PropertyTypeList: []message_bus.PropertyType{}})
	events = append(events, message_bus.EventType{Name: common.ServiceName + ":storage_status", SourceID: common.ServiceName, PropertyTypeList: []message_bus.PropertyType{}})
	// register at message bus
	for i := 0; i < 10; i++ {
		response, err := service.MyService.MessageBus().RegisterEventTypesWithResponse(context.Background(), events)
		if err != nil {
			_log.Error(ctx, "error when trying to register one or more event types - some event type will not be discoverable", err)
		}
		if response != nil && response.StatusCode() != http.StatusOK {
			_log.Error(ctx, "error when trying to register one or more event types - some event type will not be discoverable", nil,
				slog.String("status", response.Status()),
				slog.String("body", string(response.Body)))
		}
		if response.StatusCode() == http.StatusOK {
			break
		}
		time.Sleep(time.Second)
	}
	// register at message bus
	for devtype, eventTypesByAction := range common.EventTypes {
		response, err := service.MyService.MessageBus().RegisterEventTypesWithResponse(ctx, lo.Values(eventTypesByAction))
		if err != nil {
			_log.Error(ctx, "error when trying to register one or more event types - some event type will not be discoverable", err,
				slog.String("devtype", devtype))
		}

		if response != nil && response.StatusCode() != http.StatusOK {
			_log.Error(ctx, "error when trying to register one or more event types - some event type will not be discoverable", nil,
				slog.String("status", response.Status()),
				slog.String("body", string(response.Body)),
				slog.String("devtype", devtype))
		}
	}

}
