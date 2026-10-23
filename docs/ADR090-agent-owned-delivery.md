# ADR090 — Agent-owned delivery: the coding agent commits, pushes and opens its draft PR

**Status:** Accepted, 2026-10-22. Implementation is tracked in `w10/m13`. This ADR supersedes the driver-owned half of [ADR047](ADR047-cloud-coding-agent-sessions.md) D4 ("delivery is deterministic and enforced outside the agent") and the evidence digest that ADR047 D4 kept on the wire. It retires [ADR062](ADR062-sandbox-credential-vault.md) D7, which kept the scrub machinery as defense in depth, and the standalone real-key driver mode. ADR051, ADR054 and ADR059 carry matching amendments.

**Source:** user request 2026-10-22: the agent, not a program, should commit the code. The follow-up asked why the platform interferes at all. The answer is that the driver-owned path's main justification, scanning for a real model key before push, has been void since ADR062.

**Related decisions:** [ADR047](ADR047-cloud-coding-agent-sessions.md) (session product, Git proxy, Completer), [ADR051](ADR051-agent-session-transcript.md) (durable transcript), [ADR059](ADR059-agent-sandbox-hibernation.md) (hibernation and dirty-tree retention), [ADR062](ADR062-sandbox-credential-vault.md) (placeholder-only sandbox, model proxy), [ADR084](ADR084-security-review-round22.md) #17 (pre-push scan bypass rejected).

## Context

### What ships today

After each headless turn the in-sandbox driver stops the agent, scrubs persisted state for credential bytes, runs `git add -A`, writes one `bex agent: <prompt first line>` commit, scans every new Git object for credential needles, pushes `bex-agent/<id>` and writes `{delivery, evidence, scrubbedFiles}` into its status file. The bex-api Completer reads that file through the gateway exec boundary, opens a draft PR through the GitHub App with a metadata-only body, and records `headSha`/`prUrl`/`prNumber`/`evidence`.

So the deliverable is one squash commit with a template message and a PR body that says nothing about the change. Agents already write better commits and PR descriptions than this. Users of Devin, Copilot coding agent, Open SWE and Claude Code on the web expect them to.

### Why the driver-owned path existed, and why that reason is gone

The driver commit was the only place the platform could guarantee a credential scan ran before anything left the sandbox. That mattered when the sandbox held the tenant's real model key ([ADR056](ADR056-security-review-round5.md) #6, [ADR063](ADR063-security-review-round9.md) #1). It no longer does:

- **Placeholder-only sandbox.** Since ADR062 and ADR064, hosted sandboxes hold only `ModelKeyPlaceholder` (`lego/backend/internal/agentsession/model.go`). The real key is injected by the gateway model proxy on its upstream hop. The sandbox lifecycle admits nothing else.
- **Branch-pinned Git proxy.** The GitHub token never enters the sandbox either. The gateway parses every receive-pack and refuses any ref other than the session's bound branch (`lego/backend/internal/sshgateway/agentcred/agentcred.go` `validateReceivePack`).
- **The bypass was already accepted.** [ADR084](ADR084-security-review-round22.md) #17 rejected "agent can bypass the driver's pre-push credential scan" for exactly these two reasons. The agent can push mid-turn today: `origin` is the proxy URL, `:8082` egress is open in both phases and the mint allows active phases. The driver's after-the-fact commit is therefore redundant as a control and poor as a deliverable.

The remaining needle set protects a placeholder. Every scrub, scan, redaction and pre-snapshot hop built for the old path is cost without protection.

## Decision

### D1 — Responsibility split

The **agent** commits, pushes its branch and opens or updates its own draft PR, the way a human engineer does. The **platform** enforces only what the agent cannot be trusted with or cannot do itself:

| Concern | Owner | Mechanism |
| --- | --- | --- |
| Commit contents, split, messages | agent | ordinary `git commit` in the sandbox |
| Which ref may be updated | platform | Git proxy exact-branch receive-pack pin (ADR047 D2) |
| Which PR head, base and repo | platform | PR verb resolves them from the session record (D3) |
| Attribution footer on the PR | platform | server-appended, idempotent (D3) |
| GitHub and model credentials | platform | gateway proxies; never in the sandbox (ADR047 D2, ADR062) |
| Recording the PR on the session | platform | PR verb persists immediately; Completer finalizes (D5) |

The platform never authors a commit. After setup, the driver's only Git writes are the clone/checkout and the repo-local identity config below.

