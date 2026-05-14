package main

import (
	"testing"
)

// app subcommand tests. The handler shells out to `docker logs` —
// keeping the binary lean (no Docker SDK dep). buildAppArgs builds
// the argv slice; the exec itself is integration-tested at the
// build-the-system level, not here.

func TestBuildAppArgs_BareContainerName(t *testing.T) {
	args := buildAppArgs(AppArgs{Container: "blinko-app-1"})
	if !contains(args, "logs") || !contains(args, "blinko-app-1") {
		t.Errorf("expected `logs blinko-app-1` in argv, got: %v", args)
	}
}

func TestBuildAppArgs_FollowFlag(t *testing.T) {
	args := buildAppArgs(AppArgs{Container: "x", Follow: true})
	if !contains(args, "-f") {
		t.Errorf("--follow should add -f, got: %v", args)
	}
}

func TestBuildAppArgs_TailLimit(t *testing.T) {
	args := buildAppArgs(AppArgs{Container: "x", Tail: 200})
	if !contains(args, "--tail") || !contains(args, "200") {
		t.Errorf("--tail N should add --tail N pair, got: %v", args)
	}
}

func TestBuildAppArgs_TailZero_OmitsFlag(t *testing.T) {
	// Tail=0 means "no limit" / "all lines" → don't pass --tail.
	args := buildAppArgs(AppArgs{Container: "x"})
	if contains(args, "--tail") {
		t.Errorf("Tail=0 should NOT include --tail, got: %v", args)
	}
}

func TestBuildAppArgs_TimestampsFlag(t *testing.T) {
	args := buildAppArgs(AppArgs{Container: "x", Timestamps: true})
	if !contains(args, "-t") && !contains(args, "--timestamps") {
		t.Errorf("--timestamps should add -t flag, got: %v", args)
	}
}

func TestBuildAppArgs_ContainerLast(t *testing.T) {
	// Docker accepts flags BEFORE the container name; container must
	// be the last positional argument.
	args := buildAppArgs(AppArgs{Container: "test-app", Follow: true, Tail: 50})
	if args[len(args)-1] != "test-app" {
		t.Errorf("container name should be the last arg, got: %v", args)
	}
}

func TestResolveAppContainer_FallbackPattern(t *testing.T) {
	// Operator types `powerlab-logs app blinko` — we want it to
	// resolve to the app's main container, which by Docker compose
	// convention is `blinko-<service>-1`. Without docker-cli round-
	// trip (which is exec-side), this helper just builds the
	// fallback pattern + lets `docker logs` fail clearly if the
	// real container name differs.
	got := resolveAppContainer("blinko")
	// The exact resolution is "use the app name as-is and let
	// docker do its own matching"; what we lock is that the function
	// is pure + idempotent + non-empty.
	if got == "" {
		t.Errorf("resolveAppContainer should never return empty, got %q", got)
	}
	if got != "blinko" {
		t.Errorf("MVP: resolveAppContainer should pass through, got %q", got)
	}
}
