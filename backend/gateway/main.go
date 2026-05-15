package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/model"
	"github.com/neochaotic/powerlab/backend/common/utils/audit"
	"github.com/neochaotic/powerlab/backend/common/utils/constants"
	"github.com/neochaotic/powerlab/backend/common/utils/devmode"
	http2 "github.com/neochaotic/powerlab/backend/common/utils/http"
	"github.com/coreos/go-systemd/daemon"

	"github.com/neochaotic/powerlab/backend/common/pkg/security"
	"github.com/neochaotic/powerlab/backend/gateway/common"
	"github.com/neochaotic/powerlab/backend/gateway/route"
	"github.com/neochaotic/powerlab/backend/gateway/service"
	pkgfoundation "github.com/neochaotic/powerlab/backend/pkg/foundation"
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
	"go.uber.org/fx"
)

// _log is the PowerLab-owned logger used by the
// foundation middleware (panic recovery + correlation-ID tracing).
// Constructed once in main(), passed to wrapWithFoundation.
//
// The legacy CasaOS `_log.Info(context.Background(), ...)`-style calls scattered across
// gateway code remain for now; the call-site migration is part 3 of
// the gateway kill series. Part 2 (this) only wires the middleware
// so panics get caught and correlation IDs propagate.
var _log pkglogging.Logger

// wrapWithFoundation delegates to pkg/foundation.Wrap so the gateway
// shares the canonical middleware chain (tracing + recover) with
// every other PowerLab service. See pkg/foundation for the
// composition contract — this thin shim exists only because
// callers reference wrapWithFoundation in many places and renaming
// would churn the diff.
func wrapWithFoundation(h http.Handler) http.Handler {
	return pkgfoundation.Wrap(h, _log)
}

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

	//go:embed build/sysroot/etc/powerlab/gateway.ini.sample
	_confSample string
)

func init() {
	// Skip the entire init in test binaries. init() here does heavy
	// startup work (flag.Parse, config load, logger init, _state setup)
	// that's not appropriate during `go test` — flag.Parse in particular
	// fails the test binary because it doesn't recognize -test.* flags.
	// Production main() calls runMain() which still does this setup.
	if strings.HasSuffix(os.Args[0], ".test") || strings.Contains(os.Args[0], "/_test/") {
		return
	}

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

	// PowerLab-owned logger. Replaces the CasaOS-Common
	// utils/logger global init that lived here in part 2 (file
	// rotation via lumberjack on disk; the new logger emits to
	// stdout/journalctl which is the production pattern anyway).
	// All call sites in this service now go through this logger
	// (part 3 of the gateway kill series, ADR-0016).
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
		// Fall back to a permissive default rather than crash init.
		fl, _ = pkglogging.New(pkglogging.Config{})
	}
	_log = fl
	// Wire the same instance into the route and service packages so
	// every log line in the gateway flows through one logger.
	route.SetLogger(_log)
	service.SetLogger(_log)

	runtimePath := config.GetString(common.ConfigKeyRuntimePath)
	if err := _state.SetRuntimePath(runtimePath); err != nil {
		_log.Error(context.Background(), "Failed to set runtime path", err, slog.Any(common.ConfigKeyRuntimePath, runtimePath))
		panic(err)
	}

	gatewayPort := config.GetString(common.ConfigKeyGatewayPort)
	if err := _state.SetGatewayPort(gatewayPort); err != nil {
		_log.Error(context.Background(), "Failed to set gateway port", err, slog.Any(common.ConfigKeyGatewayPort, gatewayPort))
		panic(err)
	}

	if err := _state.SetWWWPath(*wwwPathFlag); err != nil {
		_log.Error(context.Background(), "Failed to set www path", err, slog.String("wwwpath", *wwwPathFlag))
		panic(err)
	}

	if err := checkPrequisites(_state); err != nil {
		_log.Error(context.Background(), "Failed to check prequisites", err)
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
		_log.Error(context.Background(), "Failed to write pid file to runtime path", err, slog.Any("runtimePath", _state.GetRuntimePath()))
		panic(err)
	}

	defer cleanupFiles(
		_state.GetRuntimePath(),
		pidFilename, external.ManagementURLFilename, external.StaticURLFilename,
	)

	defer func() {
		if _gateway != nil {
			if err := _gateway.Shutdown(context.Background()); err != nil {
				_log.Error(context.Background(), "Failed to stop gateway", err)
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
			_log.Error(context.Background(), "Failed to notify systemd that gateway is ready", err)
		} else if supported {
			_log.Info(context.Background(), "Notified systemd that gateway is ready")
		} else {
			_log.Info(context.Background(), "This process is not running as a systemd service.")
		}
	}()

	// CertManager is created here (not via fx.Provide) so the daily
	// ticker survives the same lifetime as the process — fx graphs
	// reset on app.Stop. Initialize once + go.
	_certmgr = security.NewCertManager(_state.GetRuntimePath())
	if err := _certmgr.Setup(); err != nil {
		_log.Error(context.Background(), "CertManager setup failed (HTTPS will be unavailable)", err)
		// Non-fatal: gateway still serves HTTP. The user just gets
		// no HTTPS until the next boot resolves whatever broke us.
	} else {
		_certmgr.StartTicker(_certStop)
	}

	// Audit pipeline — ADR-0033 (middleware shape) + ADR-0035
	// (JSONL storage). Init eagerly so any failure surfaces in the
	// boot log before fx wires routes. Non-fatal on init failure:
	// gateway still serves traffic without the audit log (matches
	// CertManager pattern above).
	//
	// Path is /var/log/powerlab/audit.jsonl — operator-friendly
	// (grep + tail -F + jq from SSH), rotated by lumberjack into
	// .1.gz, .2.gz, ... when MaxSize is exceeded.
	var _auditSvc *audit.Service
	{
		auditPath := "/var/log/powerlab/audit.jsonl"
		svc, err := audit.NewService(audit.ServiceOptions{Path: auditPath})
		if err != nil {
			_log.Error(context.Background(), "audit.NewService failed (audit log disabled)", err)
		} else {
			_auditSvc = svc
		}
	}

	app := fx.New(
		fx.Provide(func() *service.State { return _state }),
		fx.Provide(func() *security.CertManager { return _certmgr }),
		fx.Provide(func() *audit.Service { return _auditSvc }),
		fx.Provide(service.NewManagementService),
		fx.Provide(route.NewManagementRoute),
		fx.Provide(route.NewGatewayRoute),
		fx.Provide(route.NewStaticRoute),
		fx.Invoke(run),
	)

	if err := app.Start(ctx); err != nil {
		if err != context.Canceled {
			_log.Error(context.Background(), "Failed to start gateway", err)
		}
	}
}

