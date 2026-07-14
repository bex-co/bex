# w8 · m11 — Reliable usage windows: durable zeroes + gap-free per-meter cursors

**Worker:** worker8 **Goal:** Every App usage meter records a trustworthy, contiguous hourly series: a successful zero is durable evidence, a failed source read advances no cursor, and the next collector pass retries the failed meter before newer windows — so `w8/001` can base Hobby caps on measured data rather than silent undercounts. **Status:** done

## Tasks (in order)

| id   | title                                                                                         | est | depends_on | status     |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Define the App-meter success/failure contract and contiguous-cursor invariant                 | 25m | —          | — **DONE** |
| t002 | Persist successful zero windows and keep failed source reads absent                           | 40m | t001       | — **DONE** |
| t003 | Catch up each App meter independently and stop at its first failed window                      | 45m | t002       | — **DONE** |
| t004 | Document the coverage query and the point when `w8/001`'s 28-day evidence clock starts        | 25m | t003       | — **DONE** |
| t005 | Simplify — run `/simplify` over the usage collector/cursor code changed by this milestone      | 20m | t004       | — **DONE** |
| t006 | Test coverage — successful zeroes, source failures, independent retries, and idempotent reruns | 35m | t004       | — **DONE** |
| t007 | Closeout — DoD met → move milestone to `done/`                                                | 10m | t006       | — **DONE** |

## Definition of done

For each App and each of `instance_seconds`, `egress_bytes`, and `build_seconds`, a successful collection writes one `usage_hourly` row even when its quantity is zero; a failed Prometheus/Kubernetes read writes no row; collection retries that meter's oldest missing window (within the existing 48-hour catch-up bound) before advancing it, without holding healthy meters behind an unrelated failure. Unit tests prove zero/failure/retry/idempotency behavior, the backend suite passes, and `docs/ADR023-usage-metering.md` plus `w8/001` contain a query whose gap-free rows can start the 28-day cap-evidence clock.

## Closeout evidence

- `usage.Service` now keeps one oldest-first cursor per App meter, persists successful zeroes, stops only the failed meter on source/store errors, retries on the next hourly pass, and clamps initial catch-up to `App.CreatedAt` within the existing 48-hour bound.
- Raw zero egress/build rows remain coverage-only: `UsageMonthToDate` omits all-zero non-instance groups. A real-Postgres acceptance test proves three raw anchors remain stored while the summary preserves its prior single zero-instance row.
- Regression tests cover successful zeroes, unavailable Prometheus/Kubernetes sources, transient store failure, healthy-meter independence, retry order, idempotent keys, and the no-pre-creation invariant.
- `docs/ADR023-usage-metering.md` documents the contract; `.pm/w8/001.md` now audits expected versus recorded hourly windows from the corrected collector's live deployment time.
- Verification passed `go test ./...`, `go test -race ./internal/usage`, focused store/usage tests, and the two real-Postgres usage acceptance tests. Markdown formatting and `git diff --check` pass.

## Source + Goal linkage

- **Source:** prerequisite split from `.pm/w8/001.md`'s 2026-07-14 gate audit. The cap-enforcement feature itself remains in the inbox until its data and outbound-egress-scope gates are met.
- **Goal linkage:** `GOAL.md` item 5 (usage metering) and pillar 1 (one reliable core behind every usage surface); hard limits cannot safely depend on counters that silently turn source failures into free usage.
- **Expected outcome:** transient Prometheus or Kubernetes failures create retryable gaps instead of permanent undercounts, while real zero-use hours become explicit coverage evidence.
- **Why now:** `001` requires roughly a month of real rows, but the current sparse-write/single-cursor contract cannot prove that a month was collected. Fixing it later would invalidate the waiting period and restart the evidence clock.
- **Render parity closing task omitted:** this milestone repairs the internal metering pipeline only; it changes no REST, GraphQL, MCP, or dashboard contract. Render comparison remains in `001` for the later tenant-facing enforcement milestone.
