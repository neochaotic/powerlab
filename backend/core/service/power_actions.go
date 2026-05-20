// Package service — power actions for the Settings → Power pane (#260).
//
// Exposes a tightly-scoped allow-list of operations that can be performed
// against the host OR against an individual PowerLab systemd unit. Every
// path is hardcoded; nothing in this file accepts an arbitrary service
// name from the caller. The whitelist is the security boundary.

package service

import (
	"fmt"
	"os/exec"
	"strings"
)

// PowerLabServices is the whitelist of systemd units the Power pane is
// allowed to restart. Defined in code (not config) so a misbehaving
// settings page can't trick this endpoint into restarting random host
// services (e.g. ssh, postgres) — an attacker who controls the request
// payload still can't escape this list.
//
// Mirrors the units declared in scripts/package-linux.sh systemd dir.
var PowerLabServices = []string{
	"powerlab-gateway",
	"powerlab-app-management",
	"powerlab-core",
	"powerlab-user-service",
	"powerlab-local-storage",
	"powerlab-message-bus",
}

// IsAllowedPowerLabService reports whether name matches an entry in
// PowerLabServices exactly. Case-sensitive — systemd unit names are
// case-sensitive.
func IsAllowedPowerLabService(name string) bool {
	for _, s := range PowerLabServices {
		if s == name {
			return true
		}
	}
	return false
}

// ServiceState is the minimal status snapshot the Power pane surfaces
// per service. Derived from `systemctl show --property=ActiveState,SubState`.
type ServiceState struct {
	Name        string `json:"name"`
	ActiveState string `json:"active_state"` // active | inactive | failed | activating | deactivating
	SubState    string `json:"sub_state"`    // running | dead | exited | start-pre | ...
	Pid         string `json:"pid,omitempty"`
}

// commandRunner is the test seam: production wires this to
// `exec.Command(...).CombinedOutput()`; tests inject a stub.
type commandRunner func(name string, args ...string) (output []byte, err error)

var defaultRunner commandRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput() //nolint:gosec
}

