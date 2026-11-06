# w9 · m176 — Approval mode: the agent asks before risky actions (needs_input)

**Worker:** worker9 **Goal:** a session can pause in `needs_input` asking the user to approve or answer, and resumes when they reply — on API, MCP (state=needs_input in wait), dashboard and push. **Status:** in progress — t001–t003, t005 done; next t004 parity, then the live pass

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Backend needs_input lifecycle and contracts — **DONE** | 90m | — |
| t002 | Approval policy and bypass — **DONE** | 60m | t001 |
| t003 | Dashboard: answer a pending decision — **DONE** | 60m | t001 |
| t004 | Render parity (Devin reference for agent surfaces) | 25m | t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- A live session pauses for approval before a risky action, appears as needs_input on REST/GraphQL/MCP/wait, and continues after an approve/answer from the dashboard; bypassApproval skips it.

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** ADR008 pillar 5; ADR094 D7 needs_input; Devin parity (approval mode).
- **Expected outcome:** customers can let agents run autonomously but gate risky steps.
- **Why now:** `needs_input` is reserved in the shipped contract but never emitted; w11/m7 waits on it.
- **Parity:** included — Devin reference.

## Design decision (2026-11-06, loopx w9, Devin-reference rule)

- **Source:** existing rule (cloud-agent work matches Devin) plus verified Devin behavior. v3 `status_detail` has `waiting_for_user` and `waiting_for_approval` ("awaiting action approval in safe mode"); create takes `bypass_approval`; Devin's permission model asks before destructive, publishing, cloud-CLI and outside-workspace actions ([Get Session](https://docs.devin.ai/api-reference/v3/sessions/get-organizations-session), [Create Session](https://docs.devin.ai/api-reference/v3/sessions/post-organizations-sessions), [permissions](https://docs.devin.ai/cli/reference/permissions)).
- **Chosen:** [ADR100](../../../docs/ADR100-agent-approval-mode.md).
  - `needs_input` gets a `waitingFor` field (`approval` or `user`), backed by durable `agent_session_decisions` with a 24h expiry.
  - Workspace `approvalMode` is `autonomous` (default, unchanged behavior), `risky` or `all`, plus `bypassApproval` on create.
  - The driver gates ACP `request_permission` against one risky-action table, and an `ask_user` tool covers questions.
- **Effect on tasks:**
  - t001 implements D1, D2 and D5 (backend and driver channel).
  - t002 implements D3 and D4 (policy, bypass, `ask_user`).
  - t003 implements D6 (dashboard callout and Blocked group).
  - w11/m7 t003 can consume the D2 answer route.

## CLI follow-through (2026-11-06, from w9/m177 t004)

- When this milestone adds its create/steer field, add `--approval-mode` and `--bypass-approval` on `bex agent run` (ADR100 D3) in `lego/cli/internal/agent`, with a contract test, as part of the implementation task.

## Live check (2026-11-06)

- **Live check attempt 2026-11-06 ~13:10 UTC: NOT RUN.** Production bex-api is still on the 92ff00293 pin (gitops `bab5d417e`). `GET /v1/agent-skills` and `GET /v1/agent-secrets` answer 404, and capabilities has no per-profile `modelKeyReady`. Every deploy run since then (379af6c08, 3310f6484, 8c498d33c) ended `success` with `build`/`deploy` skipped by supersession, so none of this work is live yet. Rerun after a real pin.

## Live check (2026-11-06, production 24a9203ee)

- Production pin 24a9203ee (gitops `d65dc2f2b`). An authenticated `GET /v1/agent-skills` and `GET /v1/agent-secrets` answered 200. QA user via CLI device login, workspace `bex` (QA user is an admin there). Every fixture was deleted and verified 404; the CLI grant was revoked (204, then 401) and the Kratos session logged out.
- **PASS — `ask_user`.** Session `ags-dbmu3be6kvbs738fl6dg` (`approvalMode: risky`) paused in `state: needs_input`, `waitingFor: user`. `outcome.pendingDecision` was `dcs-dbmu3eu6kvbs738fl6h0` (kind question, options red/blue). It was answered through MCP `answer_agent_session_decision {id, decisionId, answer: "blue"}`; the turn finished `succeeded` with the reply `PREFERS-BLUE`. A second REST answer was refused `409 AGENT_SESSION_DECISION_ANSWERED`.
- **FAIL — approval gating never fires on claude.**
  - Session `ags-dbmu3ba7qbhc739704r0` (`approvalMode: risky`) was told to run `sudo true`. It ran the command without pausing (exit 127, sudo not installed), and no `pendingDecision` appeared.
  - Session `ags-dbmu4ju6kvbs738fl770` (`approvalMode: all`) ran `echo qa-api-hello` without pausing.
  - So no ACP `session/request_permission` reaches the driver's policy for the claude engine. The backend sets `BEX_AGENT_APPROVAL_MODE` and `BEX_AGENT_DECISION_URL` (`lego/backend/internal/agentsessions/decisions.go:410-421`), and the decision channel works, since `ask_user` paused. The driver only gates inside `requestPermission` (`lego/agent-image/driver/src/acp.ts:433`, `session.ts:247`).
  - Likely cause: claude-agent-acp, as launched (profile `permissionPolicy: sandbox-auto-approve`, `lego/backend/internal/agentsession/agent-profiles.json`), auto-approves tool calls and never asks the client. The driver tests use the fake ACP agent (`driver/fixtures/acp-agent.mjs`), so the real engine path is untested.
  - The REST deny path could not be exercised live, because no approval was ever opened.
