# w8 · m4 — Usage data retention: compact hourly detail into monthly aggregates

**Worker:** worker8 **Goal:** Bound `usage_hourly`'s growth by compacting rows older than a retention window into a `usage_monthly` aggregate table and purging the compacted hourly detail — transparently, so `period=` queries across the compaction boundary keep returning correct totals. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design + document the retention policy (hot-window months kept hourly; older compacted) in `docs/usage-metering.md` | 30m | —          |
| t002 | `usage_monthly` table migration + monthly-aggregation SQL                                                        | 35m | t001       |
| t003 | Compaction routine: aggregate hourly rows older than the hot window into `usage_monthly`, then purge them — idempotent, safe to re-run | 45m | t002       |
| t004 | Wire compaction into a periodic loop (daily tick, reusing `usage.Service`'s cadence pattern) with an env knob for the hot-window length, mirrored in `.env.example`/`.env.template` | 35m | t003       |
| t005 | `monthToDateAt`/period query reads `usage_monthly` for months outside the hot window, `usage_hourly` inside it — same `Summary` shape, transparent to REST/GraphQL/MCP | 40m | t002       |
| t006 | Acceptance: seed old hourly rows → run compaction → `usage_monthly` populated, hourly detail purged, `GET /v1/usage?period=<old-month>` still returns correct totals | 30m | t004, t005 |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                      | 20m | t006       |
| t008 | Test coverage — meaningful tests for the compaction routine + boundary-crossing period queries                   | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                                   | 10m | t008       |

## Definition of done

After the hot window passes, hourly detail for old months is compacted into monthly rows and the `usage_hourly` rows are purged; querying an old period via REST/GraphQL/MCP still returns the same totals as before compaction; the compaction routine is idempotent (safe to re-run on the same window) and its cadence + window are env-tunable and documented.

## Source + Goal linkage

- **Source:** `/pm-brainstorm think of new milestones for w8` 2026-07-09 — gap analysis of `lego/backend/internal/store/migrations/0004_usage.up.sql` (no TTL/partitioning/cleanup on `usage_hourly`) and `usage.Service.Run` (`service.go:159`, insert-only, never prunes).
- **Goal linkage:** `GOAL.md` item 5 (usage metering); platform-hardening theme shared with `w3/m6` (silent operational rot in unwatched control-plane state — here, unbounded table growth instead of unwatched backups).
- **Expected outcome:** `usage_hourly` stops growing without bound; the control-plane store's size stays predictable as tenant/service count and time both grow.
- **Why now:** m1/m2 shipped 2026-07-09 — the table is still small. Every month of delay is more uncompacted backlog to migrate later.
- **Render parity task omitted:** Render has no comparable usage/billing API surface to compare against (bex's own extension, per `docs/render-parity.md` § bex ahead of Render); this is a pure storage/operations concern with no REST/GraphQL/MCP/UI surface-consistency question to check.
