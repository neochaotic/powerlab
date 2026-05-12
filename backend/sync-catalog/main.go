// Command sync-catalog imports the public Umbrel App Store catalog
// into PowerLab's `community-catalog/`, applying the four-tier
// filter pipeline from ADR-0024.
//
// Usage:
//
//	sync-catalog \
//	  --source umbrel \
//	  --output community-catalog \
//	  --upstream https://github.com/getumbrel/umbrel-apps.git \
//	  --allow-categories Bitcoin,Lightning   (optional opt-in)
//
// Exit codes:
//
//	0  — sync completed (apps may have been filter-rejected; not a fail)
//	1  — fatal error (clone failed, output dir not writable, etc.)
//	2  — invalid args
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var (
		source       = flag.String("source", "umbrel", "catalog source identifier (currently only 'umbrel')")
		output       = flag.String("output", "community-catalog", "output directory")
		upstreamRepo = flag.String("upstream", "https://github.com/getumbrel/umbrel-apps.git", "upstream git URL")
		allowedCats  = flag.String("allow-categories", "", "comma-separated soft-reject categories to opt back in (e.g. 'Bitcoin,Lightning')")
		workdir      = flag.String("workdir", "", "use this pre-cloned dir instead of cloning (skips git clone)")
		dryRun       = flag.Bool("dry-run", false, "scan + filter + report; do not write any files")
		validateOnly = flag.String("validate-only", "", "skip sync; validate an existing catalog tree at this path + exit (0 = clean, 1 = errors)")
	)
	flag.Parse()

	// --validate-only short-circuits sync entirely. Used by CI to gate
	// merges of weekly sync PRs (#307 Phase 6) and by maintainers
	// editing description-powerlab.md / icon overrides locally.
	if *validateOnly != "" {
		errs, err := ValidateCatalogTree(*validateOnly)
		if err != nil {
			log.Fatalf("validate %q: %v", *validateOnly, err)
		}
		for _, e := range errs {
			fmt.Println(e.String())
		}
		if len(errs) > 0 {
			fmt.Fprintf(os.Stderr, "\n%d validation error(s)\n", len(errs))
			os.Exit(1)
		}
		fmt.Println("catalog OK")
		return
	}

	if *source != "umbrel" {
		fmt.Fprintf(os.Stderr, "unsupported source %q (only 'umbrel' is wired today)\n", *source)
		os.Exit(2)
	}

	ctx := context.Background()

	upstreamDir := *workdir
	upstreamCommit := ""
	if upstreamDir == "" {
		tmp, err := os.MkdirTemp("", "sync-catalog-*")
		if err != nil {
			log.Fatalf("mktemp: %v", err)
		}
		defer os.RemoveAll(tmp)

		log.Printf("[sync-catalog] cloning %s → %s", *upstreamRepo, tmp)
		if out, err := exec.Command("git", "clone", "--depth=1", *upstreamRepo, tmp).CombinedOutput(); err != nil {
			log.Fatalf("git clone: %v\n%s", err, string(out))
		}
		upstreamDir = tmp
	}

	// Resolve the commit SHA for the source block — even with --depth=1
	// `git rev-parse HEAD` returns the single-commit SHA.
	if shaBytes, err := exec.Command("git", "-C", upstreamDir, "rev-parse", "HEAD").Output(); err == nil {
		upstreamCommit = strings.TrimSpace(string(shaBytes))
	}
	if upstreamCommit == "" {
		log.Printf("[sync-catalog] warning: could not resolve upstream commit SHA — provenance will omit it")
	} else {
		log.Printf("[sync-catalog] upstream HEAD = %s", upstreamCommit)
	}

	// Build the known-app-IDs set from the upstream directory listing —
	// the filter uses this to distinguish "same-compose sibling" (allow)
	// from "cross-app sibling" (Tier 1 reject).
	entries, err := os.ReadDir(upstreamDir)
	if err != nil {
		log.Fatalf("read upstream dir: %v", err)
	}
	known := make(map[string]bool, len(entries))
	apps := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // .github, etc.
		}
		known[name] = true
		apps = append(apps, name)
	}
	sort.Strings(apps)
	log.Printf("[sync-catalog] %d candidate apps in upstream", len(apps))

	allowedCatList := splitCSV(*allowedCats)
	filter := &Filter{
		KnownAppIDs:       known,
		AllowedCategories: allowedCatList,
	}

	resolver := NewDescriptionResolver(*output)

	var counts struct {
		allowed, hardReject, softReject, manualTriage, parseError int
	}

	for _, appID := range apps {
		manifestPath := filepath.Join(upstreamDir, appID, "umbrel-app.yml")
		composePath := filepath.Join(upstreamDir, appID, "docker-compose.yml")

		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			log.Printf("[skip] %-40s — no umbrel-app.yml (%v)", appID, err)
			counts.parseError++
			continue
		}
		composeData, err := os.ReadFile(composePath)
		if err != nil {
			log.Printf("[skip] %-40s — no docker-compose.yml (%v)", appID, err)
			counts.parseError++
			continue
		}

		manifest, err := ParseUmbrelManifest(manifestData)
		if err != nil {
			log.Printf("[skip] %-40s — manifest parse: %v", appID, err)
			counts.parseError++
			continue
		}
		compose, err := ParseComposeFile(composeData)
		if err != nil {
			log.Printf("[skip] %-40s — compose parse: %v", appID, err)
			counts.parseError++
			continue
		}

		verdict := filter.Apply(appID, manifest, compose)
		switch verdict.Tier {
		case TierHardReject:
			log.Printf("[hard]  %-40s — %s", appID, verdict.Reason)
			counts.hardReject++
			continue
		case TierSoftReject:
			log.Printf("[soft]  %-40s — %s", appID, verdict.Reason)
			counts.softReject++
			continue
		case TierManualTriage:
			log.Printf("[triag] %-40s — %s", appID, verdict.Reason)
			counts.manualTriage++
			// TODO Phase 6: write into community-catalog/_pending/
			continue
		}

		if *dryRun {
			log.Printf("[ok ✓]  %-40s — would emit", appID)
			counts.allowed++
			continue
		}

		desc, descErr := resolver.Resolve(ctx, manifest)
		if descErr != nil {
			// Description fetch failures don't block emit; we just leave
			// the description empty and rely on the maintainer override
			// path to fill it in.
			log.Printf("[warn]  %-40s — description: %v", appID, descErr)
			desc = ""
		}

		path, err := Emit(EmitContext{
			OutputRoot:       *output,
			UpstreamRepo:     *upstreamRepo,
			UpstreamCommit:   upstreamCommit,
			TransformVersion: CurrentTransformVersion,
		}, manifest, composeData, desc)
		if err != nil {
			log.Printf("[err]   %-40s — emit: %v", appID, err)
			counts.parseError++
			continue
		}

		log.Printf("[ok]    %-40s — %s", appID, path)
		counts.allowed++
	}

	fmt.Println()
	fmt.Println("=== sync-catalog summary ===")
	fmt.Printf("candidates       %d\n", len(apps))
	fmt.Printf("allowed (Tier 4) %d\n", counts.allowed)
	fmt.Printf("hard-reject T1   %d\n", counts.hardReject)
	fmt.Printf("soft-reject T2   %d\n", counts.softReject)
	fmt.Printf("manual triage T3 %d\n", counts.manualTriage)
	fmt.Printf("parse errors     %d\n", counts.parseError)
	if *dryRun {
		fmt.Println("(dry-run — no files written)")
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
