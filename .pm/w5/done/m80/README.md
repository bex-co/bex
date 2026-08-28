# w5 · m80 — Agent-session failure reliability: bounded, fast, clearly-attributed terminal failures

**Worker:** worker5 **Goal:** a misconfigured, hung, or capacity-blocked agent session fails in seconds with a clear, actionable reason and releases its sandbox promptly — instead of the current ~3-minute (worst-case 4-hour) hang that holds plan quota behind a blank failure card. **Status:** done

## Tasks (in order)

| id   | title                                                                                                | est | depends_on             | status     |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------------------- | ---------- |
| t001 | Reproduce + characterize failure latency and attribution; set targets and freeze the fix boundary    | 45m | —                      | — **DONE** |
| t002 | Sane default turn timeout wired through bex-api and the driver                                        | 45m | t001                   | — **DONE** |
| t003 | Model-proxy terminal-auth fast-fail: terminalize on non-transient upstream 401/403                   | 75m | t001                   | — **DONE** |
| t004 | Prompt reclaim of terminally-failed sandboxes with no open editor SSH                                 | 60m | t001                   | — **DONE** |
| t005 | `failureReason` completeness across every terminal-failure class (absorbs `w5/048`)                  | 45m | t003                   | — **DONE** |
| t006 | Render parity                                                                                         | 30m | t002, t003, t004, t005 | — **DONE** |
| t007 | Simplify                                                                                              | 25m | t006                   | — **DONE** |
| t008 | Test coverage                                                                                         | 60m | t006                   | — **DONE** |
| t009 | Closeout                                                                                              | 10m | t007, t008             | — **DONE** |

**Live-verified on dev-5 (2026-08-27):** a session with an invalid workspace model key reached `failed` at **t+17s** (~5s in `running`) vs the ~192s measured before, carrying the actionable `failureReason` "the model provider rejected this workspace's API key …" with `status="failed"` (t003 + t005); its sandbox was reclaimed promptly, leaving 0 lingering BatchSandboxes (t004). Backend + driver unit tests cover every behavior (turn-timeout injection, 401/403→report vs 429 relay, `ModelAuthFailer` ×4, failed-grace skip + open-SSH pin, `failureReason` completeness); `make lint-backend` = 0 issues; the rebuilt gateway image was imported to the CAPD nodes and rolled out to run the live proof.

## Definition of done

- A session created with an invalid or expired model key reaches `failed` in **seconds** (not the observed ~192s), and its `failureReason` names the authentication failure — verified on `dev-5`.
- `BEX_AGENT_TURN_TIMEOUT_MS` has a non-4h default wired through bex-api's `driverEnv` and the driver's fallback, documented in `.env.example` and the env inventory; a turn that never converges is bounded by that default rather than 4 hours.
- A terminally-**failed** session with no open editor SSH releases its sandbox on a bounded short grace (not the 30m completed-session editor-idle TTL), proven by watching the `BatchSandbox`/pod reclaim after a failed session.
- Every terminal-failure class (sandbox-create capacity/timeout, model-auth, turn-timeout) surfaces a non-empty, actionable `failureReason` on REST, GraphQL, and MCP, and the dashboard failed-session callout renders it — closing inbox note `w5/048`.
- ADR047 (and ADR059 for the reclaim-grace change) record the new failure-latency and failed-session reclaim policy.

## Source + Goal linkage

- **Source:** `dev-5` end-to-end agent-session run 2026-08-26/27 (create ~100ms → sandbox running 3–10s → agent ready <1s [`initialize` 132ms, `session/new` 736ms] → **prompt turn 191,677ms → single instant 401**; failed-session sandboxes still Running 4–21min later). Absorbs existing inbox note `w5/048` (empty `failureReason` on sandbox-create/capacity failures, `ags-da51u99jg4reds94nrv0`).
- **Goal linkage:** ADR008 pillar 5 / ADR047 cloud coding-agent sessions — makes the AI-native execution path fail fast, cheap, and legibly.
- **Expected outcome:** a bad/expired BYO key (the common misconfiguration) fails a session in seconds with a clear reason instead of ~3 minutes of user wait; a genuinely hung turn is bounded by a sane timeout instead of 4 hours; failed sandboxes stop holding plan quota for 30 minutes; and no terminal failure renders a blank card.
- **Why now:** the failure path wastes ~3 minutes of user wait plus up to 4 hours of sandbox compute per misconfigured key, and on a hobby workspace (~2-sandbox cap) a burst of slow-failing sessions blocks new work — a live self-wedge we reproduced on `dev-5`. Instrumentation to keep these bounds honest is the sibling milestone `w5/m81`.
- **Render parity included:** `failureReason`/`reason` is a user-facing field exposed on REST/GraphQL/MCP + dashboard (+ mobile). Agent sessions are a bex-original feature with no render.com equivalent, so the Render-parity task verifies **×3-surface + UI consistency and error-shape** of the reason string and the fast-fail timing, not render.com parity.
