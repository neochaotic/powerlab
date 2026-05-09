# Release Checklist

This is the playbook for cutting a PowerLab release. It is the
**only** authoritative source for "what we verify before pushing a
tag." If something matters and is not on this list, add it here in
the same PR that verifies it.

The list is split into two phases. **Phase 1 (verification)** runs
in a clean working tree against `main`; nothing is mutated.
**Phase 2 (release)** mutates the repo (changelog, tag, push) and
should only run after Phase 1 is clean.

> Every release goes through Phase 1 in full, even patch releases.
> The five extra minutes are cheap insurance against a regression
> shipping under our name.

---

## Phase 1 — Verification

### 1.1 Clean working tree on `main`

- [ ] `git checkout main && git pull --ff-only`
- [ ] `git status` reports nothing modified, nothing untracked.
      (Untracked files in dev workdirs that shouldn't ship are
      a smell — investigate before continuing.)

### 1.2 CI is green on the merge commit being released

- [ ] `gh run list --branch main --limit 1` shows the latest run as
      `completed: success`.
- [ ] All required checks listed under branch protection
      have run and succeeded — one missed `success` from the
      required set means the tag will not auto-build.

### 1.3 Local validation suite passes

- [ ] `./scripts/validate.sh --quick` (~30s — frontend + native go test)
- [ ] `./scripts/validate.sh` (~3min — same checks CI runs)
- [ ] `./scripts/validate.sh --full` (~7min — adds package smoke
      + Docker CGO; **mandatory** for tag, optional for branch).

### 1.4 Foundation contract sanity (manual)

The bug-#64 SIGSEGV class is structurally closed in code, but a
runtime smoke confirms the wiring still composes correctly after
any kill PR series.

- [ ] On a Linux host: `./powerlab-bin gateway` (or the equivalent
      service-startup command), then hit a route that intentionally
      panics (`/v1/__panic__` if exposed, otherwise contrive one).
      Expected: `HTTP 500` with `{code, i18n_key, correlation_id}`
      body, log line `"panic recovered in handler"` carrying the
      same `correlation_id`. Process keeps running.
- [ ] Verify `X-Request-Id` round-trip: send a request with a
      custom `X-Request-Id: abc-123`. Response carries the same
      header back. Server logs for that request include
      `correlation_id=abc-123`.

### 1.5 Schema migrations sanity

- [ ] On a fresh DB: install the candidate, confirm services boot
      and `~/.local/share/powerlab/*.db` is created.
- [ ] On an upgrade DB: install the previous release, create
      a user + at least one app, then upgrade in place to the
      candidate. Confirm: user can still log in, apps still
      listed, no data loss.

(Note: until `pkg/migrations` lands per #100, this step is the only
guard against AutoMigrate silently dropping data. Do NOT skip.)

### 1.6 Documentation freshness

- [ ] `CHANGELOG.md` has an entry for the release version (Phase
      2 generates this — confirm the changie fragments under
      `.changes/unreleased/` are complete and accurate).
- [ ] If user-visible features changed: `README.md` install
      command and feature list still match.
- [ ] If platform support changed: `SUPPORT.md` reflects the
      new matrix.
- [ ] ADRs for any architectural decisions made during the
      release cycle exist under `docs/decisions/`.
- [ ] `release-manifest.yaml` summary updated for THIS release.
      Run `./scripts/check-manifest-fresh.sh` — exit 0 means
      the summary differs from the previously published release.
      Exit 1 means **stop and edit** (the check was added after
      the v0.5.4 mishap where the YAML still carried v0.5.0's
      summary, see issue #156).

### 1.7 Issue / PR housekeeping

- [ ] All milestone-tagged PRs for this release are merged.
- [ ] Open issues with the release-blocker label are zero.
- [ ] `gh issue list --label sprint-N` for the just-finished
      sprint matches what shipped.

---

## Phase 2 — Release

> Run these in order. Each step assumes Phase 1 is clean. If anything
> fails, stop and investigate; do **not** force through.

### 2.1 Generate the changelog from fragments

```bash
# changie binary must be installed: go install github.com/miniscruff/changie@latest
changie batch <VERSION>            # e.g. v0.5.0
changie merge                       # writes the new section into CHANGELOG.md
git diff CHANGELOG.md               # eyeball the new section
```

- [ ] CHANGELOG diff looks correct — version header right, every
      fragment present, no duplicates.
- [ ] `.changes/unreleased/` is now empty (changie moves consumed
      fragments to `.changes/<version>/`).

### 2.2 Commit the generated CHANGELOG

```bash
git add CHANGELOG.md .changes/
git commit -m "chore(release): v<VERSION>"
```

### 2.3 Tag and push

```bash
git tag -a v<VERSION> -m "PowerLab v<VERSION>"
git push origin main
git push origin v<VERSION>
```

- [ ] Tag pushed.
- [ ] CI starts the release workflow (visible at
      `gh run list --workflow ci.yml`).

### 2.4 Verify the GitHub Release

- [ ] `gh release view v<VERSION>` shows: amd64 tarball, arm64
      tarball, `manifest.json` attached.
- [ ] Tarball download + extract + `./install.sh --dry-run`
      works end-to-end on a clean Linux box (or a fresh container).
- [ ] In-app updater (if a previous release is installed) detects
      the new version: `curl -fsSL <release-url>/manifest.json`
      returns the new SHA-256s.

### 2.5 Post-release

- [ ] Close the milestone for the version released.
- [ ] Open the milestone for the next version.
- [ ] Move any deferred-from-this-release issues into the next
      sprint label.
- [ ] Update README badges if they reference a specific version.

---

## Special gates for `v1.0.0`

The v1.0 tag is a **contract**: backwards compatibility within the
major version starts here. Beyond Phase 1 + Phase 2 above, v1.0
requires:

- [ ] **Explicit user approval** before tagging. v1.0 is not
      automatic even if "all sprint work is done" — the project
      lead signs off in writing.
- [ ] Zero open `bug` label issues at moderate-or-higher severity.
- [ ] Apache/Airflow-level documentation site live at
      `docs.powerlab.io` (mkdocs-material per Phase 3 of the
      docs commitment).
- [ ] `pkg/migrations` shipped and exercised on a real upgrade
      path (Phase 1.5 verifies the version is at the expected
      schema migration after upgrade).
- [ ] Security-sensitive paths reviewed: TLS handshake on
      gateway, JWT signing on user-service, certificate-trust
      onboarding flow, OAuth proxy strategy decided per #101.
- [ ] An end-to-end load + chaos test run (kill a service
      mid-request, verify recovery + correlation_id observability).
- [ ] Cross-platform CI green (Ubuntu LTS oldstable + current,
      arm64 hardware sanity check on a Pi).
