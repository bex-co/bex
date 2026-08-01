# w7 · m69 — Scripted backup restore: executable recovery for every ADR031 store, verified end to end

**Worker:** worker7 **Goal:** recovery stops being a prose runbook — every backed-up store has a parameterized restore script with safety rails, and one full end-to-end exercise (fresh backup → scripted restore → data verification → teardown) proves each script works against real prod backups. **Status:** done

## Tasks (in order)

| id   | title                                                                      | est | depends_on             |
| ---- | -------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | `scripts/restore-etcd.sh` — scripted ADR011 Path A — **DONE**                       | 60m | —                      |
| t002 | `scripts/restore-openbao.sh` — scripted ADR015 throwaway restore — **DONE**         | 45m | —                      |
| t003 | `scripts/restore-postgres.sh` — generic CNPG recovery-cluster driver — **DONE**     | 60m | —                      |
| t004 | `scripts/restore-keyvalue.sh` — AOF-aware RDB seed restore — **DONE**                | 45m | —                      |
| t005 | End-to-end drill: backup → scripted restore → verify → teardown, all stores — DONE | 90m | t001, t002, t003, t004 |
| t006 | Amend ADR031: script contract + runbooks point at scripts — **DONE**                | 30m | t005                   |
| t007 | Simplify — **DONE**                                                                 | 20m | t006                   |
| t008 | Test coverage: shellcheck gate + DRY_RUN self-tests — **DONE**                      | 45m | t006                   |
| t009 | Closeout — **DONE**                                                                 | 15m | t008                   |

## Definition of done

`scripts/restore-etcd.sh`, `scripts/restore-openbao.sh`, `scripts/restore-postgres.sh`, and `scripts/restore-keyvalue.sh` exist, are parameterized (snapshot/target selection, PITR `targetTime` where applicable), support `DRY_RUN=1`, and share the safety rails: restore into a **throwaway** target only, never in-place, never touching the live store. One end-to-end drill per store ran against real prod backups using **only the scripts** (no ad-hoc kubectl surgery): a marker/known row was verified in each recovered target and every throwaway was torn down clean, recorded under `drills/`. ADR031's restore sections point at the scripts (prose kept as the explainer, scripts as the executable truth). CI shellchecks the four scripts and runs their `DRY_RUN` self-tests.

## Source + Goal linkage

- **Source:** user request 2026-07-31 (`/pm m69 …` — "prepare recover-from-backup scripts + ADR and actually try backup and recover end to end"), following the same day's persistent-store backup audit that materialized w7/m67–m68 and inbox notes `015`/`016`.
- **Goal linkage:** ADR031's consolidated data-protection policy — closes its "no automatic failover; all restores manual (runbook-driven)" consequence and de-risks the recovery half of every backup shipped in m29/m67/m68.
- **Expected outcome:** an operator under incident pressure runs one script per store instead of transcribing prose runbooks; restore correctness is proven end to end, not inferred from one-time drills.
- **Why now:** m67/m68 are adding two more restore paths — scripting all of them in one pass amortizes the shared plumbing (S3 fetch, throwaway lifecycle, verification, teardown), and the runbook knowledge is freshest right after the 2026-07-28 PITR drill. **Overlap note (gap analysis):** m67/t004 and m68/t005 drill their *backup mechanisms manually at ship time*; m69 is the disjoint next layer — *scripted* restores exercised end to end across **all** stores. If m67/m68 land first, their drills seed t003/t004's verification steps; if not, t003 covers bex-db + tenant Databases now and auth-dbs by parameter (same CNPG mechanism), and t004 implements against m68's recorded design.
- **Render parity omitted:** platform operations tooling (scripts + drills + ADR); no REST/GraphQL/MCP/UI surface change.
