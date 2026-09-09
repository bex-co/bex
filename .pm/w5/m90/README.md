# w5 · m90 — Correct CPU/memory percentages across replicas and rollouts

**Worker:** worker5 **Goal:** Normalize each replica with its own trustworthy limit before aggregation so usage charts remain correct through rollouts. **Status:** todo

**Estimate:** 3h30m implementation; 5h including standing closing tasks (8 tasks). **Priority:** 1 in the approved 2026-09-08 corrective queue.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Normalize each instance before replica aggregation | 60m | — |
| [t002](t002.md) | Associate retained usage with trustworthy instance limits | 60m | w5/m90/t001 |
| [t003](t003.md) | Expose consistent percentage semantics through API adapters and hooks | 45m | w5/m90/t002 |
| [t004](t004.md) | Render truthful limits and unavailable percentage states | 45m | w5/m90/t003 |
| [t005](t005.md) | Render parity | 25m | w5/m90/t004 |
| [t006](t006.md) | Simplify | 20m | w5/m90/t005 |
| [t007](t007.md) | Test coverage | 35m | w5/m90/t005, w5/m90/t006 |
| [t008](t008.md) | Closeout | 10m | w5/m90/t007 |

## Definition of done

- [ ] For replicas using 0.4/0.5 CPU and 0.5/1 CPU at the same timestamp, raw percentages are 80% and 50%; MIN is 50%, MAX is 80%, AVG is 65%. Equivalent memory cases pass.
- [ ] Normalization precedes replica aggregation; selection still precedes aggregation. Total mode preserves absolute samples and missing values are never filled with fabricated zeroes.
- [ ] Historical points never inherit a different instance’s limit or an unproven current limit. Missing, zero, or otherwise untrustworthy denominators omit the affected percentages with an explicit unavailable explanation; trustworthy retained history remains usable.
- [ ] REST, GraphQL, MCP, and dashboard agree on the supported percentage contract, defaults, units, and errors. Any bex extension or upstream evidence limit is documented.
- [ ] A disposable dev-5 mixed-limit fixture proves selection, rollout, and deleted-instance behavior. Desktop and narrow-mobile pending/ready captures show matching geometry and readable limit/unavailable states.
- [ ] Affected backend/dashboard checks pass; live evidence and any source limitations are recorded before closeout.

## Source + Goal linkage

- **Source:** User-approved pm-brainstorm proposal 1 for w5, materialized 2026-09-08. [Shipped m89](../done/m89/README.md) supplies the original contract.
- **Gap analysis:** m89 checked mixed-limit pairing as complete, but application-metrics-card.tsx still divides every point by latestValue of an aggregate limit. service.go resourceLimitMetric returns current pod limits only. This is source-review evidence from 2026-09-08, not a live reproduction. This milestone corrects that specific incomplete criterion; it does not reimplement m89 selection or m87 identity.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillars 1–3 and [.pm/GOAL.md](../../GOAL.md) basic operational observability: trustworthy, deterministic diagnostics for people and agents.
- **Expected outcome:** A user or agent can identify a saturated replica without misleading percentages or historical samples inheriting another replica’s limit.
- **Why now:** m89 exposes replica aggregation directly, making the pre-existing single-denominator shortcut more consequential.
- **Render parity:** Included because existing REST/GraphQL/MCP/UI metrics semantics change. Correct the specific percentage/aggregation or instance-filtering clauses in [ADR018](../../../docs/ADR018-render-parity.md)'s Core metrics row; preserve its independent decisions and evidence limits.

## Scope and execution

Historical limit availability is a verify-first implementation decision. Reuse available telemetry, preserve truthful values, and explicitly omit unsupported history; do not create a new telemetry warehouse or fabricate historical limits.

Approved order: m90 → m91. Both build on shipped m87/m89; neither reimplements those capability families. Existing m89 archived status is retained as historical shipping evidence; these milestones explicitly track the disproven acceptance claims. Follow the worker5 isolated environment instructions in [w5 README](../README.md). Do not close on code-only evidence when the definition of done requires a live walkthrough.

No external drains, new billing surface, first-token telemetry, unrelated API instrumentation, or chart-library rewrite. Preserve operator → types ← backend and thin dashboard/API adapters.
