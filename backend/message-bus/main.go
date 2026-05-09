//go:generate bash -c "mkdir -p codegen && go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12.4 -generate types,server,spec -package codegen api/message_bus/openapi.yaml > codegen/message_bus_api.go"

package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/external"
	"github.com/IceWhaleTech/CasaOS-Common/model"
	"github.com/IceWhaleTech/CasaOS-Common/utils/file"
	util_http "github.com/IceWhaleTech/CasaOS-Common/utils/http"
	"github.com/coreos/go-systemd/daemon"
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/common"
	"github.com/neochaotic/powerlab/backend/message-bus/config"
	"github.com/neochaotic/powerlab/backend/message-bus/repository"
	"github.com/neochaotic/powerlab/backend/message-bus/route"
	"github.com/neochaotic/powerlab/backend/message-bus/service"
	pkgfoundation "github.com/neochaotic/powerlab/backend/pkg/foundation"
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
)

// _log is the package-level logger used by main.go's call sites and
// shared with route/service/repository via SetLogger.
var _log pkglogging.Logger

// wrapWithFoundation wraps any http.Handler with PowerLab's foundation
// middleware: tracing.Middleware (correlation IDs in/out via X-Request-Id)
// → lifecycle.RecoverMiddleware (panic recovery → structured 500).
//
// Mirrors the gateway's wrapWithFoundation; same playbook applied to
// the message-bus's two http.Server instances (HTTP listener + UDS
// socket listener — both share the same mux).
//
// Legacy `_log.Info(context.Background(), ...)`-style call sites scattered across
// message-bus code remain on CasaOS-Common's logger for now;
// call-site migration is part 3 of the message-bus kill series.
func wrapWithFoundation(h http.Handler, logger pkglogging.Logger) http.Handler {
	return pkgfoundation.Wrap(h, logger)
}

const localhost = "127.0.0.1"

var (
	commit = "private build"
	date   = "private build"

	//go:embed api/index.html
	_docHTML string

	//go:embed api/message_bus/openapi.yaml
	_docYAML string

	//go:embed build/sysroot/etc/powerlab/message-bus.conf.sample
	_confSample string

	unixSocketPath = "/tmp/message-bus.sock"
)

func main() {
	// arguments
	configFlag := flag.String("c", "", "config file path")
	versionFlag := flag.Bool("v", false, "version")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("v%s\n", common.MessageBusVersion)
		os.Exit(0)
	}

	println("git commit:", commit)
	println("build date:", date)

	// initialization
	config.InitSetup(*configFlag, _confSample)

	// Foundation logger replaces the legacy CasaOS logger.LogInit
	// path. All production call sites in this service now use _log.
	// Constructed early so the rest of main() and the route/service/
	// repository packages can share the same instance.
	{
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
			fl, _ = pkglogging.New(pkglogging.Config{})
		}
		_log = fl
		route.SetLogger(_log)
		service.SetLogger(_log)
		repository.SetLogger(_log)
	}

	// repository
	if err := file.IsNotExistMkDir(config.CommonInfo.RuntimePath); err != nil {
		panic(err)
	}

	databaseFilePath := filepath.Join(config.CommonInfo.RuntimePath, "message-bus.db")
	persistDatabaseFilePath := filepath.Join(config.AppInfo.DBPath, "db", "message-bus.db")
	repository, err := repository.NewDatabaseRepository(databaseFilePath, persistDatabaseFilePath)
	if err != nil {
		panic(err)
	}
	defer repository.Close()

	// service
	services := service.NewServices(&repository)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	services.Start(&ctx)
	go services.YSKService.Start(true)

	// route
	swagger, err := codegen.GetSwagger()
	if err != nil {
		panic(err)
	}

	apiRouter, err := route.NewAPIRouter(swagger, &services)
	if err != nil {
		panic(err)
	}

	docRouter, err := route.NewDocRouter(swagger, _docHTML, _docYAML)
	if err != nil {
		panic(err)
	}

	mux := &util_http.HandlerMultiplexer{
		HandlerMap: map[string]http.Handler{
			"v2":  apiRouter,
			"doc": docRouter,
		},
	}

	// http listener
	listener, err := net.Listen("tcp", net.JoinHostPort(localhost, "0"))
	if err != nil {
		panic(err)
	}

	// remove unix socket file. don't need check whether it exists or not
	os.Remove(unixSocketPath)
	// socket listener
	socketListener, err := net.Listen("unix", unixSocketPath)
	if err != nil {
		panic(err)
	}

	// register at gateway
	u, err := url.Parse(swagger.Servers[0].URL)
	if err != nil {
		panic(err)
	}

	apiPath := strings.TrimRight(u.Path, "/")
	apiPaths := []string{apiPath, "/doc" + apiPath}

	gatewayManagement, err := external.NewManagementService(config.CommonInfo.RuntimePath)
	if err != nil {
		panic(err)
	}

	for _, apiPath := range apiPaths {
		err = gatewayManagement.CreateRoute(&model.Route{
			Path:   apiPath,
			Target: "http://" + listener.Addr().String(),
		})

		if err != nil {
			panic(err)
		}
	}

	// write address file
	addressFilePath, err := writeAddressFile(config.CommonInfo.RuntimePath, external.MessageBusAddressFilename, "http://"+listener.Addr().String())
	if err != nil {
		panic(err)
	}

	// notify systemd
	if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		_log.Error(context.Background(), "Failed to notify systemd that message bus service is ready", err)
	} else if supported {
		_log.Info(context.Background(), "Notified systemd that message bus service is ready")
	} else {
		_log.Info(context.Background(), "This process is not running as a systemd service.")
	}

	// start http server
	_log.Info(context.Background(), "MessageBus service is listening...", slog.Any("address", listener.Addr().String()), slog.String("filepath", addressFilePath))

	wrappedMux := wrapWithFoundation(mux, _log)

	server := &http.Server{
		Handler:           wrappedMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	socketServer := &http.Server{
		Handler:           wrappedMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	httpServerErrChan := make(chan error, 1)
	socketServerErrChan := make(chan error, 1)

	go func() {
		err := server.Serve(listener)
		httpServerErrChan <- err
	}()

	go func() {
		err := socketServer.Serve(socketListener)
		socketServerErrChan <- err
	}()

	select {
	case err := <-httpServerErrChan:
		if err != nil {
			_log.Info(context.Background(), "MessageBus service is stopped", slog.Any("error", err))
			panic(err)
		}
	case err := <-socketServerErrChan:
		if err != nil {
			_log.Info(context.Background(), "MessageBus socket service is stopped", slog.Any("error", err))
			panic(err)
		}
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
