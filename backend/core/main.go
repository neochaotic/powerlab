//go:generate bash -c "mkdir -p codegen && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -generate types,server,spec -package codegen api/core/openapi.yaml > codegen/core_api.go"
//go:generate bash -c "mkdir -p codegen/message_bus && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -generate types,client -package message_bus ../message-bus/api/message_bus/openapi.yaml > codegen/message_bus/api.go"
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/command"
	"github.com/neochaotic/powerlab/backend/common/utils/constants"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/neochaotic/powerlab/backend/common/utils/paths"

	util_http "github.com/neochaotic/powerlab/backend/common/utils/http"

	"github.com/neochaotic/powerlab/backend/core/common"
	"github.com/neochaotic/powerlab/backend/core/pkg/cache"
	"github.com/neochaotic/powerlab/backend/core/pkg/config"
	"github.com/neochaotic/powerlab/backend/core/pkg/sqlite"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
	"github.com/neochaotic/powerlab/backend/core/route"
	"github.com/neochaotic/powerlab/backend/core/service"
	"github.com/coreos/go-systemd/daemon"
	"go.uber.org/zap"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

const LOCALHOST = "127.0.0.1"

var sqliteDB *gorm.DB

var (
	commit = "private build"
	date   = "private build"

	//go:embed api/index.html
	_docHTML string

	//go:embed api/core/openapi.yaml
	_docYAML string

	//go:embed build/sysroot/etc/powerlab/core.conf.sample
	_confSample string

	configFlag  = flag.String("c", "", "config address")
	dbFlag      = flag.String("db", "", "db path")
	versionFlag = flag.Bool("v", false, "version")
)

func init() {
	// Skip the entire init in test binaries. init() does heavy startup
	// work (flag.Parse, config load, sqlite open, message-bus connect)
	// that's not appropriate during `go test` — flag.Parse fails because
	// it doesn't recognize -test.* flags. Production main() still runs
	// the work via the normal startup path. Same pattern as
	// gateway/main.go (issue #131 / #159 follow-up).
	if strings.HasSuffix(os.Args[0], ".test") || strings.Contains(os.Args[0], "/_test/") {
		return
	}

	flag.Parse()
	if *versionFlag {
		fmt.Println("v" + common.VERSION)
		return
	}

	println("git commit:", commit)
	println("build date:", date)

	config.InitSetup(*configFlag, _confSample)

	logger.LogInit(config.AppInfo.LogPath, config.AppInfo.LogSaveName, config.AppInfo.LogFileExt)
	if len(*dbFlag) == 0 {
		*dbFlag = config.AppInfo.DBPath + "/db"
	}

	// Refuse to start if core's casaOS.db exists at multiple paths.
	// Three candidates we know about (see docs/audits/db-paths.md):
	//   1. <dbFlag>/casaOS.db    — the path core is about to open
	//   2. /var/lib/casaos/db/casaOS.db — when /etc/powerlab/core.conf
	//      still has the pre-rebrand DBPath = /var/lib/casaos because
	//      install.sh's skip-if-exists preserved the old conf value
	//   3. <DataPath>/core.db    — the future canonical path (no /db/,
	//      no inherited casaOS naming). Not yet written by any code,
	//      but if an operator placed one manually it must surface.
	// Empty / duplicate paths are skipped silently inside the helper.
	inUseCorePath := filepath.Join(*dbFlag, "casaOS.db")
	legacyCasaos := paths.LegacyCasaOSCoreDB()
	if inUseCorePath == legacyCasaos {
		legacyCasaos = ""
	}
	canonicalCore := paths.CanonicalCoreDB()
	if inUseCorePath == canonicalCore {
		canonicalCore = ""
	}
	if err := paths.AssertNoSplitBrain(context.Background(), nil, "core",
		inUseCorePath, legacyCasaos, canonicalCore,
	); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	sqliteDB = sqlite.GetDb(*dbFlag)
	// gredis.GetRedisConn(config.RedisInfo),

	service.MyService = service.NewService(sqliteDB, config.CommonInfo.RuntimePath)

	service.Cache = cache.Init()

	service.GetCPUThermalZone()

	route.InitFunction()

	//service.MyService.System().GenreateSystemEntry()
	///
	//service.MountLists = make(map[string]*mountlib.MountPoint)
	//configfile.Install()
}

