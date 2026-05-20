package service

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Power-action service tests (#260).
//
// Production wires commandRunner to exec.Command; tests inject a stub
// closure so we never actually invoke systemctl on the CI host.

func TestIsAllowedPowerLabService(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"powerlab-gateway", true},
		{"powerlab-app-management", true},
		{"powerlab-core", true},
		{"powerlab-user-service", true},
		{"powerlab-local-storage", true},
		{"powerlab-message-bus", true},
		// not in whitelist
		{"ssh", false},
		{"postgresql", false},
		{"powerlab-gateway.service", false}, // mustn't accept "with-suffix" form
		{"Powerlab-Gateway", false},         // case-sensitive
		{"", false},
		{"powerlab-../../etc/shadow", false}, // shell-injection bait
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAllowedPowerLabService(tc.name); got != tc.want {
				t.Errorf("IsAllowedPowerLabService(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestRestartPowerLabService_RejectsNonWhitelisted(t *testing.T) {
	// Critical security test: any caller-supplied service name that
	// isn't in PowerLabServices MUST error out BEFORE exec'ing systemctl.
	var called bool
	stub := func(name string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	for _, n := range []string{"", "ssh", "powerlab-gateway.service", "Powerlab-Gateway", "powerlab-../../etc/shadow"} {
		t.Run(n, func(t *testing.T) {
			called = false
			_, err := restartPowerLabServiceWith(stub, n)
			if err == nil {
				t.Errorf("expected error for non-whitelisted %q, got nil", n)
			}
			if called {
				t.Errorf("commandRunner invoked for non-whitelisted %q — SECURITY BUG", n)
			}
		})
	}
}

func TestRestartPowerLabService_HappyPath(t *testing.T) {
	var capturedName string
	var capturedArgs []string
	stub := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil // systemctl restart returns empty on success
	}

	_, err := restartPowerLabServiceWith(stub, "powerlab-app-management")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if capturedName != "systemctl" {
		t.Errorf("expected systemctl, got %q", capturedName)
	}
	wantArgs := []string{"restart", "powerlab-app-management"}
	if !reflect.DeepEqual(capturedArgs, wantArgs) {
		t.Errorf("args: got %v, want %v", capturedArgs, wantArgs)
	}
}

func TestRestartPowerLabService_SurfacesSystemctlError(t *testing.T) {
	stub := func(name string, args ...string) ([]byte, error) {
		return []byte("Unit powerlab-gateway.service is masked.\n"), fmt.Errorf("exit status 1")
	}
	_, err := restartPowerLabServiceWith(stub, "powerlab-gateway")
	if err == nil {
		t.Fatal("expected error from masked-unit response, got nil")
	}
	if !strings.Contains(err.Error(), "masked") {
		t.Errorf("expected systemctl output in error, got %v", err)
	}
}

func TestQueryServiceState_ParsesSystemctlShowOutput(t *testing.T) {
	stub := func(name string, args ...string) ([]byte, error) {
		// systemctl show --value with multiple --property returns the
		// values in the same order as the properties, newline-separated.
		return []byte("active\nrunning\n12345\n"), nil
	}
	state, err := queryServiceStateWith(stub, "powerlab-gateway")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if state.Name != "powerlab-gateway" {
		t.Errorf("Name: got %q, want powerlab-gateway", state.Name)
	}
	if state.ActiveState != "active" {
		t.Errorf("ActiveState: got %q, want active", state.ActiveState)
	}
	if state.SubState != "running" {
		t.Errorf("SubState: got %q, want running", state.SubState)
	}
	if state.Pid != "12345" {
		t.Errorf("Pid: got %q, want 12345", state.Pid)
	}
}

func TestQueryServiceState_StoppedServiceHidesPid(t *testing.T) {
	// systemctl returns MainPID=0 for stopped services; ServiceState.Pid
	// should be empty so the UI doesn't render "PID: 0".
	stub := func(name string, args ...string) ([]byte, error) {
		return []byte("inactive\ndead\n0\n"), nil
	}
	state, err := queryServiceStateWith(stub, "powerlab-gateway")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if state.Pid != "" {
		t.Errorf("Pid: got %q, want empty for stopped service", state.Pid)
	}
}

func TestQueryServiceState_RejectsNonWhitelisted(t *testing.T) {
	var called bool
	stub := func(name string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	_, err := queryServiceStateWith(stub, "ssh")
	if err == nil {
		t.Error("expected error for non-whitelisted service")
	}
	if called {
		t.Error("commandRunner invoked for non-whitelisted service — SECURITY BUG")
	}
}

func TestQueryAllServiceStates_ReturnsOnePerService(t *testing.T) {
	// Stub returns success for everything; result should have one
	// entry per PowerLabServices.
	stub := func(name string, args ...string) ([]byte, error) {
		return []byte("active\nrunning\n100\n"), nil
	}
	states, errs := queryAllServiceStatesWith(stub)
	if len(states) != len(PowerLabServices) {
		t.Errorf("states count: got %d, want %d", len(states), len(PowerLabServices))
	}
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestQueryAllServiceStates_PartialFailureContinues(t *testing.T) {
	// One service errors → that one gets a placeholder, the rest still
	// query.
	stub := func(name string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[1] == "powerlab-core" {
			return []byte("Failed to get unit\n"), fmt.Errorf("exit status 1")
		}
		return []byte("active\nrunning\n100\n"), nil
	}
	states, errs := queryAllServiceStatesWith(stub)
	if len(states) != len(PowerLabServices) {
		t.Errorf("states count: got %d, want %d (placeholder for failed service must still appear)", len(states), len(PowerLabServices))
	}
	if len(errs) != 1 {
		t.Errorf("errs count: got %d, want 1", len(errs))
	}
	// The failed service appears with ActiveState="unknown".
	var coreState ServiceState
	for _, s := range states {
		if s.Name == "powerlab-core" {
			coreState = s
		}
	}
	if coreState.ActiveState != "unknown" {
		t.Errorf("failed service ActiveState: got %q, want unknown", coreState.ActiveState)
	}
}

// Layer 4: gateway self-restart must fork a transient systemd-run unit so
// the gateway's own cgroup is not torn down before the HTTP response fires.

func TestRestartGateway_UsesSystemdRun(t *testing.T) {
	var capturedExe string
	stub := func(name string, args ...string) ([]byte, error) {
		capturedExe = name
		return []byte(""), nil
	}
	_, err := restartPowerLabServiceWith(stub, "powerlab-gateway")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if capturedExe != "systemd-run" {
		t.Errorf("expected systemd-run for gateway self-restart, got %q — cgroup escape won't work", capturedExe)
	}
}

func TestRestartGateway_DelayedCommandContainsSleepAndServiceName(t *testing.T) {
	var capturedArgs []string
	stub := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}
	_, err := restartPowerLabServiceWith(stub, "powerlab-gateway")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// The shell command arg must encode both a delay and the correct unit.
	shellCmd := strings.Join(capturedArgs, " ")
	if !strings.Contains(shellCmd, "sleep") {
		t.Errorf("gateway restart command must include a sleep delay; got args: %v", capturedArgs)
	}
	if !strings.Contains(shellCmd, "powerlab-gateway") {
		t.Errorf("gateway restart command must reference the service name; got args: %v", capturedArgs)
	}
}

func TestRestartNonGateway_UsesSystemctlDirectly(t *testing.T) {
	// All non-gateway services must still use the synchronous systemctl path.
	for _, svc := range PowerLabServices {
		if svc == "powerlab-gateway" {
			continue
		}
		svc := svc
		t.Run(svc, func(t *testing.T) {
			var capturedExe string
			stub := func(name string, args ...string) ([]byte, error) {
				capturedExe = name
				return []byte(""), nil
			}
			if _, err := restartPowerLabServiceWith(stub, svc); err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if capturedExe != "systemctl" {
				t.Errorf("expected systemctl for %q, got %q", svc, capturedExe)
			}
		})
	}
}

// Layer 5: preflight endpoint — is-enabled per unit for the UI modal.

func TestQueryServiceEnabled_ReturnsTrueOnExitZero(t *testing.T) {
	stub := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil // systemctl is-enabled exits 0 → enabled
	}
	enabled, err := queryServiceEnabledWith(stub, "powerlab-gateway")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !enabled {
		t.Error("expected enabled=true for exit-0 stub")
	}
}

func TestQueryServiceEnabled_ReturnsFalseOnNonZeroExit(t *testing.T) {
	stub := func(name string, args ...string) ([]byte, error) {
		return []byte("disabled"), fmt.Errorf("exit status 1") // disabled unit
	}
	enabled, err := queryServiceEnabledWith(stub, "powerlab-gateway")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if enabled {
		t.Error("expected enabled=false for non-zero exit stub")
	}
}

func TestQueryServiceEnabled_RejectsNonWhitelisted(t *testing.T) {
	var called bool
	stub := func(name string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	_, err := queryServiceEnabledWith(stub, "sshd")
	if err == nil {
		t.Error("expected error for non-whitelisted service")
	}
	if called {
		t.Error("commandRunner invoked for non-whitelisted service — SECURITY BUG")
	}
}

func TestQueryServiceEnabled_InvokesIsEnabledQuiet(t *testing.T) {
	var capturedName string
	var capturedArgs []string
	stub := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil
	}
	if _, err := queryServiceEnabledWith(stub, "powerlab-core"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if capturedName != "systemctl" {
		t.Errorf("expected systemctl, got %q", capturedName)
	}
	if len(capturedArgs) < 2 || capturedArgs[0] != "is-enabled" {
		t.Errorf("expected first arg to be is-enabled, got %v", capturedArgs)
	}
	found := false
	for _, a := range capturedArgs {
		if a == "powerlab-core" {
			found = true
		}
	}
	if !found {
		t.Errorf("service name not in args: %v", capturedArgs)
	}
}

func TestQueryAllServiceEnabled_ReturnsOnePerService(t *testing.T) {
	stub := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}
	states := queryAllServiceEnabledWith(stub)
	if len(states) != len(PowerLabServices) {
		t.Errorf("got %d entries, want %d", len(states), len(PowerLabServices))
	}
	for _, s := range states {
		if !s.Enabled {
			t.Errorf("service %q should be enabled (stub returns exit 0)", s.Name)
		}
	}
}

func TestQueryAllServiceEnabled_PropagatesDisabledState(t *testing.T) {
	stub := func(name string, args ...string) ([]byte, error) {
		// Only powerlab-core is disabled.
		for _, a := range args {
			if a == "powerlab-core" {
				return []byte("disabled"), fmt.Errorf("exit status 1")
			}
		}
		return []byte(""), nil
	}
	states := queryAllServiceEnabledWith(stub)
	if len(states) != len(PowerLabServices) {
		t.Fatalf("got %d entries, want %d", len(states), len(PowerLabServices))
	}
	for _, s := range states {
		if s.Name == "powerlab-core" && s.Enabled {
			t.Error("powerlab-core should be disabled")
		}
		if s.Name != "powerlab-core" && !s.Enabled {
			t.Errorf("%q should be enabled", s.Name)
		}
	}
}

func TestRebootHost_InvokesSystemctlReboot(t *testing.T) {
	var capturedName string
	var capturedArgs []string
	stub := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil
	}
	_, err := rebootHostWith(stub)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if capturedName != "systemctl" || !reflect.DeepEqual(capturedArgs, []string{"reboot"}) {
		t.Errorf("expected `systemctl reboot`, got %q %v", capturedName, capturedArgs)
	}
}

func TestShutdownHost_InvokesSystemctlPoweroff(t *testing.T) {
	var capturedName string
	var capturedArgs []string
	stub := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil
	}
	_, err := shutdownHostWith(stub)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// poweroff, not shutdown — `systemctl shutdown` doesn't exist;
	// systemd uses `poweroff` for the explicit power-off action.
	if capturedName != "systemctl" || !reflect.DeepEqual(capturedArgs, []string{"poweroff"}) {
		t.Errorf("expected `systemctl poweroff`, got %q %v", capturedName, capturedArgs)
	}
}
