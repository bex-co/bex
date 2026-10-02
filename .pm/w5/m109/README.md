# w5 · m109 — Enforce the twelve-hour cron runtime limit

**Worker:** worker5 **Goal:** Bound scheduled and manual cron execution consistently with Render's documented twelve-hour limit. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render runtime limit and Kubernetes timing | 20m | — |
| t002 | Enforce one deadline policy for scheduled and manual cron runs | 30m | t001 |
| t003 | Verify deadline termination and subsequent scheduling | 40m | t002 |
| t004 | Render parity and surface disposition | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 25m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- Dated Render research establishes the twelve-hour limit and records known/unknown timer origin, termination grace and status behavior for scheduled and manual runs.
- Both scheduled and manual execution paths enforce the selected limit. Queueing, image pull, container start and Job suspension timing are explicitly compared with Kubernetes semantics; any unverified difference is documented.
- A shortened-deadline local drill proves termination, terminal API/dashboard history, release of scheduling and a later run. The record explicitly says it is an accelerated drill, not a real twelve-hour execution.
- Cancellation and existing single-run/preemption behavior continue to work; no Render retry guarantee is inferred or retry policy changed.
- Operator and affected backend tests plus lint pass; REST/GraphQL/MCP/UI outcomes are consistent and disposable fixtures are removed.

## Source + Goal linkage

- **Source:** User-approved w5 brainstorm and Render parity review, 2026-10-01; [w5/m106 research](../done/m106/README.md) records the public limit while current scheduled/manual Job construction sets backoffLimit without an execution deadline.
- **Goal linkage:** ADR008 predictable hosting resource usage and ADR038/ADR018 cron parity.
- **Expected outcome:** Scheduled and manually triggered cron runs terminate at the researched execution limit, expose a truthful terminal result, and allow subsequent scheduled work.
- **Why now:** Both cron Job construction paths lack a duration bound; add the documented limit after establishing timer origin, grace and status behavior rather than assuming a Kubernetes field exactly reproduces Render.
- **Render parity included:** This changes user-visible deploy or cron behavior. Check REST, GraphQL, MCP and dashboard semantics against the t001 research, distinguishing direct Render requirements from bex correctness and undocumented details.

## Execution notes

Start with t001 before implementation. Work only in isolated dev-5 for live checks. Queue order is m107 → m108 → m109; m108 and m109 share cron code, so land m108 before changing it for m109. Research and all implementation tasks remain todo at filing.
