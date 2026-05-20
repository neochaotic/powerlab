// Command powerlab-logs surfaces systemd-journal, Docker-container,
// and install/upgrade logs from a PowerLab host without depending on
// any running PowerLab daemon. The operator workflow:
//
//	powerlab-logs journal --service core --follow
//	powerlab-logs app blinko --tail 200 --follow
//	powerlab-logs install --list
//
// All three subcommands surface a unified phase-aware view of
// what's happening on the host. Designed to survive total
// PowerLab-daemon failure (see ADR-0027).
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

const usage = `powerlab-logs — surface PowerLab + Docker + install logs

Usage:
  powerlab-logs journal [--service NAME] [--follow] [--lines N] [--json] [--no-color]
  powerlab-logs app <name>   [--follow] [--tail N] [--timestamps]
  powerlab-logs install      [--list]
  powerlab-logs help

Subcommands:
  journal   Stream systemd journal entries for powerlab-* services.
            Without --service, matches all powerlab-*.service units.
  app       Stream Docker container logs for an installed app.
            <name> is the compose project name or container name.
  install   Surface install/upgrade logs from /var/log/powerlab/install-*.log.
            Default mode dumps the newest file; --list enumerates all.
  help      Print this message.

Examples:
  powerlab-logs journal --follow                  # tail every powerlab-* service
  powerlab-logs journal --service core --lines 100
  powerlab-logs app blinko --follow
  powerlab-logs install --list
  powerlab-logs install                           # dump newest install log

See docs/operations/powerlab-logs.md for the full reference.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	sub := os.Args[1]
	args := os.Args[2:]

	switch sub {
	case "journal":
		if err := cmdJournal(args, os.Stdout, isTTY(os.Stdout)); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "app":
		if err := cmdApp(args, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "install":
		if err := cmdInstall(args, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n%s", sub, usage)
		os.Exit(2)
	}
}

// cmdJournal parses flags then exec's journalctl with the built argv,
// piping stdout through runJournal.
func cmdJournal(args []string, out io.Writer, tty bool) error {
	var jArgs JournalArgs
	color := tty
	jsonOut := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--follow", "-f":
			jArgs.Follow = true
		case "--json":
			jsonOut = true
		case "--no-color":
			color = false
		case "--service", "-u":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			i++
			jArgs.Service = args[i]
		case "--lines", "-n":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			i++
			n := 0
			if _, err := fmt.Sscanf(args[i], "%d", &n); err != nil || n < 0 {
				return fmt.Errorf("invalid --lines value %q", args[i])
			}
			jArgs.Lines = n
		default:
			return fmt.Errorf("unknown flag %q for journal", a)
		}
	}

	cmd := exec.Command("journalctl", buildJournalArgs(jArgs)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("journalctl stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start journalctl: %w", err)
	}
	if err := runJournal(stdout, out, JournalOpts{Color: color, JSON: jsonOut}); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	return cmd.Wait()
}

// cmdApp exec's `docker logs <container>` with the appropriate flags
// and pipes its output directly to stdout. No parsing — Docker's
// native output already includes timestamps + level prefixes per
// the container's own logger.
func cmdApp(args []string, out io.Writer) error {
	var aArgs AppArgs

	if len(args) == 0 {
		return fmt.Errorf("app subcommand requires a container/app name")
	}
	// Positional arg first.
	aArgs.Container = args[0]
	rest := args[1:]

	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch a {
		case "--follow", "-f":
			aArgs.Follow = true
		case "--timestamps", "-t":
			aArgs.Timestamps = true
		case "--tail":
			if i+1 >= len(rest) {
				return fmt.Errorf("--tail requires a value")
			}
			i++
			n := 0
			if _, err := fmt.Sscanf(rest[i], "%d", &n); err != nil || n < 0 {
				return fmt.Errorf("invalid --tail value %q", rest[i])
			}
			aArgs.Tail = n
		default:
			return fmt.Errorf("unknown flag %q for app", a)
		}
	}

	aArgs.Container = resolveAppContainer(aArgs.Container)
	cmd := exec.Command("docker", buildAppArgs(aArgs)...)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// cmdInstall dumps /var/log/powerlab/install-*.log via runInstall.
func cmdInstall(args []string, out io.Writer) error {
	var opts InstallOpts
	for _, a := range args {
		switch a {
		case "--list", "-l":
			opts.List = true
		default:
			return fmt.Errorf("unknown flag %q for install", a)
		}
	}
	return runInstall("/var/log/powerlab", out, opts)
}

// isTTY reports whether w is a terminal — gates ANSI coloring.
// stdlib has no public IsTerminal in os; we use the
// golang.org/x/term-style stat approach without the dependency:
// check if the writer is *os.File backed by a character device.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
