# w1 · m166 — Truthful Key Value readiness

**Worker:** worker1 **Goal:** Available means the Key Value client path is serving, not merely that a plaintext socket accepts connections. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Measure the Key Value readiness gap | 35m | — |
| t002 | Probe authenticated serving readiness | 45m | t001 |
| t003 | Publish Ready only for the serving generation and route | 40m | t002 |
| t004 | Render parity | 25m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 35m | t004, t005 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- Maxmemory changes, persistence changes and resume have dated concurrent external TLS PING and API-status timelines; there is no available-but-unreachable window beyond one sampling interval.
- A loading/authentication/TLS failure cannot satisfy readiness. Use authenticated checks without logging secrets.
- New resources say creating; previously served stores say config_restart while unavailable; suspension and generation guards remain correct.
- REST, GraphQL, MCP and dashboard agree. Relevant operator/backend tests pass, with meaningful failing-before regressions.
- Root cause is measured, not assumed; local evidence and production evidence are distinguished explicitly.

## Source + Goal linkage

- **Source:** approved w1 brainstorm item 1, 2026-09-28; absorbs former w4/m137/t009, whose full evidence is [preserved here](source-w4-m137-t009.md). Completed work and the remaining production closeout remain in [w4/m137](../../w4/blocked/m137/README.md). No implementation is marked done by this transfer.
- **Goal linkage:** ADR008 pillar 2 agent-readable state; ADR021 managed Key Value and ADR018 Key Value row.
- **Expected outcome:** agents can safely proceed when a store reports available.
- **Why now:** live re-probe measured status leading successful client access by 10–23 seconds even after the status-label fixes shipped.
- **Render parity:** included; tenant-facing status crosses all four surfaces. User direction (2026-09-28): where behavior is uncertain, verify the Render contract and keep parity; do not introduce an undocumented divergence. Existing explicit non-goals remain out of scope.
- **Sizing:** 120m implementation; 210m total, seven tasks.
- **Environment:** use isolated dev-1; local harness recovery is pre-approved, but actual capacity failures must be reported honestly. No production rollout is authorized by filing.
