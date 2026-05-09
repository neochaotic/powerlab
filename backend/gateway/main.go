package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/external"
	"github.com/IceWhaleTech/CasaOS-Common/model"
	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS-Common/utils/devmode"
	http2 "github.com/IceWhaleTech/CasaOS-Common/utils/http"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/coreos/go-systemd/daemon"

	"github.com/IceWhaleTech/CasaOS-Common/pkg/security"
	"github.com/neochaotic/powerlab/backend/gateway/common"
	"github.com/neochaotic/powerlab/backend/gateway/route"
	"github.com/neochaotic/powerlab/backend/gateway/service"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const localhost = "127.0.0.1"

// HTTPSPort is the port the gateway binds for HTTPS. Hard-coded for
// v0.2.7 (configurable via gateway.ini comes in v0.2.8). Picked
// 8443 because :443 requires CAP_NET_BIND_SERVICE and we already
// avoid that on the HTTP side (default :8765).
const HTTPSPort = "8443"

var (
	commit = "private build"
	date   = "private build"

	_state    *service.State
	_gateway  *http.Server
	_https    *http.Server
	_certmgr  *security.CertManager
	_certStop = make(chan struct{})
	_mdns     = service.NewMDNSService("powerlab")

	_managementServiceReady = make(chan struct{})
	_gatewayServiceReady    = make(chan struct{})

	ErrCheckURLNotOK = errors.New("check url did not return 200 OK")

	//go:embed build/sysroot/etc/casaos/gateway.ini.sample
	_confSample string
)

func init() {
	versionFlag := flag.Bool("v", false, "version")
	wwwPathFlag := flag.String("w", filepath.Join(constants.DefaultDataPath, "www"), "www path")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("v%s\n", common.Version)
		os.Exit(0)
	}

	println("git commit:", commit)
	println("build date:", date)

	_state = service.NewState()

	// create default config file if not exist
	configPath := constants.DefaultConfigPath
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if currentDir, err := os.Getwd(); err == nil {
			configPath = currentDir
		}
	}
	ConfigFilePath := filepath.Join(configPath, common.GatewayName+"."+common.GatewayConfigType)
	if _, err := os.Stat(ConfigFilePath); os.IsNotExist(err) {
		fmt.Println("config file not exist, create it")
		// create config file
		file, err := os.Create(ConfigFilePath)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		// write default config
		_, err = file.WriteString(_confSample)
		if err != nil {
			panic(err)
		}
	}

	config, err := common.LoadConfig()
	if err != nil {
		panic(err)
	}

	// Dev-only: redirect runtime/log paths into the project tree so multiple
	// services can share a sandbox under `./start.sh`. In production
	// (/etc/powerlab present), trust the loaded config file as-is —
	// otherwise systemd's cwd of "/" would make us write to /runtime.
	if devmode.IsDev() {
		if currentDir, err := os.Getwd(); err == nil {
			sharedRuntime := filepath.Join(filepath.Dir(currentDir), "runtime")
			config.Set(common.ConfigKeyRuntimePath, sharedRuntime)
			config.Set(common.ConfigKeyLogPath, filepath.Join(filepath.Dir(currentDir), "logs"))
		}
	}

	logger.LogInit(
		config.GetString(common.ConfigKeyLogPath),
		config.GetString(common.ConfigKeyLogSaveName),
		config.GetString(common.ConfigKeyLogFileExt),
	)

	runtimePath := config.GetString(common.ConfigKeyRuntimePath)
	if err := _state.SetRuntimePath(runtimePath); err != nil {
		logger.Error("Failed to set runtime path", zap.Any("error", err), zap.Any(common.ConfigKeyRuntimePath, runtimePath))
		panic(err)
	}

	gatewayPort := config.GetString(common.ConfigKeyGatewayPort)
	if err := _state.SetGatewayPort(gatewayPort); err != nil {
		logger.Error("Failed to set gateway port", zap.Any("error", err), zap.Any(common.ConfigKeyGatewayPort, gatewayPort))
		panic(err)
	}

	if err := _state.SetWWWPath(*wwwPathFlag); err != nil {
		logger.Error("Failed to set www path", zap.Any("error", err), zap.String("wwwpath", *wwwPathFlag))
		panic(err)
	}

	if err := checkPrequisites(_state); err != nil {
		logger.Error("Failed to check prequisites", zap.Any("error", err))
		panic(err)
	}

	_state.OnGatewayPortChange(func(port string) error {
		config.Set(common.ConfigKeyGatewayPort, port)
		return config.WriteConfig()
	})
}

