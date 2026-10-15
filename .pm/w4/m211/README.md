# w4 · m211 — Keep batched hosting changes inside one response budget

**Worker:** worker4 **Goal:** a hosting move sent as a GraphQL batch returns readable per-operation outcomes before the HTTP response deadline; a batch that has exhausted its budget starts no later operation. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Share the execution budget across the GraphQL batch | 40m | — |
| t002 | Keep completed, uncertain and unstarted operations distinct | 40m | t001 |
| t003 | Audit batch clients and preserve passing controls | 30m | t002 |
| t004 | Render parity | 15m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 40m | t004, t005 |
| t007 | Closeout | 25m | t006 |

## Definition of done

Repeat the [recorded native journey](finding.md) with a disposable owned project, Alpha/Beta environments, a Free image web service and its linked synthetic-value group:

- Select the service and group in Alpha and Move to Beta. Repeat the reverse from a fresh 390px page. The native two-operation POST returns a readable GraphQL result array within the shared execution budget, with an outcome for each input in order. A successful operation keeps its actual success result. An interrupted dispatched write reports `WRITE_OUTCOME_UNKNOWN`; a write never dispatched reports `OPERATION_NOT_STARTED`. Neither recorded action ends as an edge 502 and generic failure of both operations.
- After an uncertain result, wait for the existing placement reads to settle before another write is offered. The ready source/target lists and selections agree with fresh membership reads. Preserve links, the group value, the original Live deploy and runtime output in this placement-only journey.
- Repeat the recorded single and two-operation read controls. Operations completing within the shared budget retain their normal responses. The old 26.5s successful move may instead report a truthful late-operation timeout under the proposed 25s shared budget; preserving its entire running time is not a requirement.
- Delete every owned resource, verify the five exact API lookups and public URL return 404, restore all seven inventory ID sets, and revoke only this QA session.

Local deadline, skipped-resolver and late-completion scenarios are additional verification work in t001/t002/t006; they were not injected into production.

## Source + Goal linkage

- **Source:** continuous functional `qa-find-bugs`, a569/a570, 2026-10-15 UTC, user-directed w4 filing. [Finding](finding.md), [durable requests, timing and cleanup](evidence.json).
- **Goal linkage:** ADR008 usable hosting operations and [ADR006](../../../docs/ADR006-bex-api.md) reliable API outcomes; [ADR032](../../../docs/ADR032-environments.md) existing environment placement.
- **Expected outcome:** a normal bulk move gets an acknowledgment or a specific bounded uncertainty result. The user can inspect actual placement before retrying.
- **Why now:** a fresh native batch returned 502 after approximately 31s and two failure toasts, although both memberships subsequently committed; the passing direction returned both successes in approximately 26.5s.
- **Sizing:** 205m across request budgeting, dispatch/outcome semantics, shared-client audit and required closing work. Filing only; no implementation included.
- **Relationship:** this is the batch-envelope gap beyond w4/done/m140's single-document timeout and w4/blocked/m206's individual group-operation recovery. Their existing guarantees remain required; see the explicit disposition in the finding.
- **Render parity included:** this changes GraphQL outcomes consumed by the dashboard. REST/MCP placement semantics stay intact; compare surfaces and document that no authenticated Render timing replay was performed.
