package journal

import (
	"context"
	"os/exec"
)

// Exec is the production Runner: it invokes journalctl with the given
// args and returns its stdout. The command is the literal "journalctl"
// and args come only from BuildArgs (a fixed set of flags with
// validated/canonicalised values, passed as a literal argv — no shell),
// so the gosec G204 subprocess-with-variable-args finding is a false
// positive here.
func Exec(ctx context.Context, args []string) ([]byte, error) {
	// #nosec G204 -- command is the constant "journalctl"; args are built by BuildArgs, not user-assembled.
	return exec.CommandContext(ctx, "journalctl", args...).Output()
}