// @title PowerLab Core API
// @version 1.0.0
// @contact.name PowerLab contributors
// @contact.url https://github.com/neochaotic/powerlab
// @description PowerLab core service v1 API
// @host 192.168.2.217:8089
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @BasePath /v1
func main() {
	if *versionFlag {
		return
	}
	v1Router := route.InitV1Router()

	v2Router := route.InitV2Router()
	v2DocRouter := route.InitV2DocRouter(_docHTML, _docYAML)
	v3File := route.InitFile()
	mux := &util_http.HandlerMultiplexer{
		HandlerMap: map[string]http.Handler{
			"v1":  v1Router,
			"v2":  v2Router,
			"v3":  v3File,
			"doc": v2DocRouter,
		},
	}

	crontab := cron.New(cron.WithSeconds())
	if _, err := crontab.AddFunc("@every 5s", route.SendAllHardwareStatusBySocket); err != nil {
		logger.Error("add crontab error", zap.Error(err))
	}

	crontab.Start()
	defer crontab.Stop()

	listener, err := net.Listen("tcp", net.JoinHostPort(LOCALHOST, "8089"))
	if err != nil {
		panic(err)
	}
	routers := []string{
		"/v1/sys",
		"/v1/port",
		"/v1/file",
		"/v1/folder",
		"/v1/batch",
		"/v1/image",
		"/v1/notify",
		// /v1/driver, /v1/cloud, /v1/recover removed in Sprint 3 Phase 3
		// (#101) — see backend/core/route/v1.go for rationale.
		"/v1/other",
		"/v1/test",
		// PowerLab-specific endpoints (issue #21 — in-UI updater).
		// Registered as a separate gateway prefix so it doesn't
		// collide with the legacy /v1/sys/update path that still
		// targets the upstream CasaOS version probe.
		"/v1/powerlab-update",
		// PowerLab version handshake. Unauthenticated probe so the UI
		// can warn a user staring at a stale login screen that the
		// JS bundle in their browser is older than the running backend.
		"/v1/powerlab",
		route.V2APIPath,
		route.V2DocPath,
		route.V3FilePath,
	}
	for _, apiPath := range routers {
		if service.MyService.Gateway() != nil {
			err = service.MyService.Gateway().CreateRoute(&model.Route{
				Path:   apiPath,
				Target: "http://" + listener.Addr().String(),
			})
			if err != nil {
				fmt.Println("err", err)
				panic(err)
			}
		}
	}

	// register at message bus
	for i := 0; i < 10; i++ {
		response, err := service.MyService.MessageBus().RegisterEventTypesWithResponse(context.Background(), common.EventTypes)
		if err != nil {
			logger.Error("error when trying to register one or more event types - some event type will not be discoverable", zap.Error(err))
		}
		if response != nil && response.StatusCode() != http.StatusOK {
			logger.Error("error when trying to register one or more event types - some event type will not be discoverable", zap.String("status", response.Status()), zap.String("body", string(response.Body)))
		}
		if response != nil && response.StatusCode() == http.StatusOK {
			break
		}
		time.Sleep(time.Second)
	}

	go func() {
		time.Sleep(time.Second * 2)
		if config.ServerInfo.HttpPort != "" && service.MyService.Gateway() != nil {
			changePort := model.ChangePortRequest{}
			changePort.Port = config.ServerInfo.HttpPort
			err := service.MyService.Gateway().ChangePort(&changePort)
			if err == nil {
				config.Cfg.Section("server").Key("HttpPort").SetValue("")
				config.Cfg.SaveTo(config.SystemConfigInfo.ConfigPath)
			}
		}
	}()

	urlFilePath := filepath.Join(config.CommonInfo.RuntimePath, "casaos.url")
	if err := file.CreateFileAndWriteContent(urlFilePath, "http://"+listener.Addr().String()); err != nil {
		logger.Error("error when creating address file", zap.Error(err),
			zap.Any("address", listener.Addr().String()),
			zap.Any("filepath", urlFilePath),
		)
	}

	// run any script that needs to be executed
	scriptDirectory := filepath.Join(constants.DefaultConfigPath, "start.d")
	command.ExecuteScripts(scriptDirectory)

	if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		logger.Error("Failed to notify systemd that powerlab-core is ready", zap.Any("error", err))
	} else if supported {
		logger.Info("Notified systemd that powerlab-core is ready")
	} else {
		logger.Info("This process is not running as a systemd service.")
	}
	// http.HandleFunc("/v1/file/test", func(w http.ResponseWriter, r *http.Request) {

	// 	//http.ServeFile(w, r, r.URL.Path[1:])
	// 	http.ServeFile(w, r, "/DATA/test.img")
	// })
	// go http.ListenAndServe(":8081", nil)

	s := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // fix G112: Potential slowloris attack (see https://github.com/securego/gosec)
	}

	logger.Info("PowerLab core service is listening...", zap.Any("address", listener.Addr().String()))
	// defer service.MyService.Storage().UnmountAllStorage()
	err = s.Serve(listener) // not using http.serve() to fix G114: Use of net/http serve function that has no support for setting timeouts (see https://github.com/securego/gosec)
	if err != nil {
		panic(err)
	}
}
