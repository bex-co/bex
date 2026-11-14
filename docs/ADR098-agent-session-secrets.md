# ADR098 — Cloud Agent secrets: workspace agent secrets and inline session secrets

**Status:** Accepted, 2026-11-05. Implemented by w9/m174. Amended 2026-11-13 by w9/m385: [D9 — Scopes](#d9--scopes-secrets-that-apply-without-being-named).

**Source:** w9/m174, the top Devin gap in ADR094's 2026-11-02 gap list. A customer has to be able to give an agent session the API keys it needs, for example for integration tests or a third-party API, without putting them in the repo or the prompt.

**Related decisions:**

- [ADR047](ADR047-cloud-coding-agent-sessions.md): sessions, the driver and the sandbox.
- [ADR059](ADR059-agent-sandbox-hibernation.md): every turn runs in a fresh sandbox.
- [ADR062](ADR062-sandbox-credential-vault.md): the model key and the Git credential are placeholder-only inside the sandbox.
- [ADR013](ADR013-secrets.md): OpenBao KV v2 behind `core.SecretKV`.
- [ADR090](ADR090-agent-owned-delivery.md): the prompt overlay.
- [ADR094](ADR094-coding-agent-mcp.md): the `cloud-agents` MCP toolset.
- [ADR095](ADR095-agent-attachments.md): per-dispatch inputs (the attachments precedent).

## Context

Devin's v3 API takes two kinds of secret on session create:

- `secret_ids`, which point to secrets stored for the organization or the user;
- inline `session_secrets[{key, value, sensitive}]`, which exist only for that session.

Both reach the session as environment variables.

ADR062 keeps the model key and the Git credential out of the sandbox, because the platform can do that work for the agent: the gateway swaps in the real model key, and the Git proxy authenticates the push. That cannot work for a customer's own secret. The agent runs `curl -H "Authorization: Bearer $STRIPE_KEY"` or a test suite that reads `DATABASE_URL`, so it needs the actual value. These secrets therefore get the opposite treatment from ADR062's credentials. The value goes into the sandbox on purpose. What the platform guarantees is that the value is stored write-only, stays scoped to the session, and is kept out of everything bex persists or returns.

## Decision

### D1 — Two sources, one env-var delivery

- **Workspace agent secrets** are stored once per workspace and referenced by name: `secretNames: ["STRIPE_KEY"]`. They are Devin's `secret_ids`. bex has no per-user secrets, so the workspace is the store.
- **Inline session secrets** belong to one session: `secrets: [{name, value}]`. They are Devin's `session_secrets`. Every one is sensitive; there is no non-sensitive mode.

Create and steer accept both fields. A session's attached set is the union of every name attached so far, kept in `agent_sessions.secret_names`. Steer can add names to the set but cannot remove them. If the same name is both an inline secret and a workspace reference, the inline value wins.

### D2 — Storage

All values live in OpenBao, through the same `core.SecretKV` that holds the BYO model key (`agent-sessions/<ws>/model-key`):

| Secret | Path | Data |
| --- | --- | --- |
| Workspace agent secret | `agent-sessions/<ws>/secrets/<NAME>` | `{value}` |
| Inline session secrets | `agent-sessions/<ws>/sessions/<ags-id>/secrets` | `{NAME: value, …}` |

Each workspace secret has its own path, so `List` returns the names without reading any value. Postgres holds names only, in `agent_sessions.secret_names`. No value or hash of a value goes into Postgres. The idempotency digest covers the names but not the values.

### D3 — Naming and limits

- **Names.** A name must match `^[A-Za-z_][A-Za-z0-9_]{0,127}$`. That is stricter than the Secret-key rule in `core.ValidEnvKey`, because the value has to work as a shell variable and the name travels in a comma-separated list.
- **Reserved prefixes.** These are refused because the platform owns them, and setting one would reroute the model or break the driver: `BEX_`, `ANTHROPIC_`, `OPENAI_`, `CLAUDE_`, `CODEX_`, `GEMINI_`, `OPENSANDBOX_`, `GIT_`.
- **Reserved names.** These are refused for the same reason: `GOOGLE_API_KEY`, `GOOGLE_GEMINI_BASE_URL`, `PATH`, `HOME`, `USER`, `SHELL`, `PWD`, `TERM`, `LANG`, `HOSTNAME`, `NODE_OPTIONS`, `LD_PRELOAD`, `LD_LIBRARY_PATH`, the proxy variables in both cases (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, `ALL_PROXY`), and `SSL_CERT_FILE`.
- **Values.** A value must be 1 B to 32 KiB, with no NUL.
- **Counts and sizes.** A workspace can hold at most 50 agent secrets. A session can have at most 20 attached names, and the values together can total at most 128 KiB.

A refusal names the field and the rule. It never includes the value.

### D4 — Write-only API

Every read returns names only. No REST, GraphQL or MCP read, view, log line or error ever carries a value. Changing a value means setting it again.

| Verb | REST | GraphQL | MCP | Relation |
| --- | --- | --- | --- | --- |
| List workspace secrets | `GET /v1/agent-secrets?ownerId=` | `agentSecrets(ownerId)` | `list_agent_secrets` | `can_create` |
| Set (create or rotate) | `PUT /v1/agent-secrets/{name}?ownerId=` `{value}` | `setAgentSecret` | `set_agent_secret` | `can_manage` |
| Delete | `DELETE /v1/agent-secrets/{name}?ownerId=` | `deleteAgentSecret` | `delete_agent_secret` | `can_manage` |
| Attach on create | `secretNames`, `secrets` | same | same | `can_create` |
| Attach on steer | `secretNames`, `secrets` | same | same | `can_create` on the session |

The three MCP tools are also in the `cloud-agents` toolset (ADR094). A session view shows the attached names as `secretNames`.

Developers can list the names, because they need to know what they can attach. Only admins can set or delete a workspace secret. Attaching a secret is a `can_create` action, the same as starting the session it goes into.

### D5 — Delivery at dispatch

Every dispatch rebuilds the session's secret env (ADR059): create, steer and recovery alike. `dispatch` takes each name in `secret_names` and reads its value from the session document first, then from the workspace secret. It adds each value to the sandbox env under its own name, and adds `BEX_AGENT_SECRET_NAMES`, a comma-separated list of the names.

If a workspace secret was deleted after it was attached, the dispatch fails with `AGENT_SESSION_SECRET_UNAVAILABLE` rather than running the agent without it. Create and steer check that each referenced name exists before they accept the request; a missing one is refused with `AGENT_SECRET_NOT_FOUND`.

The driver spawns the agent with its own env (`acp.ts`), so the agent and every tool it runs see the variables. The prompt overlay is unchanged. Secret names are not stated to the agent beyond what the user wrote in the task.

**Accepted tradeoff:** the values sit in the sandbox pod's env for the pod's life, which is one turn. They take the same path `BEX_AGENT_PROMPT` already takes. Anyone with Kubernetes read access to the tenant namespace can read them, which is the operator trust boundary already established in ADR043. They are never stamped into sandbox metadata, labels or audit.

### D6 — Redaction before persistence

The driver redacts at its single publication choke point, before a part reaches the live stream, the gateway's durable transcript tee or the turn log:

- **Which values.** The driver reads the names from `BEX_AGENT_SECRET_NAMES` and their values from its own env.
- **What it replaces.** In every string field of a part, recursively, it replaces each value with `[redacted:NAME]`. It also replaces the value's standard base64, unpadded base64, base64url, URL encoding and JSON-escaped forms.
- **Text and reasoning deltas.** A value can be split across two deltas. The driver holds back any tail that could be the start of a value, and releases it at the block's end.
- **Failure messages.** The failure message in the status file is redacted too.
- **Short values.** A value shorter than 4 characters is not redacted, because scrubbing every `abc` would destroy the transcript and protect nothing.

Redaction is best-effort against an agent that transforms a value in some other way, for example by printing it reversed. The guarantee covers the value as the agent received it and its cheap encodings. bex-api never logs a value: dispatch logs record the session id and the failure code only.

### D7 — Lifetime

- **Inline secrets** live until the session is deleted. Delete removes the session document after the row is gone. Archiving keeps the document, because an unarchived session can be steered again.
- **Workspace secrets** last until an admin deletes them. Deleting a workspace secret does not change a session's `secret_names`. The session's next dispatch fails with `AGENT_SESSION_SECRET_UNAVAILABLE`, so a revoked secret is never silently used and never silently dropped.

### D8 — Dashboard

- **New-session composer.** A Secrets control lets the user pick workspace agent secrets and add inline ones. Inline values go in masked inputs and are never echoed back.
- **Settings.** The agent settings page has a Workspace secrets section. It lists names, sets values from a masked input and deletes secrets. Only admins see the set and delete actions.
- **Session details.** The session view lists the attached names.

### D9 — Scopes: secrets that apply without being named

_Added 2026-11-13 by w9/m385._

Devin's organization secrets "apply to the entire organization" and reach every future session; its repository secrets are available "when it works in that repository" ([Secrets](https://docs.devin.ai/product-guides/secrets), checked 2026-11-13). D1 gave bex only the named kind, so a session someone started without listing names (an automation, a `/bex` comment, a PR-feedback steer, a child) received nothing. Each workspace agent secret now has a **scope**:

| Scope | Reaches |
| --- | --- |
| `workspace` | every session in the workspace |
| `repo` | every session whose repository is the scope's `repo` (`owner/name`, lowercased like a session's repository) |
| `explicit` | only sessions that name it in `secretNames` (D1's behavior) |

- **Every origin.** Scopes are resolved at every dispatch (D5), so they reach API and dashboard sessions, steers, automation runs, the `/bex` takeover, PR-feedback steers and children alike, and a secret added or rescoped now reaches every session's next turn. Scoped names are not written into `secret_names`; that column stays the set a caller attached.
- **Precedence.** A name is unique in the workspace, so a scope decides only whether a secret applies. A name the session attached keeps its own source, and an inline value still wins over a workspace secret of the same name (D1).
- **Redaction.** Every applied name, scoped or named, joins `BEX_AGENT_SECRET_NAMES`, so D6 scrubs its value.
- **A deleted scoped secret** is simply not applied. Only a name a session attached fails its dispatch when it disappears (D7).
- **Limits.** A scope that would push the secrets one session receives without naming them (every workspace-scoped value plus the largest repository's repo-scoped values) past D3's 128 KiB is refused `AGENT_SECRET_LIMIT_EXCEEDED`. A malformed scope or repository is refused `AGENT_SECRET_SCOPE_INVALID`.
- **Storage.** Values stay in OpenBao (D2). The scope lives in Postgres `agent_secret_scopes (workspace_id, name, scope, repo)`, so a list reads names and scopes without reading values. A secret with no row is `explicit`.
- **Defaults.** A new secret without a scope is `workspace` (Devin). Secrets created before this amendment have no row and read `explicit`, so nothing stored under "only sessions that name it" reaches more sessions without an admin acting. That is a divergence from Devin for pre-existing secrets, recorded as the brainstorm's recommendation and open for the user's sign-off; making them `workspace` instead is a one-line data change.
- **What applied.** Each dispatch records the names it applied with their source (`inline`, `explicit`, `repo`, `workspace`) in `agent_sessions.applied_secrets`; every session read shows them as `appliedSecrets`. Names only.
- **Surfaces.** REST `PUT /v1/agent-secrets/{name}` takes `{value?, scope?, repo?}` (omit `value` to rescope an existing secret) and reads return `{name, scope, repo?}`; GraphQL `setAgentSecret(…, scope, repo)` and `AgentSecret.scope`/`repo`; MCP `set_agent_secret` and `list_agent_secrets` the same; `bex agent secrets set --scope --repo`; the dashboard's Workspace secrets section sets the scope and the composer shows scoped secrets as included.
- **Not adopted:** Devin's personal secrets (bex has no per-user secrets, D1).

## Consequences

- An agent can run integration tests and call third-party APIs with real credentials, without the values ever going into the repo or the prompt.
- Values exist in three places: OpenBao, the sandbox env for one turn, and the agent's own memory and process. They are not in Postgres, the transcript, the turn log, logs or any API read.
- Leaving out the values from the idempotency digest means a retried create with the same names but a different value replays the first session. That matches what the key is for.
- Live check (DoD): a session given a secret runs `curl` against a test endpoint with `$NAME`. The transcript and the tool output show `[redacted:NAME]`, and no read returns the value.
