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