func main() {
	pidFilename, err := writePidFile(_state.GetRuntimePath())
	if err != nil {
		logger.Error("Failed to write pid file to runtime path", zap.Any("error", err), zap.Any("runtimePath", _state.GetRuntimePath()))
		panic(err)
	}

	defer cleanupFiles(
		_state.GetRuntimePath(),
		pidFilename, external.ManagementURLFilename, external.StaticURLFilename,
	)

	defer func() {
		if _gateway != nil {
			if err := _gateway.Shutdown(context.Background()); err != nil {
				logger.Error("Failed to stop gateway", zap.Any("error", err))
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	kill := make(chan os.Signal, 1)
	signal.Notify(kill, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-kill
		cancel()
	}()

	go func() {
		<-_managementServiceReady
		<-_gatewayServiceReady

		if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
			logger.Error("Failed to notify systemd that gateway is ready", zap.Any("error", err))
		} else if supported {
			logger.Info("Notified systemd that gateway is ready")
		} else {
			logger.Info("This process is not running as a systemd service.")
		}
	}()

	// CertManager is created here (not via fx.Provide) so the daily
	// ticker survives the same lifetime as the process — fx graphs
	// reset on app.Stop. Initialize once + go.
	_certmgr = security.NewCertManager(_state.GetRuntimePath())
	if err := _certmgr.Setup(); err != nil {
		logger.Error("CertManager setup failed (HTTPS will be unavailable)", zap.Error(err))
		// Non-fatal: gateway still serves HTTP. The user just gets
		// no HTTPS until the next boot resolves whatever broke us.
	} else {
		_certmgr.StartTicker(_certStop)
	}

	app := fx.New(
		fx.Provide(func() *service.State { return _state }),
		fx.Provide(func() *security.CertManager { return _certmgr }),
		fx.Provide(service.NewManagementService),
		fx.Provide(route.NewManagementRoute),
		fx.Provide(route.NewGatewayRoute),
		fx.Provide(route.NewStaticRoute),
		fx.Invoke(run),
	)

	if err := app.Start(ctx); err != nil {
		if err != context.Canceled {
			logger.Error("Failed to start gateway", zap.Any("error", err))
		}
	}
}

func run(
	lifecycle fx.Lifecycle,
	management *service.Management,
	managementRoute *route.ManagementRoute,
	gatewayRoute *route.GatewayRoute,
	staticRoute *route.StaticRoute,
) {
	// management server
	lifecycle.Append(
		fx.Hook{
			OnStart: func(context.Context) error {
				listener, err := net.Listen("tcp", net.JoinHostPort(localhost, "0"))
				if err != nil {
					return err
				}

				managementServer := &http.Server{
					Handler:           managementRoute.GetRoute(),
					ReadHeaderTimeout: 5 * time.Second,
				}

				urlFilePath, err := writeAddressFile(_state.GetRuntimePath(), external.ManagementURLFilename, "http://"+listener.Addr().String())
				if err != nil {
					return err
				}

				go func() {
					logger.Info("Management service is listening...",
						zap.Any("address", listener.Addr().String()),
						zap.Any("filepath", urlFilePath),
					)
					err := managementServer.Serve(listener)
					if err != nil {
						logger.Error("management server error", zap.Any("error", err))
						os.Exit(1)
					}
				}()

				if err := management.CreateRoute(&model.Route{
					Path:   "/v1/gateway/port",
					Target: "http://" + listener.Addr().String(),
				}); err != nil {
					return err
				}

				_managementServiceReady <- struct{}{}

				return nil
			},
		},
	)

	// gateway service
	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				// Wrap the mux with the HSTS gate from ADR 0006.
				// HSTS header + HTTP→HTTPS redirect are conditional
				// on /etc/powerlab/tls/.hsts-armed existing (set by
				// POST /v1/sys/trust-confirmed). Pre-trust the
				// gateway behaves as plain HTTP, so the user can
				// finish the trust dance.
				route := gatewayRoute.WrapHSTS(gatewayRoute.GetRoute(), HTTPSPort)

				if _state.GetGatewayPort() == "" {
					// check if a port is available starting from port 80/8080
					portsToCheck := []int{}
					for i := 80; i < 90; i++ {
						portsToCheck = append(portsToCheck, i)
					}

					for i := 8080; i < 8090; i++ {
						portsToCheck = append(portsToCheck, i)
					}

					port := ""
					for _, p := range portsToCheck {
						port = fmt.Sprintf("%d", p)
						logger.Info("Checking if port is available...", zap.Any("port", port))
						if listener, err := net.Listen("tcp", net.JoinHostPort("", port)); err == nil {
							if err = listener.Close(); err != nil {
								logger.Error("Failed to close listener", zap.Any("error", err), zap.Any("port", port))
								continue
							}
							break
						}
					}

					if port == "" {
						return errors.New("No port available for gateway to use")
					}

					if err := _state.SetGatewayPort(port); err != nil {
						return err
					}
				}

				_state.OnGatewayPortChange(func(port string) error {
					return reloadGateway(port, route)
				})

				if err := reloadGateway(_state.GetGatewayPort(), route); err != nil {
					return err
				}

				// Announce the gateway on the local network via mDNS/Bonjour so
				// users can browse to `powerlab.local` instead of an IP address.
				// Failure to announce should NOT break the gateway — log and continue.
				portInt, _ := strconv.Atoi(_state.GetGatewayPort())
				if portInt > 0 {
					if err := _mdns.Announce(portInt); err != nil {
						logger.Info("Failed to announce mDNS service (non-fatal)",
							zap.Any("error", err),
							zap.Any("port", portInt),
						)
					}
				}

				_gatewayServiceReady <- struct{}{}

				return nil
			},
			OnStop: func(context.Context) error {
				_mdns.Shutdown()
				return nil
			},
		})

	// HTTPS listener (port 8443). Wraps the SAME route handler as
	// the HTTP gateway (with HSTS middleware applied), but listens
	// on a TLS socket using the cert managed by CertManager.
	//
	// If CertManager.Setup() failed earlier (cert files missing),
	// _certmgr is nil-safe and we skip starting HTTPS — gateway still
	// serves HTTP. The user can re-trigger setup by restarting after
	// fixing whatever blocked Setup (perm issues on /etc/powerlab/tls,
	// usually).
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if _certmgr == nil {
				logger.Info("HTTPS skipped — CertManager not initialized")
				return nil
			}
			serverCert, serverKey := _certmgr.GetServerPaths()
			if _, err := os.Stat(serverCert); err != nil {
				logger.Info("HTTPS skipped — server cert not yet available",
					zap.String("path", serverCert),
					zap.Error(err))
				return nil
			}

			route := gatewayRoute.WrapHSTS(gatewayRoute.GetRoute(), HTTPSPort)
			_https = &http.Server{
				Addr:              ":" + HTTPSPort,
				Handler:           route,
				ReadHeaderTimeout: 5 * time.Second,
			}

			go func() {
				logger.Info("HTTPS gateway is listening...",
					zap.String("addr", _https.Addr),
					zap.String("cert", serverCert),
				)
				if err := _https.ListenAndServeTLS(serverCert, serverKey); err != nil && err != http.ErrServerClosed {
					logger.Error("HTTPS gateway error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(_certStop) // stops the cert renewal ticker
			if _https != nil {
				_ = _https.Shutdown(ctx)
			}
			return nil
		},
	})

	// static web
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			listener, err := net.Listen("tcp", net.JoinHostPort(localhost, "0"))
			if err != nil {
				return err
			}

			staticServer := &http.Server{
				Handler:           staticRoute.GetRoute(),
				ReadHeaderTimeout: 5 * time.Second,
			}

			target := "http://" + listener.Addr().String()

			urlFilePath, err := writeAddressFile(_state.GetRuntimePath(), external.StaticURLFilename, target)
			if err != nil {
				return err
			}

			if err := management.CreateRoute(&model.Route{
				Path:   "/",
				Target: target,
			}); err != nil {
				return err
			}

			logger.Info(
				"Static web service is listening...",
				zap.Any("address", listener.Addr().String()),
				zap.Any("filepath", urlFilePath),
			)
			return staticServer.Serve(listener)
		},
	})
}