- **Not run:** the needs_input webhook (optional) and the dashboard callout (browser).

## Browser check (2026-11-06, production 24a9203ee)

QA user in the `bex` workspace, Playwright at 1440×900 and 375×812. Fixtures used the `qa-ui-` prefix and were all deleted; the Kratos session was logged out.

- **PASS — Settings and composer.** Settings → Agents shows the Approvals section with three modes. The composer popover offered "Workspace default (Autonomous)", "Ask before risky actions" and "Ask before every action", and the button read "Approvals: Ask before risky actions".
- **FAIL — no pause.** Session `ags-dbmu4n27qbhc739705q0` was stored with `approvalMode: "risky"` (GraphQL). It ran `sudo true` without any decision, failing only on `sudo: command not found`, and finished Completed.
  - bex-api logged no decision for it, though a question decision (`dcs-dbmu3eu6kvbs738fl6h0`, `ask_user`) did open for another session.
  - The sandbox ran the pinned agent image `sha256:1074e2a8…`, and its logs had no approval lines.
  - This matches the API check: the claude engine never sends `request_permission`. Likely cause: `permissionPolicy: sandbox-auto-approve` in `agent-profiles.json`.
  - The callout's Deny path and the Blocked group could not be exercised.
- **Skeleton (session page): minor.** The header is 98px pending against 104px ready, a 6px shift. The composer bar matches.

## Production finding fixed (2026-11-06)

- **Cause:** approvals never fired for claude on 24a9203ee. claude-agent-acp sends a client advertising the AIR extension a slim permission `toolCall` without `kind`; `kind` lives only on the earlier `tool_call` update (`dist/tool-calls/renderer.js` `permissionToolCall`). The driver read the missing kind as "other" and allowed `sudo true` once.
- **Fix:** the driver now remembers each announced tool call and completes the permission request before gating. A call with no kind but a shell command reads as execute.
- **Verification:** reproduced and verified with the real adapter (opt-in `test/real-adapter-approval.test.ts`).
- **Not a bug:** `echo` in `all` mode is Claude Code's own read-only auto-allow, which ADR100's "every non-read-only action" permits.
- **Follow-up, codex:** codex-acp 1.4.0 applies its own on-request/workspace-write policy per turn and asks only for out-of-sandbox requests, so in-sandbox risky commands are not gated. Recorded in ADR100.
- **Next:** a live recheck after the fix deploys, then closeout.

## Recheck (2026-11-06, production b8370c30c)

Production pin b8370c30c (gitops `ca9b90b8f`, deploy run 42238432288 success). Signed in as the QA user in the browser; API calls were made from it with the session cookie (REST, and MCP over `/mcp/cloud-agents`). The CLI device flow stopped at a Kratos reauthentication prompt, so no CLI grant was issued. Every fixture was deleted (DELETE 204, then GET 404), the Kratos session was logged out with this run's jar, and the run dir was removed.

- **PASS — risky approval over REST.** `ags-dbmvh67uqs1c73d5q0pg` (`approvalMode: risky`, "run `sudo true`") paused at `needs_input`, `waitingFor: approval`. Its `outcome.pendingDecision` was `dcs-dbmvhkvuqs1c73d5q100`: "Run (run as root): sudo true", rule `sudo`, `toolKind` execute, `policyVersion` 2. A REST deny (`POST …/decisions/dcs-…`, `{allow:false}`) returned 200 with state `denied`. The turn then succeeded with "The command `sudo true` was not run — the permission prompt was refused". The transcript shows the refusal and no `sudo` output.
- **PASS — all-mode approval over MCP.** `ags-dbmvi4hpc2us73bhtdg0` (`approvalMode: all`, `touch qa-re.txt && ls -l qa-re.txt`) opened `dcs-dbmvi7nuqs1c73d5q1h0` with rule `all:execute:touch qa-re.txt` and paths `/workspace/qa-re.txt`. MCP `answer_agent_session_decision` (`allow: true, scope: session`) returned 200. The turn succeeded, and `ls -l` showed the new 0-byte file.
- **PASS — dashboard.**
  - The composer approval picker set "Approvals: Ask before risky actions". The session `ags-dbmvit7uqs1c73d5q1o0` paused.
  - The callout showed "Run (run as root): sudo true", the command and "Matched rule: sudo", with Allow once / Allow for this session / Deny.
  - The nav showed "Blocked 1" with that session, and the header showed a Blocked badge.
  - Clicking Deny removed the callout. The turn succeeded with "The command `sudo true` was refused by the permission system, so it did not run."
  - The first REST session also showed the callout and the Blocked group while pending.
- **Seen, out of scope:** the session page logs React error #418 (hydration text mismatch) on load.
- The production finding is fixed: approval mode now gates claude tool calls in risky and all modes. The m176 DoD (pause, needs_input on REST/MCP, answer from the dashboard, continue) holds live.
