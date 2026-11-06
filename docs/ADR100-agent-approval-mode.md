# ADR100 — Approval mode for cloud agents (`needs_input`, Devin parity)

**Status:** accepted (design, w9/m176 t001) · **Date:** 2026-11-06 · **Extends:** [ADR094](ADR094-coding-agent-mcp.md) D7 (`needs_input`), [ADR047](ADR047-cloud-coding-agent-sessions.md) (driver), [ADR090](ADR090-agent-owned-delivery.md) (overlay) · **Unblocks:** w11/m7 t003 (decision-request contract)

## Context

ADR094 reserved the coarse state `needs_input` and the webhook event `agent_session_needs_input`, but nothing emits them. bex agents always run fully autonomously. The driver answers every ACP `session/request_permission` with the engine's `allow_always` option (`lego/agent-image/driver/src/acp.ts`), and a question the agent wants to ask can only be stated at the end of its turn.

Customers want to let an agent run unattended and still be asked before a risky step. The mobile client (w11/m7) is waiting on a durable "decision request" to show and answer.

**What Devin does (verified 2026-11-06):**

- The v3 session status `running` carries `status_detail` with these values ([Get Session](https://docs.devin.ai/api-reference/v3/sessions/get-organizations-session), [Create Session](https://docs.devin.ai/api-reference/v3/sessions/post-organizations-sessions)):

  - `working`;
  - `waiting_for_user`: needs user input;
  - `waiting_for_approval`: "awaiting action approval in safe mode";
  - `finished`.

  v1 folds both waits into `blocked`, and API pollers stop on it ([v1 usage examples](https://docs.devin.ai/api-reference/v1/usage-examples)).

- Create takes `bypass_approval` ("allows bypassing approval workflows") and an optional `security_profile`.
- Devin's CLI and Desktop define which actions need approval ([permissions](https://docs.devin.ai/cli/reference/permissions)):
  - Routine build/test/lint runs without a prompt.
  - Package installs, downloads that execute code, mutating git, `rm`, `sudo`, cloud CLIs and anything destructive always ask.
  - Org deny/ask rules override a member's own rules and Bypass.
  - A prompt offers Allow (once or for the session) and Deny.

## Decision

### D1 — Two waits, one coarse state

A turn can pause in `needs_input` for one of two reasons, exposed as `waitingFor`:

| `waitingFor` | Devin `status_detail` | Raised by |
| --- | --- | --- |
| `approval` | `waiting_for_approval` | the engine asks permission for a tool call that the session's policy (D3) gates |
| `user` | `waiting_for_user` | the agent calls the driver's `ask_user` tool (D4) |

While paused, the session's phase stays `running`, so the sandbox, its stream and the turn are all kept. The derived `state` is `needs_input` and `steerable` is false. `wait_agent_sessions` settles on it (ADR094 D7), the dashboard and push show it, and the webhook `agent_session_needs_input` fires once per decision.

### D2 — The decision record

```sql
agent_session_decisions (
  id            text primary key,    -- dcs-<xid>
  session_id    text not null,
  workspace_id  text not null,
  turn          int  not null,
  kind          text not null,       -- 'approval' | 'question'
  title         text not null,       -- short, e.g. "Run: git push --force origin main"
  detail        jsonb not null,      -- approval: {toolKind, command?, paths?, rule}; question: {question, options?}
  state         text not null,       -- 'pending' | 'allowed' | 'denied' | 'answered' | 'expired' | 'canceled'
  answer        jsonb null,          -- approval: {scope: once|session}; question: {text}
  answered_by   text null,
  created_at    timestamptz not null,
  answered_at   timestamptz null,
  expires_at    timestamptz not null -- created_at + 24h
)
```

A turn has at most one pending decision at a time: the engine blocks on the answer. Text in `title`/`detail` goes through the ADR098 redactor before it is published.

**Answering.** The route is `POST /v1/agent-sessions/{id}/decisions/{decisionId}`, with the same verb on GraphQL (`answerAgentSessionDecision`) and MCP (`answer_agent_session_decision`). The body is `{allow: bool, scope?: "once"|"session"}` for an approval or `{answer: string}` for a question.

- It needs `can_create` on the workspace, the same relation as steer.
- The answer is idempotent by decision id. A second answer is refused `409 AGENT_SESSION_DECISION_ANSWERED`; a stale or expired one is refused `409 AGENT_SESSION_DECISION_EXPIRED`.
- Every answer is audited.

The session view carries `pendingDecision` (the record without `answer`) while one exists.

**Expiry.** A decision unanswered for 24 hours is resolved automatically: an approval as **deny**, a question as "no answer; continue with your best assumption". The turn then continues, so a forgotten session neither holds a sandbox forever nor silently does the risky thing. Cancel resolves the pending decision as `canceled`.

### D3 — Policy: which tool calls need approval

The session's `approvalMode` decides:

- `autonomous`: today's behavior, nothing is gated.
- `risky`: the default list below asks.
- `all`: every non-read-only tool call asks.

**Where the mode comes from.** It is set per workspace in agent settings, with `autonomous` as the default so existing sessions keep their behavior. Create may lower it with `bypassApproval: true` (Devin's `bypass_approval`) or set it explicitly with `approvalMode`. A member can lower the mode only when the workspace setting allows it, mirroring Devin's org rules overriding Bypass.

**`risky` asks before:**

- Shell commands that match the gated list:
  - `git push --force` or `-f`, and deleting a remote branch;
  - `git reset --hard` on the base branch;
  - `rm -rf` outside the repo checkout;
  - `sudo`;
  - package publishing (`npm publish`, `cargo publish`, `twine upload`, `gh release`);
  - cloud and infra CLIs that mutate state (`kubectl delete/apply`, `terraform apply`, `aws`/`gcloud`/`az` mutating verbs);
  - `curl`/`wget` piped into a shell.
- Writes outside the checkout and `/tmp`.
- Mutating calls to the bex MCP or the Render API (deploys, deletes, env changes).

The list lives in one versioned table in the driver. The engine's ACP `toolCall` (kind, title, raw input) is matched against it. Delivery to the session's own draft PR is never gated: it is the product's normal output.

### D4 — `ask_user`: questions as a tool, not end-of-turn text

The driver serves an `ask_user` tool to the engine, next to `share_file` and `submit_result`. It takes `{question, options?}` and blocks until the answer arrives, then returns it as the tool result.

The overlay changes only when the session has `askUser` enabled; it is on when `approvalMode` is not `autonomous`. In that case the overlay says "ask with `ask_user` when genuinely blocked". Otherwise the ADR090 rule stands: "make reasonable assumptions, ask at the end".

### D5 — Mechanism in the driver

1. ACP `session/request_permission` is matched against the policy (D3):
   - not gated → answer `allow_always` or `allow_once` as today;
   - gated with a session-scoped allow already granted → allow;
   - otherwise post the decision to bex-api over the existing Pod-bound driver channel (the one `submit_result` uses), then wait.
2. bex-api records the decision, flips the derived state to `needs_input`, and fans out the webhook, push and wait notifications.
3. The answer reaches the driver over the same channel, by long-poll or the status stream. The driver maps it to the engine's `allow_once`, `allow_always` (scope `session`) or `reject_once` option and resumes. The engine sees an ordinary permission outcome.

### D6 — Dashboard and clients

- **Session page:** a pending decision renders as a callout above the composer. An approval shows the command or paths, the matched rule, **Allow once**, **Allow for this session** and **Deny**. A question shows its text with an answer box and any options.
- **Session lists:** they group `needs_input` as **Blocked**, as Devin's sidebar does.
- **Agent settings:** the workspace setting appears there with the three modes.
- **Composer:** it gains an approval-mode selector, limited to what the workspace allows.
- **Mobile (w11/m7):** it uses the same answer route.

Skeletons follow the ready geometry rule.

## Consequences

- Devin parity for `waiting_for_user`/`waiting_for_approval` and `bypass_approval`. `needs_input` becomes real on every surface, and w11/m7 gets its decision contract.
- The default stays `autonomous`, so no existing session changes. Gating is opt-in per workspace.
- A paused turn keeps its sandbox. The 24h expiry bounds the cost, and the ADR059 hibernation rules treat a pending decision as activity for at most that window.
- **Follow-ups, not gates:**
  - Devin-style "Smart" auto-approval by a fast model;
  - org-level deny rules (`security_profile`);
  - per-rule editing in the dashboard.

## As built (2026-11-06, w9/m176 t001–t002)

Backend, gateway, driver and CLI shipped together. Where this differs from the design above, this section is the contract.

- **Storage.** Migration 0205 adds `agent_session_decisions` (`dcs-` ids). The session row mirrors its oldest open decision (`pending_decision_id`, a foreign key `ON DELETE SET NULL`, and `pending_decision_kind`) and how many are open (`pending_decision_count`). Every read derives `needs_input` from the mirror without a join: Go `sessionState`, and SQL `store.AgentSessionStateSQL` for the `states` list filter. One helper (`refreshPendingDecisions`) recomputes the mirror after every open, answer and expiry. A trigger on the phase change cancels the open decisions and clears the mirror, whichever path ended the turn. The state NOTIFY also fires on the mirror and the count, so `wait_agent_sessions` and a held await wake on a pause or an answer.
- **Several decisions at once, idempotent opens.** An engine may ask about parallel tool calls at the same time (Claude Code does), so each open adds a decision and leaves the others open; the session stays `needs_input` until the last is answered. The driver sends its own `requestId`, so a retried open returns the decision it already opened.
- **Lock order.** Every decision write locks the session row first and the decision rows second, the order the phase trigger uses. An answer racing a cancel or turn end therefore waits instead of deadlocking. The expiry sweep claims sessions with `FOR UPDATE SKIP LOCKED` and refreshes their mirrors by primary key.
- **Where the decision is shown.** The view carries `waitingFor` (`approval`/`user`, from the oldest open decision) and `approvalMode`. The decision itself is `outcome.pendingDecision`, the oldest open one, filled like `outcome.summary` by the single read, wait and webhook. A list page shows only `waitingFor`.
- **Answer.** The route is `POST /v1/agent-sessions/{id}/decisions/{decisionId}`, with GraphQL `answerAgentSessionDecision` and MCP `answer_agent_session_decision` on `/mcp/cloud-agents`.
  - The body is `{allow, scope}` for an approval and `{answer}` for a question; the stored answer is `{allow, scope}` / `{text}`.
  - Refusals: `400 AGENT_SESSION_DECISION_INVALID`, `404 AGENT_SESSION_DECISION_NOT_FOUND`, `409 AGENT_SESSION_DECISION_ANSWERED`, `409 AGENT_SESSION_DECISION_EXPIRED`.
  - Concurrent answers resolve in one row-conditional UPDATE, so exactly one wins.
- **Expiry.** A one-minute sweep (`RunDecisionSweep`), plus an inline check on await, resolves decisions past 24 h: an approval as `{"allow":false}`, a question as `{"text":""}`, both with state `expired`.
- **Policy (D3).** The workspace setting is `approvalMode` plus `approvalBypassAllowed` (default `true`), on REST, GraphQL and MCP `update_agent_environment`; admins only.
  - Create takes `approvalMode` and `bypassApproval` (CLI `--approval-mode`, `--bypass-approval`).
  - Raising the mode is always allowed. Lowering it, or bypassing, is refused `403 AGENT_SESSION_APPROVAL_BYPASS_FORBIDDEN` when bypass is not allowed.
  - Asking for a mode a deployment without the decision store cannot honor is `503 AGENT_SESSION_APPROVAL_UNAVAILABLE`.
  - The effective mode is stored on the session and is part of the idempotency digest.
- **Channel (D5).** The gateway serves `POST /agent-decisions/<ns>/<session>` (open) and `…/<dcs-id>` (await) on the agent listener (`:8082`), so the sandbox egress needs no change. It forwards a `agent-decision`-domain HMAC envelope to bex-api's internal `/v1/agent-session-decisions` (`BEX_AGENT_DECISION_API_URL`).
  - An await is held up to 20 s at bex-api, under its 30 s write timeout, and the driver polls again. A held await wakes on its own session's state notification and re-reads at most every 15 s without one; a decision found past its expiry is swept at most once per hold.
  - Awaits hold a slot on their own limiter (2048 per gateway, 4 per sandbox), never one of the 64 shared by Git, PR, share and result requests, so paused sessions cannot starve other sandboxes. They are not charged to the per-session request budget, so a day-long wait cannot exhaust it. An await bex-api answered at once instead of holding (an answer, an unknown decision, a refusal) is charged to a separate quick-await budget (1000 per session), so a sandbox cannot spin on bogus awaits. Opens are charged to the request budget.
- **Driver.**
  - `BEX_AGENT_APPROVAL_MODE` and `BEX_AGENT_DECISION_URL` are injected only for a non-autonomous session; without the URL the driver grants everything, as before.
  - `src/approval.ts` holds the versioned risky-action table (`approvalPolicyVersion` 2). Rule ids are `git-force-push`, `git-delete-remote-branch`, `git-reset-hard-remote`, `sudo`, `package-publish`, `infra-mutation`, `cloud-mutation`, `pipe-to-shell`, `decode-and-run`, `platform-api-mutation` (curl/wget mutations of `api.render.com` and `api.bex.co`), `rm-outside-checkout`, and `command-too-long` for a command over 16 KiB.
  - Before matching, a command is normalized. Quotes and backslashes are dropped, a program is read by basename (`/usr/bin/sudo`, `\sudo`), env assignments and wrappers (`env`, `nohup`, `doas`/`sudo`) are looked through, git's global options (`-C`, `-c`, `--git-dir`) are skipped, substitutions and `bash -c` scripts are read as commands, and `rm` targets are resolved against the directory the command has `cd`'d to and through existing symlinks.
  - **Risky mode is a guardrail against an agent's mistakes, not a sandbox boundary.** It covers the common spellings of the gated actions; a determined agent can still obfuscate a command, and the sandbox (ADR047 D5) is what confines it.
  - An answer maps onto the engine's own option. A "session" allow is `allow_always` and is remembered for the rest of the turn under its rule. In `all` mode an execute call's rule is its program and subcommand (`all:execute:git push`), so allowing one command does not allow every later one. Every other grant in an approval mode is `allow_once`, because an engine persists `allow_always` as a rule (Claude Code: a command prefix) that would silence a later ask. An autonomous session grants `allow_always`, as before.
  - The driver's own MCP tools (`open_or_update_draft_pr`, `share_file`, `submit_result`, `ask_user`) are never gated, recognized only by exact identity: Claude's title `mcp__bex__<tool>` (kind other) or Codex's rawInput `{server: "bex", tool}` without a shell command. A command that merely mentions one is still a command.
  - Decision text is scrubbed with the ADR098 redactor before it leaves the sandbox, string by string through the detail (not its JSON text). `ask_user` scrubs with the secret values its own process inherits.
  - Waits on people do not count against the turn timeout. Every decision writes its own entry in `decision-waits/` next to the status file, and the turn timer pushes its deadline by the union of their intervals, so parallel waits count once. The clock starts when the decision is open: retries of an unreachable gateway are the agent's time, and they stop when the decision's 24 h lifetime would have ended.
- **Notifications.** Webhook `agent_session_needs_input` fires once per decision. Push `agent_needs_decision` is generic lock-screen text, deduped per decision; its "not sent yet" flag is gone.
- **CLI.** `bex agent answer <ags-id> (--allow [--scope session] | --deny | --text …)`. `wait` exits 5 on `needs_input` and names the pending decision.
- **Not covered yet:** a resume of a sleeping session does not re-check the mode. The session keeps the mode it was created with, so a later workspace change does not reach running sessions. The dashboard (D6) is a separate task.

### Production finding and fix (2026-11-06, w9/m176)

**Symptom.** On production (24a9203ee), a `risky` session ran `sudo true` and an `all` session ran `echo` with no decision, while `ask_user` paused correctly.

**Root cause.** The permission request reached the driver, but without its `kind`.

- The driver advertises the JetBrains AIR client extension (`typedSessionFailureCapabilities`, for codex-acp's typed failures).
- Toward an AIR client, claude-agent-acp 0.94.0 sends a permission request's toolCall as only `{toolCallId, title, rawInput}`. The rest, including `kind`, is on the `tool_call` it announced earlier (`dist/tool-calls/renderer.js` `permissionToolCall`: "The client already holds the rest").
- `gateFor` read the request alone, treated the missing kind as `other`, and granted `sudo true` with `allow_once`.
- The fixture agent always sent a full toolCall, so the tests missed it. A real-adapter run through the driver reproduced it: the command executed, and no decision was opened.

**Fix.**

- `acp.ts` remembers each announced tool call (`kind`, `title`, `rawInput`, `locations`) from `session/update`, keyed by `toolCallId`, and completes a permission request's missing fields from it before the policy runs (`completePermissionToolCall`). A finished call is forgotten.
- As a second line of defence, `gateFor` reads a kindless call that carries a shell command (`rawInput` string or `{command}`) as `execute`.
- Regression tests:
  - Unit tests and an AIR-shaped fixture test (`ACP_FIXTURE_ANNOUNCE_TOOL_CALL`). It fails without the fix.
  - An opt-in real-adapter test, `driver/test/real-adapter-approval.test.ts`, which runs when `BEX_REAL_CLAUDE_ACP` points at the pinned adapter. It uses a fake Anthropic API and a fake decision gateway. With the fix, `sudo true` opens decision rule `sudo` and is refused, and `touch out.txt` in `all` mode opens `all:execute:touch out.txt`.

**What `all` means with Claude.** Claude Code auto-allows commands it classifies as read-only (`echo hi`, `ls`) before asking any client, so they never reach the gate. That matches D3's "every non-read-only tool call". The production `echo` observation is that classification, not a bypass.

**Codex: gated through its exec policy (w9/081, 2026-11-06).** codex-acp 1.4.0 runs every turn with its session mode's approval policy, and `CODEX_CONFIG` cannot override it (`AgentMode` in `dist/index.js`):

- default `agent` is `on-request` with a workspace-write sandbox;
- `read-only` is `on-request` with a read-only sandbox;
- `agent-full-access` is `never`.

There is no `untrusted` mode, so under `on-request` codex asks only for commands the model escalates out of its own sandbox, and an in-sandbox `sudo true` reached no permission request. codex's exec policy closes the gap: a `prefix_rule(..., decision = "prompt")` makes codex ask before a matching command whatever the turn's policy, and the ask arrives as an ordinary ACP permission request that the driver's gate answers.

- For a non-autonomous codex session the driver writes `$CODEX_HOME/rules/bex-approval.rules` before codex starts (`driver/src/codex-rules.ts`); an autonomous session gets none, and a stale file is removed.
- `risky` prompts every program the approval table inspects (git, sudo, rm, package managers, cluster and cloud CLIs, curl/wget, shells …). The rules are broad by design: the gate still decides each request, so a prompted `git status` is allowed once at once and only a risky command reaches a person.
- `all` prompts every executable on the sandbox's `PATH` except a short read-only list (`ls`, `cat`, `grep`, `echo` …), the codex counterpart of Claude Code's read-only auto-allow.
- Verified against the real codex-acp 1.4.0 and codex 0.147.0 with a fake Responses API:

  - in risky mode, `sudo true` and `/usr/bin/sudo true` open a decision, and a denial reaches codex as "rejected by user";
  - also in risky mode, `echo ok > ok.txt` runs ungated and writes the workspace;
  - in all mode, `touch` opens a decision, and an allow writes the workspace;
  - an autonomous session asks nothing.

  These are the opt-in tests `driver/test/real-codex-approval.test.ts` and `driver/test/codex-rules.test.ts`.

- Limits: a program that is not on `PATH` at session start, or is invoked by a path to a file the session created, is not prompted in `all` mode. In `risky` mode only the listed programs are prompted, which covers every rule in the table.

**Follow-ups:**

- verifying Gemini's `--acp` permission requests.
