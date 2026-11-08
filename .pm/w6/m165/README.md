# w6 · m165 — The bex-ci pool serves bex only; other repositories get their own runner pool

**Worker:** worker6 **Goal:** no job from another repository runs on, or waits for, a bex-ci runner. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [block-eden-mono: an `other-ci` pool and its own group; bex-ci keeps no RUNNER_GROUP_REPOS](t001.md) | 50m | — |
| t002 | [GitHub groups and the other repositories' runs-on](t002.md) | 40m | t001 |
| t003 | [Roll the runners idle-only and verify placement](t003.md) | 30m | t002 |
| t004 | [Simplify](t004.md) | 15m | t003 |
| t005 | [Test coverage](t005.md) | 20m | t004 |
| t006 | [Closeout](t006.md) | 10m | t005 |

## Definition of done

- For 24 h after the switch no job from another repository runs on a bex-ci runner (job `runner_name` evidence).
- beancount-cms-v2 and eden-cms-v2 CI stays green on their own pool.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-07, proposal 3; 2026-11-06 network-bound CI evidence (w6/m158, m160).
- **Goal linkage:** ADR008 reliable delivery; DO_NOT_DO `#RUNNER-HOSTS` is respected (same host, separate pool).
- **Expected outcome:** other repositories' long downloads no longer hold bex's CI and deploy runners.
- **Why now:** on a degraded link foreign jobs held two of seven bex-ci runners for half an hour each.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.