// queryServiceStateWith returns the current state of a PowerLab
// systemd unit. The unit name MUST come from PowerLabServices —
// pass any other string and the function returns an error rather
// than shelling out. Production callers reach this via
// QueryAllServiceStates; tests inject their own commandRunner stub.
func queryServiceStateWith(run commandRunner, name string) (ServiceState, error) {
	if !IsAllowedPowerLabService(name) {
		return ServiceState{}, fmt.Errorf("service %q not in PowerLab whitelist", name)
	}
	out, err := run("systemctl", "show", name,
		"--property=ActiveState",
		"--property=SubState",
		"--property=MainPID",
		"--value", "--no-pager")
	if err != nil {
		return ServiceState{}, fmt.Errorf("systemctl show %s: %w (output: %s)", name, err, string(out))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	// `--value` returns the values in the SAME order as the
	// --property flags. ActiveState, SubState, MainPID.
	state := ServiceState{Name: name}
	if len(lines) >= 1 {
		state.ActiveState = strings.TrimSpace(lines[0])
	}
	if len(lines) >= 2 {
		state.SubState = strings.TrimSpace(lines[1])
	}
	if len(lines) >= 3 {
		pid := strings.TrimSpace(lines[2])
		if pid != "0" {
			state.Pid = pid
		}
	}
	return state, nil
}

// QueryAllServiceStates returns ServiceState for every PowerLab unit.
// Best-effort: a single systemctl failure is logged in the returned
// error slice but doesn't abort the loop — caller still gets states
// for every queryable service.
func QueryAllServiceStates() ([]ServiceState, []error) {
	return queryAllServiceStatesWith(defaultRunner)
}

func queryAllServiceStatesWith(run commandRunner) ([]ServiceState, []error) {
	states := make([]ServiceState, 0, len(PowerLabServices))
	var errs []error
	for _, name := range PowerLabServices {
		s, err := queryServiceStateWith(run, name)
		if err != nil {
			errs = append(errs, err)
			// Still include a placeholder so the UI knows the unit
			// exists even if systemctl couldn't read its state.
			states = append(states, ServiceState{Name: name, ActiveState: "unknown"})
			continue
		}
		states = append(states, s)
	}
	return states, errs
}

// GatewayService is the systemd unit name for the PowerLab gateway.
// Restarting it requires the delayed-exec path (see restartPowerLabServiceWith).
const GatewayService = "powerlab-gateway"

// RestartPowerLabService restarts a single PowerLab unit. The unit
// MUST be in PowerLabServices. For the gateway service itself, the
// restart is forked via systemd-run so the HTTP response can be sent
// before the process is torn down; all other units restart synchronously.
func RestartPowerLabService(name string) ([]byte, error) {
	return restartPowerLabServiceWith(defaultRunner, name)
}

func restartPowerLabServiceWith(run commandRunner, name string) ([]byte, error) {
	if !IsAllowedPowerLabService(name) {
		return nil, fmt.Errorf("service %q not in PowerLab whitelist", name)
	}
	if name == GatewayService {
		// The gateway is restarting itself. A direct `systemctl restart`
		// would kill this process (and its cgroup) before the HTTP
		// response returns. systemd-run spawns a transient unit in a
		// separate cgroup so the restart fires ~2 s after we reply.
		out, err := run("systemd-run", "--no-block", "--quiet",
			"/bin/sh", "-c", "sleep 2 && systemctl restart "+name) //nolint:gosec
		if err != nil {
			return out, fmt.Errorf("systemd-run delayed restart %s: %w (output: %s)", name, err, string(out))
		}
		return out, nil
	}
	out, err := run("systemctl", "restart", name)
	if err != nil {
		return out, fmt.Errorf("systemctl restart %s: %w (output: %s)", name, err, string(out))
	}
	return out, nil
}

// ServiceEnabledState captures whether a PowerLab unit is enabled in systemd.
// Used by the /v1/sys/services/preflight endpoint so the UI can show a
// warning before restarting a service that would interrupt the user's session.
type ServiceEnabledState struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// QueryAllServiceEnabled returns the enabled/disabled state of every
// PowerLab unit. Best-effort: a systemctl failure marks the unit as
// disabled rather than aborting the loop.
func QueryAllServiceEnabled() []ServiceEnabledState {
	return queryAllServiceEnabledWith(defaultRunner)
}

func queryAllServiceEnabledWith(run commandRunner) []ServiceEnabledState {
	result := make([]ServiceEnabledState, len(PowerLabServices))
	for i, name := range PowerLabServices {
		enabled, _ := queryServiceEnabledWith(run, name)
		result[i] = ServiceEnabledState{Name: name, Enabled: enabled}
	}
	return result
}

// queryServiceEnabledWith calls `systemctl is-enabled --quiet <name>` and
// returns true if exit code is 0 (enabled), false otherwise. The unit name
// MUST be in PowerLabServices — anything else returns an error without
// shelling out.
func queryServiceEnabledWith(run commandRunner, name string) (bool, error) {
	if !IsAllowedPowerLabService(name) {
		return false, fmt.Errorf("service %q not in PowerLab whitelist", name)
	}
	_, err := run("systemctl", "is-enabled", "--quiet", name)
	return err == nil, nil
}

// RebootHost runs `systemctl reboot`. No payload, no flags — this is
// a destructive operation, the caller's handler is responsible for
// confirmation prompts + auth. Per `feedback_security_is_priority`,
// the handler should gate this behind admin role + explicit confirm
// token in the request.
func RebootHost() ([]byte, error) {
	return rebootHostWith(defaultRunner)
}

func rebootHostWith(run commandRunner) ([]byte, error) {
	out, err := run("systemctl", "reboot")
	if err != nil {
		return out, fmt.Errorf("systemctl reboot: %w (output: %s)", err, string(out))
	}
	return out, nil
}

// ShutdownHost runs `systemctl poweroff`. Same caveats as RebootHost.
func ShutdownHost() ([]byte, error) {
	return shutdownHostWith(defaultRunner)
}

func shutdownHostWith(run commandRunner) ([]byte, error) {
	out, err := run("systemctl", "poweroff")
	if err != nil {
		return out, fmt.Errorf("systemctl poweroff: %w (output: %s)", err, string(out))
	}
	return out, nil
}