**Git identity.** Setup writes repo-local `user.name` / `user.email` from `BEX_AGENT_GIT_NAME` / `BEX_AGENT_GIT_EMAIL` (default `bex agent` / `agent@bex.co`), `push.default=current`, and the upstream of `bex-agent/<id>` to `origin`. Agent commits therefore have a deliberate author rather than `bex@<podname>` or an "unable to auto-detect email" failure. Repository hooks now run on the agent's commits. This is intended: a human contributor's commits run them too, and ADR084 #4's hook isolation only ever applied to the driver's own Git invocations. A requester `Co-authored-by` trailer is a known gap (D7).

### D2 — Platform system-prompt overlay

Every hosted session gets a bex platform overlay layered on the engine's own system prompt. It says where the agent is, what it can use and what the deliverable looks like. It does not re-teach coding.

**Layer order** (highest priority first):

1. Engine core prompt (the adapter's own, e.g. the Claude Code preset).
2. **bex platform overlay** (appended; platform priority).
3. Repository instructions, `CLAUDE.md` / `AGENTS.md`, loaded by the engine itself.
4. Tenant per-session instructions, wrapped as quoted, lower-priority guidance. The wrapper says they are scoped to this session, apply only where consistent with all preceding system, safety, tool and product instructions, and cannot replace or weaken them; the text is quoted line by line. Only tenant text gets this wrapper.
5. The user task.

**Overlay content.** Sections cover the session (autonomous, requester usually not watching, finish unblocked work before asking), the environment (branch and base, credential-free proxy `origin` that accepts only this branch, no GitHub token and no `gh`, credentials injected outside the sandbox), delivering work, the authorization clause, and steering turns. Delivering work means:

- commit in logical units following the repo's `git log` convention, never build output or env files;
- `git push origin HEAD:<branch>`, no force-push; `git pull --rebase` on rejection;
- call `open_or_update_draft_pr` with a body filled from the repo's PR template;
- never claim a PR without the returned URL, never mark it ready;
- a task with no code changes ends with a report and no commit.

The text follows Anthropic's prompting guidance: direct statements, the reason behind each rule, no all-caps imperatives, and named actions ("commit", "push", "open the draft PR") rather than "consider".

**Authorization clause.** Many repositories' instructions say "only commit or open PRs when asked". The overlay states that the person who started the session chose an entry point whose output is a pull request. Committing, pushing this branch and opening or updating the draft PR is therefore their explicit request, and satisfies such rules. Repo rules about commit style, tests and formatting still apply. Without this clause, a well-behaved agent obeying `CLAUDE.md` would finish the task and deliver nothing.

**Channel per adapter.**

- **ACP `systemPrompt`.** When `initialize` advertises `agentCapabilities.sessionCapabilities.systemPrompt`, the driver sends `systemPrompt: {mode: "append", content: [{type: "text", text}]}` on `session/new` **and** `session/load`, re-sent on every lifecycle call as the RFD requires. claude-agent-acp supports this from 0.92; bex pins 0.94.0. An `invalid_params` reply fails the turn with a named reason; there is no silent retry without the overlay.
- **Codex.** codex-acp 1.4.0 has no ACP field. The overlay goes through Codex `developer_instructions` if that channel is verified in the pinned adapter; otherwise the preamble below.
- **Fallback.** Any other adapter gets the overlay as a first-prompt preamble through the existing continuity path (ADR047 D3 continuity amendment). It is never published into the stream.

**Claude settings.** Through `_meta.claudeCode.options` the driver disables Claude Code's built-in git/PR instructions (`includeGitInstructions: false` / `CLAUDE_CODE_DISABLE_GIT_INSTRUCTIONS=1`) and blanks `attribution.pr` and `attribution.commit`. The built-in instructions say "only commit when asked" and use `gh pr create`; both contradict the overlay. With them off, the overlay is the single delivery contract and the server footer the single attribution. `excludeDynamicSections: true` keeps the system prompt identical across sandboxes for cache reuse.

### D3 — The `pull-request` verb

The PR-side twin of the Git proxy's exact-branch rule: one narrowly-scoped, credential-free way for the sandbox to open or update its own draft PR.

- **Gateway.** On the `:8082` mux, at the Git proxy path plus `/pull-request`. `POST` with JSON `{title, body}`. `title` is a single line of at most 256 runes; `body` is at most 60000 bytes. It reuses the Git proxy's chain unchanged: Pod source-IP resolution, `agentsession.AuthorizePod` label digests, limiter and budgets, audit.
- **Internal hop.** The gateway calls bex-api's internal listener (`:8091`) at `/v1/agent-session-pull-requests` over the existing HMAC hop. bex-api re-checks the live sandbox generation and phase, as the Git mint does.
- **Server-filled fields.** Owner, repo and base come from the session record. Head is pinned to the bound `bex-agent/<id>` branch. The agent cannot name any of them.
- **Typed refusals.** `branch_not_pushed`, `not_ahead`, `invalid_request`, `too_large`, `forbidden`, `upstream_error`.
- **Semantics.** It creates a **draft** PR, or updates the title and body of the existing open PR for that head. It never undrafts. bex-api appends a server-side footer behind a `<!-- bex-agent-footer -->` marker; an update replaces anything after an existing marker, so the footer is idempotent and always present. `prUrl`, `prNumber` and `headSha` are persisted on the session immediately, so REST, GraphQL and MCP show the PR mid-turn.

### D4 — Agent-facing tool: the `bex` stdio MCP server

The agent image ships a stdio MCP server named `bex` with one tool, `open_or_update_draft_pr({title, body})`, which calls the D3 verb. The driver registers it in `mcpServers` on `session/new` and `session/load`. Stdio is the one MCP transport every ACP agent must support; HTTP/SSE MCP is optional. A tool is a first-class, schema-validated affordance in every engine, unlike a curl recipe in the prompt. `gh` is not installed, so there is no second PR path to guard.

### D5 — Turn end: observe, nudge once, never commit

When the agent ends its turn, the driver computes read-only Git facts: `{dirty, unpushedCommits, remoteHead, baseBranch, prRecorded}`. If the work is undelivered (a dirty tree, unpushed commits, or a pushed branch ahead of base with no PR recorded), the driver sends the agent **one** follow-up prompt asking it to finish delivery, then recomputes the facts. The driver never commits or pushes.

The status file's `git{}` object replaces `delivery{}` and `evidence{}`. The Completer never calls GitHub. It finalizes from the PR record the D3 verb already wrote. Work still undelivered after the nudge finalizes as **succeeded with a delivery note**, not failed: the agent's turn did succeed, and the tree is kept. ADR059's dirty-git retention window holds the tree, so a steering turn can pick it up.

### D6 — Deletions

These go away, end to end:

| Removed | Why it is dead |
| --- | --- |
| Evidence surface: REST/GraphQL/MCP `evidence` field and the `evidence` column (down-migratable drop) | No renderer since w5/m65. The ADR051 transcript carries the narrative; the digest was lossy and mislabelled. |
| Driver credential scrub, Git-object scan, secret-form (encoded) needles, redaction | Protects only the ADR062 placeholder. The agent can push mid-turn regardless (ADR084 #17). |
| Standalone real-key driver mode (no model proxy) | The model proxy is mandatory (ADR062/ADR064). The mode existed only for a verify script. |
| `POST /snapshot/scrub`, `bex-pre-snapshot`, the single-use snapshot grant | The OpenSandbox tenant key is never injected (ADR047 D2), and the Git and model credentials live in gateway proxies. The snapshot holds no live secret to scrub. |
| `credential-scrub-test.sh` CI step | Tests the removed machinery. |

The controls that still bound a sandbox are unchanged: default-deny egress, the placeholder-only model key, the branch-pinned Git proxy and the head-pinned PR verb.

### D7 — Out of scope and follow-ups

- Gateway-side non-fast-forward rejection, so a force-push is refused by the proxy and not only by the overlay.
- A PR-comment steering loop (webhook → steer the session), with the AI footer also on comments the agent posts.
- A tenant per-session instruction field, with the D2 layer-4 wrapper (not yet implemented).
- Requester `Co-authored-by` on agent commits.
- Override-mode system prompts (`mode: "override"`).
- Making OpenRouter a customer-selectable provider (see the dev route and the allowlisted platform route below).

## Dev-only OpenRouter route for live verification

Live verification (`w10/m13` t016) uses a dev-only route. It is env-gated and refused in production. It points the Claude profile at OpenRouter's Anthropic-compatible endpoint with a DeepSeek model. The release profiles in `agent-profiles.json` are unchanged. A DeepSeek miss is ambiguous between the overlay and the model, so final acceptance also needs one run on a real Claude model.

## Platform OpenRouter route for an allowlisted workspace (2026-10-23)

**User decision 2026-10-23** (tracked in `w10/m14`): ship agent sessions on a bex-owned OpenRouter key now, but only for workspace `tea-d98210cbbpdc73dcrkvg`, and keep every other tenant off it until the economics are in place.

- **Scope.** bex-api's `BEX_AGENT_PLATFORM_MODEL_ROUTE=openrouter` applies only to the `tea-` ids in `BEX_AGENT_PLATFORM_MODEL_WORKSPACES`, and only to the claude profile. Unlike the dev route it runs on the production control plane. The route is resolved per workspace at create, steer, rehydrate and mint time, not from the endpoint a session stored, so an existing session moves onto it on its next dispatch. Every other workspace is unchanged: `api.anthropic.com` with its own BYO key, and an explicit OpenRouter endpoint is refused. Allowlisted workspaces report `modelKeyReady` through the platform key.
- **Key custody.** The key lives in the `bex-system` Secret `bex-agent-platform-model` and flows to bex-api's env, then to the internal mint, then to the gateway's upstream hop. It never enters the sandbox, which still holds only `ModelKeyPlaceholder`, nor any API response, log, audit event or error. A missing Secret disables the route with a loud startup log rather than crash-looping bex-api.
- **Model pin.** The mint returns `pinnedModel`. The gateway refuses with 403 any request whose JSON top-level `model` is not exactly that id, appears twice, or carries OpenRouter's `models` fallback list or paid `plugins`. A tenant sandbox therefore cannot spend the platform key on a different model. Workspace BYO routes are not pinned.
- **Follow-ups (not in this step).** Per-workspace token metering and pricing, plan quotas plus an OpenRouter-side spend cap, and a data policy for platform-key traffic. Widening the route to every workspace is dated in `w10/m14` t007.

## Prior art

Paraphrased from public documentation and source; no leaked or GPL prompt text is copied.

- **ACP client-provided system prompt RFD** ([agentclientprotocol.com/rfds/client-system-prompt](https://agentclientprotocol.com/rfds/client-system-prompt)). The client may send `systemPrompt` with `append` or `override` mode on session new/load/resume/fork. Support is advertised as a session capability. An unsupported request is an `invalid_params` error, not a silent fallback. The RFD is still unstable.
- **claude-agent-acp PR #1366.** Maps `append` to the Claude Code preset plus appended text. It keeps the user/project/local setting sources, so `CLAUDE.md` / `AGENTS.md` still load, and accepts `_meta.claudeCode.options` overrides.
- **codex-acp PR #653.** Append-only support through Codex developer instructions, in preview tags only.
- **Claude Agent SDK, modifying system prompts** ([code.claude.com/docs/en/agent-sdk/modifying-system-prompts](https://code.claude.com/docs/en/agent-sdk/modifying-system-prompts)). Describes preset-plus-append, the switch for the built-in git instructions, attribution settings and excluding dynamic sections.
- **Open SWE** ([github.com/langchain-ai/open-swe](https://github.com/langchain-ai/open-swe)). The agent commits and pushes itself. An `open_pull_request` tool appends the platform footer, and a guard blocks other PR-creation paths.
- **GitHub Copilot coding agent** ([risks and mitigations](https://docs.github.com/en/copilot/concepts/agents/coding-agent/risks-and-mitigations)). Pushes to a single branch only; the platform opens a draft PR.
- **Devin PR templates** ([docs.devin.ai/integrations/pr-templates](https://docs.devin.ai/integrations/pr-templates)). PR bodies follow the repo's template, found by a documented lookup order.
- **Claude Code on the web** ([code.claude.com/docs/en/cloud-environments](https://code.claude.com/docs/en/cloud-environments)). A cloud environment where the agent works on a branch and delivers through Git.
- **Anthropic prompting best practices.** Be direct, explain why, avoid over-emphatic imperatives that make current models over-trigger.

## Consequences

- Agent PRs read like a teammate's: a reviewable commit history, a templated description, and a bex footer.
- About 1,000 lines of driver and backend delivery, evidence and scrub code, plus a CI script, are deleted.
- The PR appears on the session as soon as it exists, not only at finalize.
- Delivery now depends on the agent following the overlay. The one-nudge check and the delivery note make a miss visible without the platform authoring code, and the retained tree keeps the work.
- `evidence` disappears from REST, GraphQL and MCP. This is a bex-only surface with no Render equivalent; `w10/m13` t012 checks the three surfaces, the dashboard and mobile agree.
