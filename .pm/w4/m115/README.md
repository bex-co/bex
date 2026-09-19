# w4 · m115 — Database Insights is blind: Active processes + Top queries render 34 `<insufficient privilege>` cells

**Worker:** worker4 **Goal:** a database owner opening Insights sees their processes and top queries — or an honest explanation — never 34 raw Postgres mask strings. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est  | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Give the insights path visibility into tenant query activity (grant, not masking)              | 1h30 | —          |
| t002 | Dashboard never renders raw `<insufficient privilege>` rows                                    | 45m  | t001       |
| t003 | Render parity + docs                                                                           | 20m  | t002       |
| t004 | Simplify                                                                                       | 20m  | t003       |
| t005 | Test coverage                                                                                  | 45m  | t004       |
| t006 | Closeout                                                                                       | 10m  | t005       |

## Definition of done

Each bullet is a click the next person can repeat on production and watch succeed.

- **Processes are visible.** On a throwaway database with recent SQL-console activity, Insights → Active processes shows real query text for the owner's own sessions (today: every Query cell reads `<insufficient privilege>`).
- **Top queries are visible.** Top queries shows real query text with calls/timing (today: all 25 rows masked).
- **No raw mask strings.** No `<insufficient privilege>` cell ever renders; if any row is legitimately hidden, the section says why (today: 34 masked cells, zero explanation).
- **Least privilege preserved.** The fix grants introspection visibility only — the insights path stays read-only (`default_transaction_read_only` + statement timeout intact).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass on `https://dashboard.bex.co`, 2026-09-19 (w4-targeted run, `muse.env` credentials). Fixture: Postgres `qa-20260919-p14-db` (`dpg-dan32k92dbts73fi28o0`, deleted after the pass) with a live table + SQL-console round trip. Insights section: Active processes 9 rows × (User blank, Query `<insufficient privilege>`), Top queries 25 rows × Query `<insufficient privilege>` — 34 masked cells, unchanged after Refresh. Storage + Table scans + Parameters on the same page render fine.
- **Goal linkage:** managed-Postgres introspection ([docs/ADR009-postgresql-management.md](../../../docs/ADR009-postgresql-management.md)) — Insights is the flagship "live introspection" surface and two of its six panels are unusable.
- **Expected outcome:** the insights connection can read the activity it is shown to display, and the dashboard degrades honestly when it cannot.
- **Why now:** the mechanism is nailed in code (`runInsight` dials the tenant DB with the unprivileged app-user URI, so Postgres itself masks every other role's query text) — the fix is a provisioning grant plus a UI guard, not an investigation.
- **Explicitly out:** this pass's re-confirmed clean areas (provisioning, TLS, SQL-console CRUD + guards, Metrics charts, Recovery empty state, CIDR validation, disk-autoscaling persistence) — not re-filed.
