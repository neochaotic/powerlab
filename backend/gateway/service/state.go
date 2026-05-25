package service

import (
	"io/fs"
	"os"

	"github.com/neochaotic/powerlab/backend/gateway/web"
)

// State is the gateway's in-memory runtime state. Holds the active
// gateway port (the one the HTTP listener bound to), an observer
// stack notified on port changes, and PowerLab-owned paths the
// gateway needs to know about (runtime sockets dir, embedded SPA
// www dir).
type State struct {
	gatewayPort         string
	onGatewayPortChange []func(string) error

	runtimePath string
	wwwPath     string
}

// NewState constructs a fresh runtime state with empty defaults.
// Callers populate via Set* methods after config load.
func NewState() *State {
	return &State{
		gatewayPort:         "",
		onGatewayPortChange: make([]func(string) error, 0),

		runtimePath: "",
		wwwPath:     "",
	}
}

func (c *State) SetGatewayPort(port string) (err error) {
	defer func() {
		if err == nil {
			c.gatewayPort = port
		}
	}()
	return c.notifyOnGatewayPortChange(port)
}

func (c *State) GetGatewayPort() string {
	return c.gatewayPort
}

// Add func `f` to the stack. The stack of funcs will be called, in reverse order, when there is request to change the port.
func (c *State) OnGatewayPortChange(f func(string) error) {
	c.onGatewayPortChange = append(c.onGatewayPortChange, f)
}

func (c *State) notifyOnGatewayPortChange(port string) error {
	for i := len(c.onGatewayPortChange) - 1; i >= 0; i-- {
		if err := c.onGatewayPortChange[i](port); err != nil {
			return err
		}
	}

	return nil
}

func (c *State) SetRuntimePath(path string) error {
	c.runtimePath = path
	return nil
}

func (c *State) GetRuntimePath() string {
	return c.runtimePath
}

func (c *State) SetWWWPath(path string) error {
	c.wwwPath = path
	return nil
}

func (c *State) GetWWWPath() string {
	return c.wwwPath
}

// GetWWWFS returns the filesystem the static route serves the UI from.
// Precedence (ADR-0043): if wwwPath points at an existing directory on
// disk, serve from there — this covers the dev/debug `-w` override and
// the legacy on-disk install layout. Otherwise serve the UI embedded in
// the binary. Disk-wins-when-present keeps the migration reversible:
// drop the directory (and the systemd `-w` flag) and the binary serves
// its own embedded copy, with no code change.
func (c *State) GetWWWFS() fs.FS {
	if c.diskWWWDir() != "" {
		return os.DirFS(c.wwwPath)
	}
	return web.FS()
}

// GetWWWMode reports which source GetWWWFS resolves to, for boot-time
// logging: "disk:<path>" when an on-disk bundle wins, else "embedded".
func (c *State) GetWWWMode() string {
	if dir := c.diskWWWDir(); dir != "" {
		return "disk:" + dir
	}
	return "embedded"
}

// diskWWWDir returns wwwPath if it is a real, existing directory, else
// "". Single source of truth for the disk-vs-embed precedence so
// GetWWWFS and GetWWWMode cannot disagree.
func (c *State) diskWWWDir() string {
	if c.wwwPath == "" {
		return ""
	}
	if info, err := os.Stat(c.wwwPath); err == nil && info.IsDir() {
		return c.wwwPath
	}
	return ""
}
