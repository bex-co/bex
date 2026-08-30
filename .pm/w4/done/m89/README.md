# w4 · m89 — Honest agent-session states and a real error taxonomy

**Worker:** worker4 **Goal:** a tenant can tell "this platform was never set up" from "something is temporarily down" from "your workspace is missing a model key" — and a session that is merely starting no longer reads as broken **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Split `ErrAgentSessionsUnavailable` into distinct coded reasons — **DONE** | 1h | — |
| t002 | Map each code to specific dashboard copy — **DONE** | 45m | t001 |
| t003 | Model conversation states as not-started / connecting / live / broken — **DONE** | 45m | — |
| t004 | Surface capability gaps as pre-flight, not a post-hoc 503 — **DONE** | 30m | t002 |
| t005 | Render parity — coded errors identical across REST/GraphQL/MCP/UI — **DONE** | 30m | t002, t003, t004 |
| t006 | Simplify the code this milestone changed — **DONE** | 30m | t005 |
| t007 | Test coverage for the error taxonomy and the state machine — **DONE** | 40m | t005 |
| t008 | Closeout — **DONE** | 15m | t007 |

## Reviewed cause grouping

The original 17 `ErrAgentSessionsUnavailable` sites split into two reviewed groups:

- **`AGENT_SESSION_NOT_CONFIGURED` (14):** feature/configuration guards in `Create`, `List`, `setArchived`, `Delete`, `Transcript`, `Get`, `Resume` (feature and ticket guards), `rehydrate` (model proxy), `Cancel`, `setPin`, `modelCredential`, `Steer`, and `AttachTicket`. These are the only sites that keep the legacy `ErrAgentSessionsUnavailable` sentinel.
- **`AGENT_SESSION_SNAPSHOT_STORE_UNAVAILABLE` (3):** `Delete` when snapshot storage is absent, `Delete` when blob deletion fails, and `rehydrate` when a restore URL cannot be prepared. Nil `S3SnapshotStore` upload/download receivers use the same code defensively.

Three dependency paths that previously reached the same generic dashboard treatment were also normalized to **`AGENT_SESSION_DEPENDENCY_UNAVAILABLE`**: model-key readiness lookup, GitHub readiness lookup, and the OpenFGA tuple grant during create. Their internal causes are logged and the client receives only sanitized copy.

## Closeout evidence (2026-08-29)

- Browser-driven local runtime with GraphQL fault injection rendered two visibly distinct causes: not-configured said “Ask your operator…”, while dependency-unavailable said the setup remained intact and invited a retry. The retryable composer stayed usable. The isolated dev-4 harness was unavailable because the shared kind API was down, so the equivalent running local dashboard was used without changing production configuration. Evidence screenshots: `.playwright-mcp/m89-not-configured.png` and `.playwright-mcp/m89-dependency-unavailable.png`.
- An injected `Creating` session with an attach failure rendered “Starting the sandbox…”, the wait-for-current-turn helper, and a disabled button labeled “Send”. It contained none of “stream unavailable”, “live steering is paused”, or “Sending…”. Evidence screenshot: `.playwright-mcp/m89-creating.png`.
- Regression mutations were applied and reversed one at a time. The matching tests went red when dependency/configuration codes or copy were collapsed, a Creating attach error was classified as broken, the send label used transport busy state, or either model-key/GitHub preflight gate was removed.
- Verification passed: backend `go test ./...` and `make lint-backend` (zero issues); dashboard `yarn lint`, `yarn typecheck`, and `yarn test` (389 files, 2,860 tests); focused post-mutation suites (83 tests); and `git diff --check`.
- `/simplify` left one pure conversation-state derivation and one availability code-to-copy map. It also parallelized independent capability lookups and memoized composer preflight derivation.

## Definition of done

- A **transient dependency failure** and a **genuinely unconfigured platform** produce visibly different messages in the composer. Today both render "Agent sessions aren't configured — ask your operator to configure the agent-session gateway."
- No failure-shaped copy appears while a session is in its normal `Creating` phase. Today the composer says "The conversation stream is unavailable, so live steering is paused" while the banner says "Starting the sandbox…".
- The steering control never says "Sending…" for a turn the user did not send.
- A workspace missing a model key or a GitHub connection is told **before** it starts a session, using the existing `capabilities` projection, not by a 503 afterwards.
- Coded errors stay identical across REST, GraphQL and MCP, asserted by the existing cross-surface tests.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-08-28, findings 3 and 4 — hit live in `dev-1`, where a transient etcd timeout produced a 503 and the UI reported the platform had never been configured. `dashboard/src/features/agent-sessions/components/new-session-composer.tsx:171` maps **any** `AgentSessionsUnavailableError` to that one alert, and `lego/backend/internal/agentsessions/service.go` has **17** `ErrAgentSessionsUnavailable` return sites covering causes as different as "gateway not configured", "OpenFGA unavailable", "snapshot store unavailable" and "read model key failed".
- **Goal linkage:** pillar 5 (ADR008) and the ADR006 Render-compatible coded-error dialect, which agent sessions already follow for `AGENT_SESSION_INPUT_INVALID` / `_NOT_FOUND` / `_NOT_RESUMABLE`.
- **Expected outcome:** support burden drops and users stop being sent to an operator for faults that clear by themselves. An operator reading a report can tell which of the 17 causes fired.
- **Why now:** the collapse is actively misleading rather than merely terse, and it gets worse as pillar 5 opens up — every new dependency adds another cause to the same undifferentiated 503. Cheaper to split now, while the call sites are all in one service.
- **Render parity:** **included.** Render has no agent-session product (`docs/ADR018-render-parity.md`), so the parity task's job is bex's own cross-surface discipline: the new codes must appear identically on REST, GraphQL and MCP, in Render's error dialect.
