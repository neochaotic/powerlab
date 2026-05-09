//go:generate bash -c "mkdir -p codegen/user_service && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -generate types,server,spec -package codegen api/user-service/openapi.yaml > codegen/user_service/user_service_api.go"
//go:generate bash -c "mkdir -p codegen/message_bus && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -package message_bus https://raw.githubusercontent.com/IceWhaleTech/CasaOS-MessageBus/main/api/message_bus/openapi.yaml > codegen/message_bus/api.go"
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
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/external"
	"github.com/IceWhaleTech/CasaOS-Common/model"
	util_http "github.com/IceWhaleTech/CasaOS-Common/utils/http"
	"github.com/IceWhaleTech/CasaOS-Common/utils/jwt"
	"github.com/IceWhaleTech/CasaOS-UserService/codegen/message_bus"
	"github.com/IceWhaleTech/CasaOS-UserService/common"
	"github.com/IceWhaleTech/CasaOS-UserService/pkg/config"
	"github.com/IceWhaleTech/CasaOS-UserService/pkg/sqlite"
	"github.com/IceWhaleTech/CasaOS-UserService/pkg/utils/encryption"
	"github.com/IceWhaleTech/CasaOS-UserService/pkg/utils/random"
	v1route "github.com/IceWhaleTech/CasaOS-UserService/route"
	v1userroute "github.com/IceWhaleTech/CasaOS-UserService/route/v1"
	"github.com/IceWhaleTech/CasaOS-UserService/service"
	"github.com/coreos/go-systemd/daemon"
	pkgfoundation "github.com/neochaotic/powerlab/backend/pkg/foundation"
	pkglifecycle "github.com/neochaotic/powerlab/backend/pkg/lifecycle"
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
)

// _log is the PowerLab-owned slog-based logger used by the foundation
// middleware (panic recovery + correlation-ID tracing). Constructed
// once at the top of main() and passed to wrapWithFoundation.
var _log pkglogging.Logger

// wrapWithFoundation delegates to pkg/foundation.Wrap so user-service
// shares the canonical middleware chain (tracing + recover) with
// every other PowerLab service. See pkg/foundation for the contract.
func wrapWithFoundation(h http.Handler) http.Handler {
	return pkgfoundation.Wrap(h, _log)
}

const localhost = "127.0.0.1"

var (
	commit = "private build"
	date   = "private build"

	//go:embed api/index.html
	_docHTML string

	//go:embed api/user-service/openapi.yaml
	_docYAML string

	//go:embed build/sysroot/etc/powerlab/user-service.conf.sample
	_confSample string
)

func init() {
	configFlag := flag.String("c", "", "config address")
	dbFlag := flag.String("db", "", "db path")
	resetUserFlag := flag.Bool("ru", false, "reset user")
	userFlag := flag.String("user", "", "user name")
	versionFlag := flag.Bool("v", false, "version")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("v%s\n", common.Version)
		os.Exit(0)
	}

	println("git commit:", commit)
	println("build date:", date)

	config.InitSetup(*configFlag, _confSample)

	// LogInit removed in Sprint 2 Kill #4 — pkg/logging is constructed
	// in main() and is the single logger for this process. No
	// remaining call sites in user-service use the CasaOS-Common
	// legacy logger after this kill.

	if len(*dbFlag) == 0 {
		*dbFlag = config.AppInfo.DBPath
	}

	sqliteDB := sqlite.GetDb(*dbFlag)
	service.MyService = service.NewService(sqliteDB, config.CommonInfo.RuntimePath)

	if *resetUserFlag {
		if userFlag == nil || len(*userFlag) == 0 {
			fmt.Println("user is empty")
			return
		}

		userData := service.MyService.User().GetUserAllInfoByName(*userFlag)

		if userData.Id == 0 {
			fmt.Println("user not exist")
			return
		}

		password := random.RandomString(6, false)
		hashedPassword, err := encryption.HashPassword(password)
		if err != nil {
			fmt.Printf("failed to hash password: %v\n", err)
			return
		}
		userData.Password = hashedPassword
		service.MyService.User().UpdateUserPassword(userData)
		fmt.Println("User reset successful")
		fmt.Println("UserName:" + userData.Username)
		fmt.Println("Password:" + password)
	}
}