func run(
	lifecycle fx.Lifecycle,
	management *service.Management,
	managementRoute *route.ManagementRoute,
	gatewayRoute *route.GatewayRoute,
	staticRoute *route.StaticRoute,
	auditSvc *audit.Service,
) {
	// Audit lifecycle (Sprint 16 B1c). OnStop drains the writer
	// goroutine, halts the retention loop, and closes the DB. Nil
	// when audit failed to init at startup — Close is nil-safe.
	if auditSvc != nil {
		lifecycle.Append(fx.Hook{
			OnStop: func(context.Context) error {
				return auditSvc.Close()
			},
		})
	}

	// management server
	lifecycle.Append(
		fx.Hook{
			OnStart: func(context.Context) error {
				listener, err := net.Listen("tcp", net.JoinHostPort(localhost, "0"))
				if err != nil {
					return err
				}

				managementServer := &http.Server{
					Handler:           wrapWithFoundation(managementRoute.GetRoute()),
					ReadHeaderTimeout: 5 * time.Second,
				}

				urlFilePath, err := writeAddressFile(_state.GetRuntimePath(), external.ManagementURLFilename, "http://"+listener.Addr().String())
				if err != nil {
					return err
				}

				go func() {
					_log.Info(context.Background(), "Management service is listening...",
						slog.Any("address", listener.Addr().String()),
						slog.Any("filepath", urlFilePath),
					)
					err := managementServer.Serve(listener)
					if err != nil {
						_log.Error(context.Background(), "management server error", err)
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
						_log.Info(context.Background(), "Checking if port is available...", slog.Any("port", port))
						if listener, err := net.Listen("tcp", net.JoinHostPort("", port)); err == nil {
							if err = listener.Close(); err != nil {
								_log.Error(context.Background(), "Failed to close listener", err, slog.Any("port", port))
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
						_log.Info(context.Background(), "Failed to announce mDNS service (non-fatal)",
							slog.Any("error", err),
							slog.Any("port", portInt),
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
				_log.Info(context.Background(), "HTTPS skipped — CertManager not initialized")
				return nil
			}
			serverCert, serverKey := _certmgr.GetServerPaths()
			if _, err := os.Stat(serverCert); err != nil {
				_log.Info(context.Background(), "HTTPS skipped — server cert not yet available",
					slog.String("path", serverCert),
					slog.Any("error", err))
				return nil
			}

			route := gatewayRoute.WrapHSTS(gatewayRoute.GetRoute(), HTTPSPort)
			_https = &http.Server{
				Addr:              ":" + HTTPSPort,
				Handler:           wrapWithFoundation(route),
				ReadHeaderTimeout: 5 * time.Second,
			}

			go func() {
				_log.Info(context.Background(), "HTTPS gateway is listening...",
					slog.String("addr", _https.Addr),
					slog.String("cert", serverCert),
				)
				if err := _https.ListenAndServeTLS(serverCert, serverKey); err != nil && err != http.ErrServerClosed {
					_log.Error(context.Background(), "HTTPS gateway error", err)
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
				Handler:           wrapWithFoundation(staticRoute.GetRoute()),
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

			_log.Info(context.Background(),
				"Static web service is listening...",
				slog.Any("address", listener.Addr().String()),
				slog.Any("filepath", urlFilePath),
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
		_log.Info(context.Background(), "Port is the same as current running gateway - no change is required")
		return nil
	}

	// start new gateway
	gatewayNew := &http.Server{
		Addr:              addr,
		Handler:           wrapWithFoundation(route),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		err := gatewayNew.Serve(listener)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				_log.Info(context.Background(), "A gateway is stopped", slog.Any("address", gatewayNew.Addr))
				return
			}
			_log.Error(context.Background(), "Error when serving a gateway", err, slog.Any("address", gatewayNew.Addr))
		}
	}()

	// test if gateway is running. listener.Addr() returns the BIND
	// address (e.g. "[::]:8765" or "0.0.0.0:8765") which is invalid
	// as a CLIENT destination — http.Get to "[::]:PORT" fails on
	// IPv6-strict configs. Substitute the loopback before pinging.
	checkURL := "http://" + clientLoopback(addr) + "/ping"
	if err := checkURLWithRetry(checkURL, 10); err != nil {
		return err
	}

	_log.Info(context.Background(), "New gateway is listening...", slog.Any("address", gatewayNew.Addr))

	// stop old gateway
	if _gateway != nil {
		gatewayOld := _gateway
		go func() {
			_log.Info(context.Background(), "Stopping previous gateway in 1 seconds...", slog.Any("address", gatewayOld.Addr))
			time.Sleep(time.Second) // so that any request to the old gateway gets a response
			if err := gatewayOld.Shutdown(context.Background()); err != nil {
				_log.Error(context.Background(), "Error when stopping previous gateway", err, slog.Any("address", gatewayOld.Addr))
			}
		}()
	}

	_gateway = gatewayNew

	return nil
}

// clientLoopback rewrites a server bind address to a client-routable
// loopback address. listener.Addr() returns the BIND address (e.g.
// "[::]:8765" or "0.0.0.0:8765") — neither is valid as a TCP client
// destination on IPv6-strict configs.  Without this rewrite, a
// self-ping can fail with connection-refused even though the server
// is listening on every interface.
func clientLoopback(addr string) string {
	switch {
	case strings.HasPrefix(addr, "[::]:"):
		return "127.0.0.1:" + strings.TrimPrefix(addr, "[::]:")
	case strings.HasPrefix(addr, "0.0.0.0:"):
		return "127.0.0.1:" + strings.TrimPrefix(addr, "0.0.0.0:")
	default:
		return addr
	}
}

// checkURLWithRetry retries checkURL up to `retry` times with a 1s
// sleep between attempts. Returns the last error, or nil on first
// success.
//
// `retry` is `int` deliberately. Previously this was `uint` with the
// loop condition `count >= 0`, which is ALWAYS true for unsigned
// integers — `count--` from 0 wraps to MAX_UINT64, making the loop
// run forever. Combined with the boot-time self-ping bug (the URL
// pointed at a non-routable bind address), this turned the gateway
// into a 100% CPU spinner that never finished startup. Locked in
// by gateway/main_check_url_test.go.
func checkURLWithRetry(url string, retry int) error {
	var err error
	for i := 0; i <= retry; i++ {
		_log.Info(context.Background(), "Checking if service at URL is running...", slog.Any("url", url), slog.Any("attempt", i+1), slog.Any("max", retry+1))
		if err = checkURL(url); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}

// checkURL pings url and returns nil if any HTTP response was received
// (including 301/302 — the gateway's HTTP→HTTPS redirect), or the
// underlying transport error if the server didn't respond at all.
// The boot path is the most common caller — it retries via
// checkURLWithRetry while services come up.
//
// Bug-#64 history: this function used to nil-deref on the err != nil
// branch because the original code wrote `if err == nil { return err }`
// then unconditionally `defer response.Body.Close()`, which crashed
// when http.Get returned (nil, err). Fixed here while DELIBERATELY
// preserving the original "any response is up" semantics — the
// gateway's HTTP listener returns 301 for /ping (redirect to HTTPS),
// and a stricter "must be 200" check would loop forever during boot
// because the HTTPS listener hasn't started yet.
func checkURL(url string) error {
	response, err := http2.Get(url, 5*time.Second)
	if err != nil {
		return err
	}
	_ = response.Body.Close()
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
			_log.Error(context.Background(), "Failed to cleanup file", err, slog.Any("filename", filename))
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
