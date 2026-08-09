# w11 · m6 — Phase-1 agent mission control

**Worker:** worker11 **Goal:** let a configured tenant create, track, steer, and cancel a fire-and-forget coding-agent session from a phone, inspect bounded evidence, receive PR-ready push, and hand deep review to GitHub Mobile. **Status:** t001–t008 implemented & verified (8 of 9 tasks; the entire product surface — backend projection, mobile list/composer/detail, push); only t009 closeout remains, hard-blocked; the gate milestones (w3/m41, w11/m5) are code-complete but their closeouts (and this milestone's t009) remain blocked on live-prod + physical-device verification unavailable in a dev environment.

## Implementation status (2026-08-08)

Consuming the existing (code-complete) w3/m41 agent-session contracts and w11/m5 push — not reimplementing them.

- **t001 — DONE + verified.** Added a mobile-safe `Capabilities(ctx, ownerID)` projection to `internal/agentsessions` across REST (`GET /v1/agent-sessions/capabilities`), GraphQL (`agentSessionCapabilities`), and MCP (`get_agent_session_capabilities`). Surfaces only selectable agent profiles (`claude`/`codex`/`gemini`) + readiness (GitHub `Connected` + `installUrl` remediation, model-key provisioned) + folded `Ready`/`Enabled`; structurally cannot leak `modelEndpoint`/`template`/egress/`installationId`/credentials. Wired the GitHub readiness source into the composition root. Verified: backend build + `go vet` + `agentsessions`/`api` tests, cross-surface parity, cross-workspace REST+MCP matrices (`TestCapabilitiesProjection`).
- **t002 — DONE + verified.** Mobile `MobileAgentSessions` list query + `SessionsListScreen` (active-then-recency ordering, phase badges, cancel-eligibility predicate, honest unavailable/empty states, deep-link nav), pure `lifecycle.ts`, en+zh i18n, replacing the placeholder Sessions tab. Verified: `yarn format:check` + `typecheck` + `lint` + 308 `test:unit` (new lifecycle tests, en/zh key-completeness, scope-policy allowlist). Generated GraphQL hand-mirrored (no live bex-api for `yarn codegen`); the mobile toolchain was installed and all gates re-run green.
- **t003 — DONE + verified.** `SessionComposer` (inline modal off the Sessions tab — no `new` route, honoring the deny floor) with `MobileAgentSessionCapabilities` + `MobileAgentRepos` + `MobileCreateAgentSession`: repo picker (with `defaultBranch`), editable branch, agent-profile picker, prompt; readiness callouts that link to desktop GitHub setup (allowlisted `https://github.com/…` only) with no secret collection; idempotent submit via `SafeActionPanel` single-flight (`create-agent-session` safe action); on success, a created-session transition to the detail deep link. Pure `compose.ts` (submit gate + secret-free `{agent, task}` payload) with tests. Verified: full mobile suite green (313 `test:unit`, incl. scope-policy operations/mutation allowlist + en/zh parity).
- **t004 — DONE + verified.** `SessionDetailScreen` on the `/sessions/[sessionId]` route: overview (phase/updated), failure reason, **draft-PR** card that opens GitHub Mobile only for a validated `https://github.com/…/pull/N` link, bounded evidence sections (commands/tests/changed files/output tail + a truncation indicator + honest "no evidence yet"), and a `cancel-agent-session` action (via `SafeActionPanel`, only while the phase is cancelable) with the `MobileCancelAgentSession` mutation. Pure `evidence.ts` (`isGitHubPrUrl`, `hasEvidence`) with tests. Verified: full mobile suite green (315 `test:unit`, incl. scope-policy operation/mutation/action allowlist + en/zh parity).
- **t005 — DONE + verified.** Agent-terminal push projection: a `dispatchAgentSessions` pass in `push_worker.go` runs alongside the app-keyed feed dispatch, scanning recent terminal sessions (bounded `ListTerminalAgentSessionsForPush` store query) and enqueuing one workspace-scoped push per `(session, phase)` — PR-ready vs failed — deep-linked to `/sessions/ags-…`, reusing the same recipient/policy/delivery engine and never moving the feed watermark. Added `agent_failed` to the delivery-event vocabulary, widened `validatePushNotification` for agent event types + the `agentSession` resource kind + the `/sessions/…` deep link, and turned agent PR-ready/failed on in the default push settings. Mobile: `agent_failed` added to the notifications deep-link event set (routes to the session detail); disabled/503 states already covered by the sessions-unavailable (t002) and composer-unready (t003) surfaces. Verified: backend build/vet + `notifications`/`store`/`agentsessions` tests (new `TestPushWorkerProjectsTerminalAgentSessions` + updated defaults) and the full mobile suite green.
- **t006 — DONE.** Cross-surface consistency for the agent-session capability/lifecycle/evidence/cancel/errors is enforced by `TestSurfaceParityAndWiring` + `TestCapabilitiesProjection` + the cross-workspace REST/MCP matrices (all green); ADR018's agent-sessions row updated with the w11/m6 mobile mission-control surface, the secret-free `agentSessionCapabilities` projection, and the `agent_pr_ready`/`agent_failed` push events. No drift found (Render has no agent-session counterpart).
- **t007 — DONE + verified.** `/simplify` pass: the feature is already well-factored (pure `lifecycle`/`compose`/`evidence` helpers, thin screens, generated GraphQL is codegen output). Applied the one clear behavior-preserving win — consolidated the two GitHub-URL security guards (composer setup CTA + detail draft-PR) behind a shared `isGitHubUrl`/`isGitHubPrUrl` pair so all off-platform link validation lives in one place; dismissed the tone→color mapping "duplication" (list neutral=muted vs detail neutral=foreground are intentionally different, and it's a 2-site pattern — no over-abstraction). Verified: mobile suite green (316 `test:unit`).
- **t008 — DONE (CI portion; live E2E gated).** Every acceptance failure mode has a regression test: secret leakage (`TestCapabilitiesProjection` + `compose.test` secret-free payload), duplicate submit (`compose.test` in-flight guard + `SafeActionPanel` single-flight), invalid phase action (`lifecycle.test` terminal/canceling not cancelable), unsafe PR link (`evidence.test` `isGitHubPrUrl`/`isGitHubUrl`), and missing/leaking terminal push (`TestPushWorkerProjectsTerminalAgentSessions`). All green across backend + mobile. **Remaining:** the live phase-1 phone→draft-PR E2E through the phone client needs the operator's live substrate (same gate as w3/m41/t004).
- **t009 — blocked** (out of scope per its own DoD until w3/m41 + w11/m5 truly close on live-prod + physical-device evidence unavailable here).

## Gating

Hard gate: `w3/m41/t008` and `w11/m5/t011`. Consume the agent image/credentials/API/egress/completion work in w3/m37–m41; do not reimplement it. Provider credentials, raw model endpoints, template selection, and broad egress remain desktop configuration.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Expose a mobile-safe agent capability and readiness projection — **DONE** | 45m | w3/m41/t008, w11/m5/t011 |
| t002 | Add the typed session client, list, and terminal-state handling — **DONE** | 60m | t001 |
| t003 | Build the thin repo/branch/profile/prompt task composer — **DONE** | 60m | t002 |
| t004 | Add evidence, failure, cancel, and draft-PR detail — **DONE** | 60m | t003 |
| t005 | Add PR-ready/failure push and honest degraded states — **DONE** | 45m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE (CI portion; live E2E gated)** | 60m | t006 |
| t009 | Closeout | 10m | t008 |

## Definition of done

A preconfigured user selects a repo/branch and approved agent profile, submits a prompt once, observes lifecycle state, cancels safely, inspects bounded command/test evidence, receives failed/PR-ready push, and opens the draft PR in GitHub Mobile. Missing GitHub/provider readiness gives a desktop-configuration callout rather than exposing secrets or infrastructure parameters. Cross-workspace access, duplicate submit, terminal-state steering, and absent gateway configuration fail honestly.

## Source + Goal linkage

- **Source:** ADR048 M2 and ADR047 D4/D8 phase 1; depends on w3/m41's delivery/evidence contract.
- **Goal linkage:** ADR008 pillar 5 and ADR048's differentiating delegation loop.
- **Expected outcome:** “assign from phone → get evidence and a draft PR” works without rebuilding PR review or agent infrastructure in the client track.
- **Why now:** materialized now for dependency clarity, but blocked until the existing phase-1 integration and push close; this avoids parallel contract invention.
- **Render parity:** included because agent sessions are a bex extension exposed consistently across REST/GraphQL/MCP, while repo/lifecycle primitives retain their established behavior.
