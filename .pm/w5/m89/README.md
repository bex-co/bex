# w5 · m89 — Select and aggregate instances on the Metrics page

**Worker:** worker5 **Goal:** Let users and agents select public service instances and compare CPU/memory with MIN, MAX, or AVG across replicas at each timestamp. **Status:** todo

**Estimate:** 4h implementation; 6h 15m including standing closing tasks (9 tasks).

**Dependency:** w5/m87 must meet its definition of done first; t001 depends on w5/m87/t009 (Closeout). w5/m88 is independent and is scheduled ahead of this milestone by priority, not as a technical dependency.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Add bounded public-instance filtering to the shared metrics query | 60m | w5/m87/t009 |
| [t002](t002.md) | Implement MIN/MAX/AVG across selected replica samples | 60m | t001 |
| [t003](t003.md) | Expose consistent selection and aggregation across API adapters | 45m | t002 |
| [t004](t004.md) | Add Application Metrics instance and aggregation controls | 45m | t003 |
| [t005](t005.md) | Complete historical choices, localized states, and matching pending layout | 30m | t004 |
| [t006](t006.md) | Render parity | 30m | t005 |
| [t007](t007.md) | Simplify | 30m | t006 |
| [t008](t008.md) | Test coverage | 60m | t006, t007 |
| [t009](t009.md) | Closeout | 15m | t008 |

Task IDs in depends_on are relative to w5/m89 unless written as a full wN/mN/tNNN ID. Resolve completed dependencies through done/ locations; the ID remains stable when the file moves.

## Definition of done

- [ ] For a controlled timestamp with replica values 10 and 30, selecting the second yields 30; selecting both yields MIN 10, MAX 30, and AVG 20. Raw per-instance mode remains available.
- [ ] Instance selection precedes aggregation and uses m87's canonical public IDs. Missing samples remain absent and cannot become fabricated zeroes or carry forward from another timestamp.
- [ ] CPU/memory samples and their limits are paired under explicit Percentage/Total semantics, including a mixed-limit rollout; the UI does not independently reimplement backend aggregation.
- [ ] Live and historical instances present in the selected window can be chosen, including a pod that has since terminated. Empty, invalid, foreign, and oversized selections have defined safe behavior rather than silently broadening to all instances.
- [ ] REST, GraphQL, MCP, and the dashboard agree on selection, MIN/MAX/AVG behavior, defaults, and errors. Aggregation combines replicas at the same timestamp; CPU interval aggregation is a separate existing parameter.
- [ ] The Application Metrics controls are localized, accessible, and usable at desktop and narrow-mobile widths. Force pending and compare its geometry with ready state at both widths.
- [ ] A disposable dev-5 multi-replica walkthrough and meaningful backend/dashboard regressions prove the contract; affected checks and standing closing tasks pass.

## Source + Goal linkage

- **Source:** User-approved pm-brainstorm proposal 3 for w5, materialized 2026-09-08. Render's [service metrics documentation](https://render.com/docs/service-metrics#cpu-and-memory-usage), read 2026-09-08, describes selecting instances and minimum/maximum/average aggregation. Current ApplicationMetricsCard exposes Percentage/Total only; metricsQueryInputFromArgs omits INSTANCE and recognizes only MAX for aggregateAllMethod. [w5/m42](../done/m42/README.md) and [w3/m4.5](../../w3/done/m4.5/README.md) shipped the chart/control baseline, not this selection contract.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillars 1–3 and API-first hosting: the same diagnostic selection a human makes must be available to an agent through the shared metrics core.
- **Expected outcome:** A user can isolate a noisy live or historical replica and compare selected replicas with truthful aggregate values; REST, GraphQL, MCP, and UI express the same supported operation.
- **Why now:** w5/m87 establishes stable identities across retained telemetry and live instances. The current graphs already mix multiple live/historical series; selection makes those series actionable, and wiring it after the identity fix avoids a second incompatible selector convention.
- **Render parity:** Included: update the Core metrics (CPU · memory · instance-count · HTTP requests · latency · bandwidth) row in ADR018 with the newly confirmed CPU/memory selection and aggregation subgaps, then record their closure. The broad existing ✅ is not evidence these sub-capabilities already work. Reuse published Render names where available and document any bex REST/MCP extension instead of claiming an undocumented upstream contract.

## Scope and constraints

- Depends on w5/m87/t009, not on w5/m88. The user-approved execution priority remains m87, m88, m89.
- Target App CPU/memory selection and replica aggregation. Preserve ADR018's explicit service-level HTTP aggregation decision; this is not per-pod request instrumentation.
- The existing metrics INSTANCE discovery is live-pod-only. Extend bounded window-aware discovery or derive choices from the already-scoped returned series so historical selections remain possible.
- Validate unsupported or malformed options in the shared core/adapters instead of silently accepting them. Default all-instances behavior and existing MAX limit consumers need an explicit compatibility path.
- Use the existing feature hooks, schema codegen, translations, and route-skeleton conventions. No new frontend business-logic layer or independent polling loop.
- Do not add external metric drains or rewrite existing chart renderers as part of this work.

## Verification record

Pending. Materialization schedules implementation and verification; it is not a completion claim. Record commands, fixture identities, observable results, evidence paths, limitations, and cleanup here as work proceeds.
