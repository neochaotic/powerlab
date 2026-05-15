# Deadcode baselines

Per-service ceilings for the `check-deadcode.sh` delta-strict gate
(ADR-0037). Each file contains a single integer — the maximum allowed
`deadcode` finding count for that service. CI fails when the live
count exceeds the baseline.

## When to update

- **Going down** (count < baseline): a developer reduced dead code in
  a PR. The same PR must edit the relevant file to the new lower
  count. The script prints the exact `echo N > <file>` instruction.

- **Going up** (count > baseline): the PR introduced new dead code.
  CI fails. Fix the underlying issue (wire it up or delete it).
  Bumping the baseline is a last-resort escape hatch — reviewer
  sign-off in the PR.

## Missing baseline = treated as zero

If `<service>.txt` doesn't exist, any finding is treated as a
regression. New services must seed a baseline file (typically `0`) in
the same PR that introduces them.

## Local-storage exception

`local-storage` compiles only with Linux build tags (netlink, xattr,
fuse). The Mac dev machine skips it; CI on ubuntu-latest is where the
authoritative count is measured. Seed once from a green CI run.