func main() {
	// Initialize the PowerLab foundation logger before anything else
	// in main(). wrapWithFoundation, SafeGo, and the per-package
	// _log overrides all need this. Same env-var contract as
	// gateway / message-bus / local-storage.
	level := os.Getenv("POWERLAB_LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	format := os.Getenv("POWERLAB_LOG_FORMAT")
	if format == "" {
		format = "json"
	}
	fl, lerr := pkglogging.New(pkglogging.Config{Level: level, Format: format})
	if lerr != nil {
		fl, _ = pkglogging.New(pkglogging.Config{})
	}
	_log = fl

	// Wire the same instance into every package that owns a _log,
	// so all log lines in this process flow through one logger.
	v1route.SetLogger(_log)
	v1userroute.SetLogger(_log)
	service.SetLogger(_log)
	sqlite.SetLogger(_log)

	v1Router := v1route.InitRouter()
	v2Router := v1route.InitV2Router()
	v2DocRouter := v1route.InitV2DocRouter(_docHTML, _docYAML)

	_, publicKey := service.MyService.User().GetKeyPair()

	jswkJSON, err := jwt.GenerateJwksJSON(publicKey)
	if err != nil {
		panic(err)
	}

	mux := &util_http.HandlerMultiplexer{
		HandlerMap: map[string]http.Handler{
			"v1":                                    v1Router,
			"v2":                                    v2Router,
			"doc":                                   v2DocRouter,
			strings.SplitN(jwt.JWKSPath, "/", 2)[0]: jwt.JWKSHandler(jswkJSON),
		},
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(localhost, "0"))
	if err != nil {
		panic(err)
	}

	apiPaths := []string{
		"/v1/users",
		v1route.V2APIPath,
		v1route.V2DocPath,
		"/" + jwt.JWKSPath,
	}
	for _, v := range apiPaths {
		err = service.MyService.Gateway().CreateRoute(&model.Route{
			Path:   v,
			Target: "http://" + listener.Addr().String(),
		})

		if err != nil {
			panic(err)
		}
	}

	// write address file
	addressFilePath, err := writeAddressFile(config.CommonInfo.RuntimePath, external.UserServiceAddressFilename, "http://"+listener.Addr().String())
	if err != nil {
		panic(err)
	}

	bgCtx := context.Background()
	if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		_log.Error(bgCtx, "Failed to notify systemd that user service is ready", err)
	} else if supported {
		_log.Info(bgCtx, "Notified systemd that user service is ready")
	} else {
		_log.Info(bgCtx, "This process is not running as a systemd service.")
	}
	pkglifecycle.SafeGo(bgCtx, _log, v1route.EventListen)
	_log.Info(bgCtx, "User service is listening...",
		slog.String("address", listener.Addr().String()),
		slog.String("filepath", addressFilePath))

	var events []message_bus.EventType
	events = append(events, message_bus.EventType{Name: "zimaos:user:save_config", SourceID: common.SERVICENAME, PropertyTypeList: []message_bus.PropertyType{}})
	// register at message bus
	for i := 0; i < 10; i++ {
		response, err := service.MyService.MessageBus().RegisterEventTypesWithResponse(context.Background(), events)
		if err != nil {
			_log.Error(bgCtx, "error when trying to register one or more event types - some event type will not be discoverable", err)
		}
		if response != nil && response.StatusCode() != http.StatusOK {
			_log.Error(bgCtx, "error when trying to register one or more event types - some event type will not be discoverable", nil,
				slog.String("status", response.Status()),
				slog.String("body", string(response.Body)))
		}
		if response.StatusCode() == http.StatusOK {
			break
		}
		time.Sleep(time.Second)
	}

	s := &http.Server{
		Handler:           wrapWithFoundation(mux),
		ReadHeaderTimeout: 5 * time.Second, // fix G112: Potential slowloris attack (see https://github.com/securego/gosec)
	}

	err = s.Serve(listener) // not using http.serve() to fix G114: Use of net/http serve function that has no support for setting timeouts (see https://github.com/securego/gosec)
	if err != nil {
		panic(err)
	}
}

func writeAddressFile(runtimePath string, filename string, address string) (string, error) {
	err := os.MkdirAll(runtimePath, 0o755)
	if err != nil {
		return "", err
	}

	filepath := filepath.Join(runtimePath, filename)
	return filepath, os.WriteFile(filepath, []byte(address), 0o600)
}
