# w4 · m20 — IAM & service-payload parity chores

**Worker:** worker4 **Goal:** Close w4's own filed residuals — read-verb denials reach the audit log, the service object exposes `slug`/`subdomain`, and the parity ledger's identity rows match reality. Groups inbox `001` + `011` (each sub-hour) per the sizing rule (the w1/m23 / w6/m15 / w7/m10 pattern). **Status:** done 2026-07-14 — DoD verified: `TestAuditRecordsDeniedReadRelations`/`TestAuditCoversEveryWriteVerbExactlyOnce` (denied reads record, allowed reads don't), `TestSlugPresentOnAllThreeSurfaces` + dashboard component/unit tests (`slug` correct incl. the suffixed case, REST/GraphQL/MCP/UI), ADR018 MFA + email-recovery rows refreshed to match the board. Backend `go test ./...` + `make lint-backend` clean; dashboard `yarn typecheck && yarn lint && yarn test` clean (166 files / 1017 tests). One efficiency finding from the `/simplify` pass filed as a follow-up, not fixed inline: `w4/014.md`.

## Tasks (in order)

| id   | title                                                    | est | depends_on       |
| ---- | --------------------------------------------------------- | --- | ---------------- |
| t001 | Audit log records read-verb (view) denials — **DONE**     | 45m | —                |
| t002 | Expose `slug`/`subdomain` on the service object — **DONE** | 45m | —                |
| t003 | Refresh stale ADR018 identity rows — **DONE**              | 20m | —                |
| t004 | Render parity — **DONE**                                   | 30m | t001, t002, t003 |
| t005 | Simplify — **DONE**                                         | 30m | t004             |
| t006 | Test coverage — **DONE**                                    | 45m | t004             |
| t007 | Closeout — **DONE**                                          | 15m | t006             |

## Definition of done

A denied `can_view`/`can_view_logs`/`can_view_sensitive` check produces an `audit_events` row with `status: denied`; `slug` is readable on the REST/GraphQL/MCP service object (and the dashboard header can show it); the ADR018 MFA and email-recovery rows no longer contradict the board.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 6); groups `w4/001`-filed-as-duplicate → renumbered `w4/012` on grouping (read-denial audit gap, filed 2026-07-13) + `w4/011` (slug field, filed from w4/m19's parity check 2026-07-14).
- **Goal linkage:** w4's IAM mandate (audit completeness) + Render parity (Render's service object carries `slug`).
- **Expected outcome:** security-relevant read denials stop being invisible; clients can read the bare slug instead of parsing it out of URLs.
- **Why now:** w4 is otherwise done after m11's last two tasks; these are its own filed residuals, cheapest to close while the code is warm. Render parity task included — REST/GraphQL/MCP payload change.
