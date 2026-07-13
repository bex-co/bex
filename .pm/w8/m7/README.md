# w8 · m7 — Price sheet + estimated spend (Render-equivalent billing)

**Worker:** worker8 **Goal:** Every usage surface (REST/GraphQL/MCP/dashboard) shows a real, documented dollar estimate — 30% below Render's captured prices on every compute/Postgres/KeyValue/build-minute line, 90% below on bandwidth — computed from the existing metered quantities, with no payment collection anywhere in the path. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                                                                                        | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Live-capture Render's current pricing across every metered surface into a render-artifact snapshot (`docs/render-artifacts/pricing.md`): compute instance types, managed Postgres tiers, managed Key Value tiers, bandwidth, build-minute overage, workspace-plan included allowances | 40m | —          |
| t002 | Design bex's price sheet: a new data file outside `lego/types/tiers/` (that file's own invariant: operator-imported, must never depend on money) mapping each compute/Postgres/KeyValue tier id → bex $/month (30% off) and a bandwidth $/GB rate (90% off) and build-minute overage rate (30% off) | 45m | t001       |
| t003 | `docs/ADR030-pricing.md`: document the price sheet, the 30%-off-everything / 90%-off-bandwidth policy, and the "estimate only, no payment collection" boundary (cross-reference `FUTURE-MAYBE.md`'s fired trigger and w6's still-standing "no billing system" boundary) | 30m | t002       |
| t004 | Extend `GET /v1/usage` (REST) with an `estimatedCost` field (per-meter $ + total), computed from the existing `instance_seconds`/`egress_bytes`/`build_seconds` rows × the price sheet, clearly labeled as an estimate                        | 40m | t002       |
| t005 | GraphQL `usage` query: mirror the same `estimatedCost` shape                                                                                                                                                                                 | 20m | t004       |
| t006 | MCP `get_usage` tool: mirror the same `estimatedCost` shape                                                                                                                                                                                  | 20m | t004       |
| t007 | Dashboard Usage page: show the estimated-cost figure alongside the existing quantity charts, with a tooltip/note that it's an estimate, not an invoice                                                                                      | 45m | t005, t006 |
| t008 | Live verification: confirm a workspace with real metered usage shows a correctly-computed estimate matching the price sheet by hand-calculation                                                                                             | 30m | t007       |
| t009 | Render parity: check the new `estimatedCost` field is consistent (same shape/rounding/currency) across REST/GraphQL/MCP/UI — bex-only superset, no Render row to diff against directly                                                     | 20m | t008       |
| t010 | Simplify: run `/simplify` over the code this milestone changed                                                                                                                                                                                | 30m | t009       |
| t011 | Test coverage: meaningful tests for price-sheet lookup + estimate computation (including an unknown/unpriced tier, zero usage, and rounding)                                                                                                | 40m | t009       |
| t012 | Closeout: verify DoD, mark done, move to `w8/done/m7/`                                                                                                                                                                                       | 15m | t010, t011 |

## Definition of done

A workspace's `GET /v1/usage` (and the GraphQL/MCP/dashboard equivalents) returns an `estimatedCost` alongside the existing metered quantities, computed from a documented, versioned price sheet that is 30% below Render's captured prices on every compute/Postgres/KeyValue/build-minute line and 90% below on bandwidth — with no payment collection anywhere in the path.

## Source + Goal linkage

- **Source:** user request 2026-07-13, firing the trigger `.pm/FUTURE-MAYBE.md` recorded for "Pricing & spend estimation" (deferred 2026-07-09).
- **Goal linkage:** `GOAL.md` #5 (usage metering — the other half w8 already meters quantities for); makes bex's cost advantage over Render concrete and checkable rather than an unquantified claim.
- **Expected outcome:** every usage surface (REST/GraphQL/MCP/dashboard) shows a real, documented dollar estimate; `docs/ADR030-pricing.md` is the durable source of truth for the discount policy.
- **Why now:** the metering pipeline (`w8/m1`-`m4`) and its REST/GraphQL/MCP/UI surfaces (`w8/m2`-`m3`) are already shipped — this is additive enrichment, not new plumbing; explicit user trigger fires the parked item.
- **Scope boundary (user-confirmed 2026-07-13):** estimate only, no payment processor/invoicing/dunning — that remains a separate, larger, not-yet-triggered reopening of w6's "no billing system" boundary (the `tiers.yaml` comment's "prices are Metronome's" note is left as a forward pointer for that future work, not built now).
