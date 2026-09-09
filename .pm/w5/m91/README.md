# w5 · m91 — Preserve instance selection through empty windows and refreshes

**Worker:** worker5 **Goal:** Keep explicit instance filters stable as discovery and telemetry change, with successful empty results for valid queries. **Status:** todo

**Estimate:** 2h implementation; 3h30m including standing closing tasks (7 tasks). **Priority:** 2 in the approved 2026-09-08 corrective queue.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Validate instance-filter eligibility independently of returned data | 40m | — |
| [t002](t002.md) | Preserve explicit selection across discovery and window changes | 45m | w5/m91/t001 |
| [t003](t003.md) | Make unavailable selections clear and usable on desktop and mobile | 35m | w5/m91/t002 |
| [t004](t004.md) | Render parity | 25m | w5/m91/t003 |
| [t005](t005.md) | Simplify | 20m | w5/m91/t004 |
| [t006](t006.md) | Test coverage | 35m | w5/m91/t004, w5/m91/t005 |
| [t007](t007.md) | Closeout | 10m | w5/m91/t006 |

## Definition of done

- [ ] Selecting replica A never returns to all instances or displays B without an explicit user action, including after polling, discovery errors, and window changes.
- [ ] Explicit selections survive temporary discovery gaps and historical choices leaving the window; the UI identifies unavailable choices and offers an explicit return-to-all action.
- [ ] An authorized instance-filtered CPU/memory query with no samples returns a successful empty result. Filterable limit queries receive consistent treatment; unsupported metric/filter combinations still return defined errors.
- [ ] Unknown or foreign instance selectors never broaden an authorized query; resource authorization, selector bounds, and malformed-input validation remain intact.
- [ ] Browser verification covers an expired historical choice, refresh failure/recovery, polling, resource navigation, and returning to all. Pending and ready states match at desktop and narrow-mobile widths.
- [ ] Backend and dashboard regression checks pass, API behavior agrees across surfaces, and live evidence is recorded before closeout.

## Source + Goal linkage

- **Source:** User-approved pm-brainstorm proposal 2 for w5, materialized 2026-09-08. [Shipped m89](../done/m89/README.md) supplies the original contract.
- **Gap analysis:** m89 checked no-silent-broadening as complete. Its prune effect removes unavailable selections, and an empty selection omits the INSTANCE filter, requesting all instances. select.go also infers filter eligibility from returned instance labels, so an empty supported CPU/memory query becomes ErrBadRequest. Source-confirmed on 2026-09-08; no live reproduction is claimed.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillars 1–3 and [.pm/GOAL.md](../../GOAL.md) basic operational observability: trustworthy, deterministic diagnostics for people and agents.
- **Expected outcome:** Selecting replica A never silently displays replica B, and a valid metric query with no matching samples is not misreported as a bad request.
- **Why now:** Polling, replica replacement, and window changes are normal transitions in the newly shipped m89 selector.
- **Render parity:** Included because existing REST/GraphQL/MCP/UI metrics semantics change. Correct the specific percentage/aggregation or instance-filtering clauses in [ADR018](../../../docs/ADR018-render-parity.md)'s Core metrics row; preserve its independent decisions and evidence limits.

## Scope and execution

The first task is technically independent of m90, but execute after m90 to avoid edits colliding in the same card. This is scheduling order, not an artificial cross-milestone dependency.

Approved order: m90 → m91. Both build on shipped m87/m89; neither reimplements those capability families. Existing m89 archived status is retained as historical shipping evidence; these milestones explicitly track the disproven acceptance claims. Follow the worker5 isolated environment instructions in [w5 README](../README.md). Do not close on code-only evidence when the definition of done requires a live walkthrough.

No external drains, new billing surface, first-token telemetry, unrelated API instrumentation, or chart-library rewrite. Preserve operator → types ← backend and thin dashboard/API adapters.
