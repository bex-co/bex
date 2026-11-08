# w9 · m179 — Remove approval mode (ADR100 withdrawn)

**Worker:** worker9 **Goal:** cloud agent sessions are purely autonomous again, and every approval-mode surface w9/m176 shipped is gone from the backend, gateway, driver, dashboard, CLI and docs. **Status:** done (2026-11-08)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Backend: remove decisions, approval fields and routes; drop the schema — **DONE** | 90m | — |
| t002 | Gateway and driver: remove the decision channel, gating, `ask_user` and codex rules — **DONE** | 60m | — |
| t003 | Dashboard: remove the callout, Blocked grouping, settings section and composer picker — **DONE** | 60m | — |
| t004 | CLI and docs: remove `--approval-mode`/`--bypass-approval`/`answer`; mark ADR100 withdrawn — **DONE** | 30m | t001 |
| t005 | Simplify — **DONE** | 15m | t001, t002, t003, t004 |
| t006 | Test coverage — **DONE** | 30m | t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- On production, the API no longer exposes `approvalMode`, `bypassApproval`, `waitingFor`, `pendingDecision`, the decision answer route or `answer_agent_session_decision`, and a create that sends `approvalMode` is refused as an unknown field (strict decoding).
- A session told to run `sudo true` runs it without pausing.
- The dashboard has no approval UI.
- The `agent_session_decisions` table and the approval columns are dropped by a new migration.
- ADR100 reads Withdrawn, and ADR094 keeps `needs_input` reserved and never emitted.

## Source + Goal linkage

- **Source:** user decision 2026-11-07 ("整体移除 然后放到 do not do"); `.pm/DO_NOT_DO.md` #APPROVAL-MODE.
- **Goal linkage:** ADR008 pillar 5 (long-running unattended cloud agents); ADR090's autonomy contract.
- **Expected outcome:** less code to keep correct, and one behavior for every session.
- **Why now:** the feature just shipped (default off), so removing it before anyone depends on it is cheap.
- **Parity:** omitted. This removes an optional surface; Devin's cloud sessions are autonomous by default.

## Live check (2026-11-08, production 19dfc2982)

Production pin 19dfc2982, deploy job success, workspace `bex`. The QA user signed in with the cookie flow, so no CLI grant was minted.

- **PASS: create refuses the approval fields.**
  - `POST /v1/agent-sessions` with `approvalMode: "risky"` → `400 request body contains unknown field "approvalMode"`.
  - The same create with `bypassApproval: true` → `400 … unknown field "bypassApproval"`.
- **PASS: the environment has no approval fields.**
  - `GET /v1/agent-environment` has no `approvalMode`/`approvalBypassAllowed` keys.
  - `PATCH` with `approvalMode` → `400 … unknown field "approvalMode"`.
- **PASS: the decision routes and tools are gone.**
  - `POST /v1/agent-sessions/{id}/decisions/dcs-x` → 404.
  - `/mcp/cloud-agents` `tools/list` returns 23 tools, none for decisions, and `create_agent_session` is present.
  - GraphQL introspection has no `answerAgentSessionDecision`, no `AgentSessionDecision` type, and no `waitingFor`/`pendingDecision`/`approvalMode`/`approvalBypassAllowed`/`bypassApproval` fields. The only `*Decision` type left is `ActionDecision`, the unrelated deploy action capability in `core/decision.go`.
- **PASS: risky commands run without pausing.**
  - A repo-less claude session `qa-rm-sudo` was told to run `sudo true`. Its states were `working → succeeded`, never `needs_input`.
  - The transcript shows the Terminal tool ran `sudo true; echo "EXIT=$?"` (`output-available`), and the summary was `EXIT=127` (no sudo in the sandbox).
  - The session view had no `waitingFor`/`approvalMode` and no `outcome.pendingDecision`.
- **PASS: the dashboard has no approval UI.**
  - `/agents` loads the composer (Start session present) with no Approvals button and no Blocked group.
  - `/workspace/settings` shows Agent secrets, Agent skills and Default model, with no approval section text.
- **Cleanup.**
  - The session was deleted (204, then 404).
  - Both Kratos sessions were logged out with their real jars (`ok logged-out`), and the jars were removed.
  - The private /tmp run dir was removed, and no files were left in the repo root.
