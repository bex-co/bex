# w6 · m160 — Runner-host link probe: a slow internet link opens an issue by itself

**Worker:** worker6 **Goal:** a degraded runner-host internet link is reported automatically within an hour, with measured throughput, and the report closes itself on recovery. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Probe script: download throughput and connect time to GitHub, Cloudflare and the pool's Docker Hub mirror](done/t001.md) — **DONE** | 50m | — |
| t002 | [Scheduled workflow: open/update `runner-link-slow` after two slow readings, close on recovery](done/t002.md) — **DONE** | 40m | t001 |
| t003 | [Self-test the probe with a stubbed curl](done/t003.md) — **DONE** | 30m | t001 |
| t004 | [Simplify](done/t004.md) — **DONE** | 15m | t002, t003 |
| t005 | [Test coverage](done/t005.md) — **DONE** | 20m | t004 |
| t006 | [Closeout](done/t006.md) — **DONE** | 10m | t005 |

## Definition of done

- A `workflow_dispatch` with `RUNNER_LINK_MIN_MBPS` above the measured link opens `runner-link-slow` naming the per-target MB/s; one at a normal threshold closes it.
- Scheduled runs on a normal link stay green and are within `ci-schedule-drift`'s window.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-06, proposal 2; the 2026-11-06 network finding recorded in `.pm/w6/blocked/m158/README.md`.
- **Goal linkage:** ADR008 reliable delivery and the user's automation principle ("清理手动的东西成自动的"): the stall was found only by hand.
- **Expected outcome:** a slow link surfaces as an issue with numbers instead of as hours-long deploys diagnosed manually; m158's release condition becomes an observable signal.
- **Why now:** m158 is blocked on exactly this condition, and nothing reports it today.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.
