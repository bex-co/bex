# w2 · m93 — Migration clean-window evidence

**Worker:** worker2 **Goal:** make the registry/static identity migration's 14-day zero-legacy-read requirement reproducible before destructive cleanup. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Identify legacy-read evidence and coverage — **DONE** | 40m | — |
| t002 | Retain migration-scoped read evidence — **DONE** | 50m | t001 |
| t003 | Add a read-only migration readiness report — **DONE** | 50m | t002 |
| t004 | Document the evidence window and Phase 4 handoff — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

An operational read-only report returns `clean`, `legacy reads detected`, or `insufficient evidence`, with exact interval, source coverage and migrated-resource scope. `clean` requires 14 continuous days of zero legacy reads plus current tombstoned/scoped resource references. Missing history, collection gaps, source errors and restarts cannot silently count as zero. Controlled scratch-resource reads prove detection; evidence is retained for the full interval plus evaluation time. The initial live report and reproducible commands are recorded, and w2/035 carries the earliest defensible cleanup date. This milestone closes when collection/reporting work, even if the initial result is insufficient evidence; it does not wait 14 days or execute Phase 4.

## Source + Goal linkage

- **Source:** user-approved revalidation of `/pm-brainstorm for w2`, 2026-09-08, against `88b4f9893`; `.pm/w2/done/m92/README.md`, its phase-3 evidence, `docs/runbooks/registry-static-identity-migration.md` Phase 4, and `.pm/w2/035.md`.
- **Revalidated gap:** `scripts/verify-workspace-scoped-identity.sh` checks current inventory/ACLs, not historical reads. The runbook requires zero legacy reads for 14 days but does not identify a retained query or coverage check. Checked-in Loki retention is 168h. No dedicated readiness report was found. This is a repository evidence gap, not a claimed live telemetry outage; t001 verifies actual sources before adding collection.
- **Goal linkage:** ADR008 reliable self-hosted hosting and tenant isolation through ADR074 workspace-scoped artifact identities; safe completion of ADR055 F2/F3.
- **Expected outcome:** an operator can distinguish a proven clean interval from missing evidence before deleting old artifacts.
- **Why now:** m92 started the window on September 8; unavailable evidence can invalidate the planned September 22 cleanup date. Preserve evidence while it is still available.
- **Sizing:** 170m implementation plus 60m standing tasks. If t001 proves existing complete evidence makes the remainder sub-hour, resize via /pm instead of creating unnecessary telemetry infrastructure.
- **Render parity omitted:** platform-internal collection and read-only operational tooling; no REST/GraphQL/MCP/UI contract change.
- **Scope and authorization:** preparation only. w2/035 remains the sole Phase 4 owner, unpromoted until its clean-window and explicit change-window authorization gates hold. This board handoff does not execute cleanup or grant Phase 4 authorization.