func reloadGateway(port string, route http.Handler) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		return err
	}

	addr := listener.Addr().String()

	if _gateway != nil && _gateway.Addr == addr {
		logger.Info("Port is the same as current running gateway - no change is required")
		return nil
	}

	// start new gateway
	gatewayNew := &http.Server{
		Addr:              addr,
		Handler:           route,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		err := gatewayNew.Serve(listener)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				logger.Info("A gateway is stopped", zap.Any("address", gatewayNew.Addr))
				return
			}
			logger.Error("Error when serving a gateway", zap.Any("error", err), zap.Any("address", gatewayNew.Addr))
		}
	}()

	// test if gateway is running
	url := "http://" + addr + "/ping"
	if err := checkURLWithRetry(url, 10); err != nil {
		return err
	}

	logger.Info("New gateway is listening...", zap.Any("address", gatewayNew.Addr))

	// stop old gateway
	if _gateway != nil {
		gatewayOld := _gateway
		go func() {
			logger.Info("Stopping previous gateway in 1 seconds...", zap.Any("address", gatewayOld.Addr))
			time.Sleep(time.Second) // so that any request to the old gateway gets a response
			if err := gatewayOld.Shutdown(context.Background()); err != nil {
				logger.Error("Error when stopping previous gateway", zap.Any("error", err), zap.Any("address", gatewayOld.Addr))
			}
		}()
	}

	_gateway = gatewayNew

	return nil
}

