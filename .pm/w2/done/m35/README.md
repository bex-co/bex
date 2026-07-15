# w2 · m35 — Postgres logical exports: `pg_dump` download parity

**Worker:** worker2 **Goal:** A tenant can take their data out of bex: exports become logical `pg_dump` artifacts with an authenticated download, closing the documented "physical, not portable" divergence. **Status:** **DONE 2026-07-14**

## Tasks (in order)

| id   | title                                                       | est | depends_on       | status     |
| ---- | ----------------------------------------------------------- | --- | ---------------- | ---------- |
| t001 | Verify Render's export endpoints + artifact shape (capture) | 30m | —                | — **DONE** |
| t002 | Export Job: `pg_dump` → object store under the export id    | 60m | t001             | — **DONE** |
| t003 | Export record lifecycle (created/running/available/failed)  | 40m | t002             | — **DONE** |
| t004 | Authenticated download (`can_view_sensitive`)               | 45m | t003             | — **DONE** |
| t005 | REST/GraphQL/MCP reshape to Render's exports vocabulary     | 40m | t004             | — **DONE** |
| t006 | Dashboard: Recovery-section download button                 | 30m | t005             | — **DONE** |
| t007 | Export-artifact retention                                   | 30m | t003             | — **DONE** |
| t008 | Render parity                                               | 30m | t005, t006, t007 | — **DONE** |
| t009 | Simplify                                                    | 30m | t008             | — **DONE** |
| t010 | Test coverage                                               | 45m | t008             | — **DONE** |
| t011 | Closeout                                                    | 15m | t010             | — **DONE** |

## Definition of done

`createDatabaseExport` (all three surfaces) produces a logical `pg_dump` artifact in the object store; the export record progresses through an honest status lifecycle; an authorized caller downloads the dump and restores it into a vanilla Postgres outside bex; an unauthorized caller gets 403; artifacts expire per the recorded retention rule. The ADR018 Backups/PITR row's "physical, not logical" divergence is closed or narrowed with evidence.

## Outcome and DoD evidence (2026-07-14)

- Captured Render's singular `GET/POST /v1/postgres/{id}/export`, `202` create response, optional URL, `.dir.tar.gz` directory format, one-active-export rule, and seven-day retention in `docs/render-artifacts/postgres-exports.md`.
- bex-api now appends typed `exp-…` intent to `Database.spec.exports`; the operator owns resource-limited, sequential dump/upload Jobs and records `created` → `running` → `available|failed`, followed by `expiring` → `expired` only after object deletion succeeds. Exact expiry requeueing and TTL-based Job cleanup avoid event and crash-consistency gaps.
- REST, GraphQL, and MCP create tests each prove they write the shared intent; list parity proves identical export fields. Listing and URL minting require `can_view_sensitive`; the denial test returns REST `403`. URLs are minted per read for at most 15 minutes and never stored.
- `bash scripts/postgres-export-verify.sh` passed against a private MinIO object store: the operator's exact PostgreSQL 16 dump/tar and pinned AWS CLI upload commands produced an artifact; unsigned retrieval returned `403`; retrieval through an expiring signed URL succeeded; restoration into a separate vanilla PostgreSQL 16 container returned the seeded `1:portable` row.
- The dashboard Recovery section renders lifecycle badges, failure reasons, active-export disabled states, and a real download action. Its focused tests pass.
- Verification: operator `make test` passed; backend `go test ./...` passed; dashboard `yarn test --run` passed (168 files, 1,019 tests), plus `yarn typecheck` and `yarn lint`. Changed operator/backend packages have zero golangci-lint findings. Repository-wide `make lint` remains blocked only by 15 pre-existing `cmd/pg-sni-proxy` findings and one unrelated `internal/build/build_test.go` parameter finding.
- Simplification was performed as a manual behavior-preserving review because no `/simplify` capability is installed in this Codex environment; redundant secret-env construction and continuous idle dashboard polling were removed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 2, 2026-07-14 (item 1); `docs/ADR018-render-parity.md` Backups·PITR row + `lego/backend/internal/postgres/recovery.go:219`'s own "documented divergence" comment.
- **Goal linkage:** GOAL #4 (PostgreSQL) + Render parity; data portability is the difference between "restorable inside bex" and "the user's data".
- **Expected outcome:** a tenant can export and walk away with a standard dump — the last real managed-Postgres capability divergence closes.
- **Why now:** w2's queue is down to two milestones; the export surface (w1/m17) and backup store plumbing (w2/m27) it composes are both settled. Render parity task included — all-surface change.
