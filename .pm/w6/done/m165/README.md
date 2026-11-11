# w6 · m165 — The bex-ci pool serves bex only; other repositories get their own runner pool

**Worker:** worker6 **Goal:** no job from another repository runs on, or waits for, a bex-ci runner. **Status:** done 2026-11-11 — 24 h placement verified: no other-repository job ran on bex-ci

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [block-eden-mono: an `other-ci` pool and its own group; bex-ci keeps no RUNNER_GROUP_REPOS](done/t001.md) — **DONE** | 50m | — |
| t002 | [GitHub groups and the other repositories' runs-on](done/t002.md) — **DONE** | 40m | t001 |
| t003 | [Roll the runners idle-only and verify placement](done/t003.md) — **DONE** | 30m | t002 |
| t004 | [Simplify](done/t004.md) — **DONE** | 15m | t003 |
| t005 | [Test coverage](done/t005.md) — **DONE** | 20m | t004 |
| t006 | [Closeout](done/t006.md) — **DONE** | 10m | t005 |

## Definition of done

- For 24 h after the switch no job from another repository runs on a bex-ci runner (job `runner_name` evidence).
- beancount-cms-v2 and eden-cms-v2 CI stays green on their own pool.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-07, proposal 3; 2026-11-06 network-bound CI evidence (w6/m158, m160).
- **Goal linkage:** ADR008 reliable delivery; DO_NOT_DO `#RUNNER-HOSTS` is respected (same host, separate pool).
- **Expected outcome:** other repositories' long downloads no longer hold bex's CI and deploy runners.
- **Why now:** on a degraded link foreign jobs held two of seven bex-ci runners for half an hour each.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.

## Gate released — 2026-11-09 07:57Z (was: t006 waits out the 24 h placement window)

- **Gate (time):** the DoD needs 24 h of job placement after the switch at 2026-11-08 07:57Z. **Release condition:** it is after 2026-11-09 07:57Z.
- **Worker next:** list every job in bex-co/beancount-cms-v2, eden-cms-v2 and block-eden-mono started after the switch and confirm none ran on a bex-ci runner (bex-ci runner names are the `github-runner-runner-bex-ci-*` container hostnames: `docker inspect -f '{{.Config.Hostname}}'`; other-ci's are `runner-other-ci-*`), and that both CMS repositories' CI is green on other-ci. Then run t006.
