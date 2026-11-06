# w6 · m159 — Ship gates catch dashboard red before it reaches main

**Worker:** worker6 **Goal:** a push that changes `dashboard/` fails `scripts/ship-gates.sh` locally when it would turn main's dashboard tests, typecheck, lint or knip red. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Measure local typecheck, lint, knip and related-test run times and pick the gate's scope](t001.md) | 30m | — |
| t002 | [Run the dashboard checks in ship-gates for pushes that touch dashboard/](t002.md) | 60m | t001 |
| t003 | [Self-test the dashboard gate with a stubbed yarn](t003.md) | 40m | t002 |
| t004 | [Document the dashboard gate in /ship and AGENTS.md](t004.md) | 20m | t002 |
| t005 | [Simplify](t005.md) | 15m | t003, t004 |
| t006 | [Test coverage](t006.md) | 20m | t005 |
| t007 | [Closeout](t007.md) | 10m | t006 |

## Definition of done

- Replaying a push on top of `2333662d3` that leaves `route-heads.test.ts` stale makes `bash scripts/ship-gates.sh origin/main` exit non-zero; so does an unused dashboard file (knip).
- A dashboard-only push's gate stays within t001's budget; a push without dashboard changes is unaffected.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-06, proposal 1. Four dashboard reds in two days (runs above), each holding the deploy queue; fixed by `eab7fda78`.
- **Goal linkage:** ADR008 reliable delivery: finished work reaches production promptly; a red main stalls every deploy behind it.
- **Expected outcome:** dashboard defects are caught on the pushing machine instead of after a 1–3 h deploy run.
- **Why now:** the dashboard caused four of the last two days' red mains, and the gate already exists for the backend.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.
