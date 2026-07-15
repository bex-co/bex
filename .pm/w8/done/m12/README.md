# w8 · m12 — Managed Postgres major-version upgrade

**Worker:** worker8 **Goal:** `Database.spec.version` stops being create-time-only: a tenant upgrades a managed Postgres to a newer major version through a guarded verb on every surface, with the operator driving CNPG's declarative major-upgrade path. **Status:** done

## Tasks (in order)

| id   | title                                                        | est | depends_on | status     |
| ---- | ------------------------------------------------------------ | --- | ---------- | ---------- |
| t001 | Verify Render's upgrade UX/constraints (capture)             | 30m | —          | — **DONE** |
| t002 | Confirm deployed CNPG supports declarative major upgrades    | 30m | —          | — **DONE** |
| t003 | Operator: version bump → CNPG major upgrade, status surfaced | 60m | t002       | — **DONE** |
| t004 | Backend: version-update verb + validation + backup guard     | 45m | t003       | — **DONE** |
| t005 | Dashboard: version row + upgrade dialog                      | 40m | t004       | — **DONE** |
| t006 | Render parity                                                | 30m | t005       | — **DONE** |
| t007 | Simplify                                                     | 30m | t006       | — **DONE** |
| t008 | Test coverage                                                | 45m | t006       | — **DONE** |
| t009 | Closeout                                                     | 15m | t008       | — **DONE** |

## Definition of done

On the mock cluster, a seeded Postgres at version N upgrades to N+1 via the public verb (REST/GraphQL/MCP) and via the dashboard dialog, with data intact afterward; a downgrade or unknown version is rejected with a named 400; the upgrade is refused when no backup exists for a durability-plan instance; the Database status reflects the upgrade's progress honestly (never silently "ready" mid-upgrade).

## Closeout evidence

- Render's dashboard-only flow, target/downtime constraints, and bex's deliberate REST/GraphQL/MCP extensions are captured in `docs/render-artifacts/postgres-version-upgrade.md`; ADR009 and ADR018 carry the resulting contract and parity evidence.
- The CAPD mock cluster proved raw CNPG 16→17, public REST 16→17, GraphQL 16→17, MCP 17→18, and a real headless-dashboard 17→18 flow. Every seeded row survived; the browser rendered the downtime warning and observed Available → Upgrading → Available.
- Named validation was exercised live: unknown/equal targets returned HTTP 400 (`POSTGRES_VERSION_UNKNOWN` / `POSTGRES_VERSION_NOT_NEWER`), and a durable-plan upgrade without a current-generation completed backup returned HTTP 409 (`POSTGRES_UPGRADE_BACKUP_REQUIRED`).
- The operator now watches CNPG status-only resource-version changes, exposes observed/current versions and honest upgrade phases, rolls a failed desired version back, rotates backup archive generations, and starts a new base backup after success.
- The simplification pass extracted backup-name derivation and the CNPG readiness/upgrade state machine, reducing the main reconciler's cyclomatic complexity from 48 to 29. Feature tests cover validation, backup generations, adapters, image/status transitions, failure rollback, dialog errors, and post-mutation polling.
- Verification passed `make test`, backend `go test ./... && go build ./...`, and dashboard `yarn lint && yarn test && yarn build` (180 files / 1,066 tests). Repository-wide `make lint` has no finding in milestone code; it remains nonzero on 16 pre-existing `pg-sni-proxy`/build-test findings outside this milestone.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 2, 2026-07-14 (item 2); `lego/types/v1alpha1/database_types.go:35` (`Version` exists at create; no update verb anywhere); Render's documented Postgres version-upgrade flow.
- **Goal linkage:** GOAL #4 (PostgreSQL) + Render parity — version upgrades are how a managed database stays supported; without one, every bex Postgres is pinned forever.
- **Expected outcome:** the version field becomes a lifecycle, not a birth certificate; the parity ledger's Postgres section gains the upgrade capability.
- **Why now:** w8's queue is down to m10; CNPG's declarative major-upgrade support makes this a verb + reconcile change rather than a data-migration project (t002 verifies that premise first — if the deployed CNPG lacks it, the milestone re-scopes to the CNPG upgrade first and says so). Placed under w8 for capacity per the m8 precedent (topical owner w1 has three open milestones). Render parity task included — all-surface change.
