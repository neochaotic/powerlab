<!-- Keep PRs to one logical change. See CONTRIBUTING.md. -->

## What & why

<!-- What does this change do, and why? -->

Closes #

## How it was tested

<!-- Commands you ran and the tests you added. A bug fix starts with a test
     that reproduces the bug (CONTRIBUTING.md "Test rule"). -->

## Checklist

- [ ] Tests added or updated; a bug fix includes a regression test
- [ ] `./scripts/validate.sh` passes locally (`--quick` for UI-only changes)
- [ ] Changelog fragment added under `.changes/unreleased/` (`changie new`) if this touches `backend/`, `ui/src/` or `scripts/`
- [ ] Docs updated if behaviour, flags, config or the API changed (CONTRIBUTING.md "Documentation rule")
- [ ] No secrets, tokens or personal paths committed (the Secret scan check enforces the first two)
- [ ] One logical change (no "and also...")

<!-- UI change? Add before/after screenshots.
     New catalog app? It belongs in neochaotic/powerlab-store, not here. -->
