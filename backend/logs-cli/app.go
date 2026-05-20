package main

import "strconv"

// AppArgs models the user-facing flag surface for the `app`
// subcommand. Translated into `docker logs` argv by buildAppArgs.
//
//	powerlab-logs app blinko --follow --tail 200
type AppArgs struct {
	Container  string // bare container or app name
	Follow     bool   // --follow → docker logs -f
	Tail       int    // --tail N → docker logs --tail N; 0 = no limit
	Timestamps bool   // --timestamps → docker logs -t
}

// buildAppArgs builds the argv slice we pass to `docker`. The actual
// exec is wired in the cobra layer; this function is pure so unit
// tests can lock the flag translation.
//   - First arg is always "logs"
//   - Flags follow: -f, --tail N, -t (timestamps)
//   - Container name is the last positional argument (Docker convention)
func buildAppArgs(a AppArgs) []string {
	out := []string{"logs"}
	if a.Follow {
		out = append(out, "-f")
	}
	if a.Tail > 0 {
		out = append(out, "--tail", strconv.Itoa(a.Tail))
	}
	if a.Timestamps {
		out = append(out, "-t")
	}
	out = append(out, a.Container)
	return out
}

// resolveAppContainer maps a user-facing app name to the Docker
// container name to query. MVP: pass-through (docker logs accepts
// the name + does its own matching).
//
// A follow-up sprint can extend this to resolve via the catalog
// + x-powerlab.main (e.g. "blinko" → "blinko-app-1") so the
// operator gets the right container even when an app has multiple
// services. For now the pass-through keeps the failure mode clear.
func resolveAppContainer(name string) string {
	return name
}
