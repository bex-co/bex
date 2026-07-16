# w4/m29 normalization evidence — 2026-07-15

## t001 — mechanism

`scripts/ipallowlist-normalize.sh`, mirroring the RC5 `spec.name` backfill precedent
(`scripts/postgres-name-migrate.sh` / `keyvalue-name-migrate.sh`): idempotent,
non-destructive, dry-run by default, `--apply` to write; strings → `{cidr}`,
object entries untouched (descriptions preserved), non-string/non-object entries
abort before any write. Clusterless regression harness
`scripts/ipallowlist-normalize.test.sh` (16 assertions: plan/apply/idempotence/
description preservation/malformed preflight) — all green.

## t002 — fleet state

- **Production** (Hetzner, all namespaces, dry-run then implicit verify): both
  resource kinds reported `ipAllowList normalization already complete` — zero
  string-shaped entries exist. No `--apply` was needed; the fleet was already
  clean (every prod allow-list was written post-m24 through bex-api, which has
  always emitted the object shape).
- **Local (CAPD mock cluster)**: not running at verification time
  (`infra/local/bex.kubeconfig` endpoint stale). Nothing to normalize — the
  local cluster is ephemeral and recreates CRs through the current write path;
  the script is available for any future bring-up carrying legacy state.

Consequence for sequencing: the fleet is verifiably clean, so t003's schema
tightening may land in any LATER deploy (never the same one as this script's
introduction, per the milestone's sequencing constraint — trivially satisfied
since nothing needed rewriting).