func checkURLWithRetry(url string, retry uint) error {
	count := retry
	var err error

	for count >= 0 {
		logger.Info("Checking if service at URL is running...", zap.Any("url", url), zap.Any("retry", count))
		if err = checkURL(url); err != nil {
			time.Sleep(time.Second)
			count--
			continue
		}
		break
	}

	return err
}

func checkURL(url string) error {
	response, err := http2.Get(url, 5*time.Second)
	if err == nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		return ErrCheckURLNotOK
	}

	return nil
}

func writePidFile(runtimePath string) (string, error) {
	filename := "gateway.pid"
	filepath := filepath.Join(runtimePath, filename)
	return filename, os.WriteFile(filepath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o600)
}

func writeAddressFile(runtimePath string, filename string, address string) (string, error) {
	err := os.MkdirAll(runtimePath, 0o755)
	if err != nil {
		return "", err
	}

	filepath := filepath.Join(runtimePath, filename)
	return filepath, os.WriteFile(filepath, []byte(address), 0o600)
}

func cleanupFiles(runtimePath string, filenames ...string) {
	for _, filename := range filenames {
		err := os.Remove(filepath.Join(runtimePath, filename))
		if err != nil {
			logger.Error("Failed to cleanup file", zap.Any("error", err), zap.Any("filename", filename))
		}
	}
}

func checkPrequisites(state *service.State) error {
	path := state.GetRuntimePath()

	err := os.MkdirAll(path, 0o755)
	if err != nil {
		return fmt.Errorf("please ensure the owner of this service has write permission to that path %s", path)
	}

	return nil
}
