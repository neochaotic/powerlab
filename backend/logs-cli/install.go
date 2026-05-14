package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InstallOpts shapes the install-log subcommand. List mode enumerates
// available log files; default mode dumps the newest one.
type InstallOpts struct {
	List bool
}

// runInstall surfaces the contents of /var/log/powerlab/install-*.log.
// Designed to be unit-testable: dir parameter swappable for tempdir.
//
// Behaviour:
//   - default mode → write the newest install-*.log to w
//   - --list mode → enumerate available files with their mtimes
//   - dir does not exist → no error, no output (helpful for fresh
//     boxes pre-first-install)
//   - dir empty (no install-*.log files) → friendly "no install
//     logs found" message in default mode, silent in list mode
func runInstall(dir string, w io.Writer, opts InstallOpts) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if !opts.List {
				fmt.Fprintln(w, "No install logs found (directory does not exist yet).")
			}
			return nil
		}
		return fmt.Errorf("read install-log dir %q: %w", dir, err)
	}

	type file struct {
		name string
		full string
		info os.FileInfo
	}
	var files []file
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "install-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, file{name: name, full: filepath.Join(dir, name), info: info})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].info.ModTime().After(files[j].info.ModTime())
	})

	if opts.List {
		for _, f := range files {
			fmt.Fprintf(w, "%s  %s\n", f.info.ModTime().UTC().Format("2006-01-02T15:04:05Z"), f.name)
		}
		return nil
	}

	if len(files) == 0 {
		fmt.Fprintln(w, "No install logs found in", dir)
		return nil
	}
	data, err := os.ReadFile(files[0].full)
	if err != nil {
		return fmt.Errorf("read %s: %w", files[0].full, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
