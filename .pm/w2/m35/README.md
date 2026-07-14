# w2 · m35 — Postgres logical exports: `pg_dump` download parity

**Worker:** worker2 **Goal:** A tenant can take their data out of bex: exports become logical `pg_dump` artifacts with an authenticated download, closing the documented "physical, not portable" divergence. **Status:** todo

## Tasks (in order)

| id   | title                                                       | est | depends_on       |
| ---- | ----------------------------------------------------------- | --- | ---------------- |
| t001 | Verify Render's export endpoints + artifact shape (capture) | 30m | —                |
| t002 | Export Job: `pg_dump` → object store under the export id    | 60m | t001             |
| t003 | Export record lifecycle (created/running/available/failed)  | 40m | t002             |
| t004 | Authenticated download (`can_view_sensitive`)               | 45m | t003             |
| t005 | REST/GraphQL/MCP reshape to Render's exports vocabulary     | 40m | t004             |
| t006 | Dashboard: Recovery-section download button                 | 30m | t005             |
| t007 | Export-artifact retention                                   | 30m | t003             |
| t008 | Render parity                                               | 30m | t005, t006, t007 |
| t009 | Simplify                                                    | 30m | t008             |
| t010 | Test coverage                                               | 45m | t008             |
| t011 | Closeout                                                    | 15m | t010             |

## Definition of done

`createDatabaseExport` (all three surfaces) produces a logical `pg_dump` artifact in the object store; the export record progresses through an honest status lifecycle; an authorized caller downloads the dump and restores it into a vanilla Postgres outside bex; an unauthorized caller gets 403; artifacts expire per the recorded retention rule. The ADR018 Backups/PITR row's "physical, not logical" divergence is closed or narrowed with evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 2, 2026-07-14 (item 1); `docs/ADR018-render-parity.md` Backups·PITR row + `lego/backend/internal/postgres/recovery.go:219`'s own "documented divergence" comment.
- **Goal linkage:** GOAL #4 (PostgreSQL) + Render parity; data portability is the difference between "restorable inside bex" and "the user's data".
- **Expected outcome:** a tenant can export and walk away with a standard dump — the last real managed-Postgres capability divergence closes.
- **Why now:** w2's queue is down to two milestones; the export surface (w1/m17) and backup store plumbing (w2/m27) it composes are both settled. Render parity task included — all-surface change.
