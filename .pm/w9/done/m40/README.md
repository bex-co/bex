# w9 · m40 — Key Value `spec.name` backfill rollout: dev fleet + production

**Worker:** worker9 **Goal:** The idempotent `keyvalue-name-migrate.sh` backfill that w9/m6 live-proved in dev-9 actually runs everywhere it was deferred from — dev-1…dev-10 and production — so no Key Value store anywhere still carries name-as-id without `spec.name`, with before/after identity evidence and a production rename smoke via the official CLI. **Status:** **DONE** (2026-07-17) — dev-1…dev-10 + production backfill complete and idempotent (both fleets held zero pre-m6 stores; apply path proven live on a synthetic legacy store in dev-9, UID preserved); production official-CLI rename smoke green (`red-d9db3fp07a5s73dj32i0`, five identities preserved, self-cleaned). The rollout surfaced and fixed one script defect: `*-rename-verify.sh` silently swallowed transient `kubectl` list failures and mis-reported them as identity churn — hardened in both siblings + regression-tested (`scripts/rename-verify.test.sh`, CI-wired). Evidence: [`oplog.md`](oplog.md).

## Tasks (in order)

| id   | title                                                                                   | est  | depends_on | status |
| ---- | ---------------------------------------------------------------------------------------- | ---- | ---------- | ------ |
| t001 | Backfill dev-1…dev-10: run `keyvalue-name-migrate.sh` per environment, capture evidence | 1h   | —          | — **DONE** |
| t002 | Production backfill via the Postgres-precedent runbook + official-CLI rename smoke      | 1.5h | t001       | — **DONE** |
| t003 | Simplify                                                                                  | 15m  | t002       | — **DONE** |
| t004 | Test coverage                                                                             | 20m  | t002       | — **DONE** |
| t005 | Closeout                                                                                  | 15m  | t004       | — **DONE** |

## Definition of done

No Key Value store in any dev-N namespace or production lacks `spec.name`; `keyvalue-rename-cli-smoke.sh` passes against production with the official Render CLI; before/after identity evidence is recorded in the w9/m6 doc trail (`.pm/w9/done/m6/` op-log convention or `docs/` as appropriate).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 19, 2026-07-15 — closeout-residual sweep's only unowned finding: `w9/done/m6/README.md:36` ("run `keyvalue-name-migrate.sh` then `keyvalue-rename-cli-smoke.sh` against dev-1…dev-10 and production") and `w9/done/m6/done/t006.md:47` deferred the rollout as "operational rollout of the identical, now-live-proven mechanism" — then filed it nowhere.
- **Goal linkage:** completes shipped Render-parity work (ADR020 id discipline; the `red-` id + rename mechanism) — a parity feature isn't done while prod data predates it.
- **Expected outcome:** rename/identity behavior is uniform across every environment; no code path can encounter a store without `spec.name`.
- **Why now:** the longer old stores sit unmigrated, the more code gets written against mixed identity shapes; w9/m39's live probes just proved production/CLI access is workable, so the rollout is unblocked.
- **Render parity:** omitted — operational rollout of an already-shipped surface; no REST/GraphQL/MCP/UI change. Simplify/Test coverage are scoped to any script changes the rollout forces; if none, close them no-change-needed with the reason recorded.
