# ADR047 — Managed cloud coding-agent sessions

**Status:** Accepted; D3 agent-session control-plane API shipped in w3/m39 (2026-08-01). **D4 delivery (draft PR + evidence) and D8 phase-1 steering shipped in w3/m41 (2026-08-02):** the sandbox driver commits + pushes the `bex-agent/*` branch and captures bounded evidence; a bex-api background Completer reads the driver status file through the gateway exec boundary, opens a draft PR via the GitHub App, and records head SHA + PR URL + evidence on the session; a steering turn re-dispatches a fresh sandbox on the same branch and updates the same PR. The gateway attach proxy/transcript path (live attach, token metering) remains phase 2. Deep-research and the in-sandbox AI SDK driver amendment were completed 2026-08-01. **D9 dashboard-surface design added 2026-08-02** (frontend deep-research pass + target-API-shape decision the same day; the conversation API materialized as `w3/m43`, the dashboard consumer as `w1/m64` — the earlier interim polling-synthesized-timeline plan was discarded before build). **D9 conversation API implemented in w3/m43 (2026-08-02):** durable transcript store, gateway attach listener (verbatim SSE replay + live splice + tee, driver-direct dial), driver `POST /turn`, `attach-ticket` verb (3-surface), sandbox gateway-ingress policy, and `api.bex.co` edge path-routing — backend/gateway/driver suites + real-Postgres + lint green; the live-substrate E2E leg shares the m41 operator-run gate. Engages the `.pm/DO_NOT_DO.md` "Hosted Claude Code inside sandbox" deferral — this ADR re-opens that item as a phased product.

---

## Context

### The product

"Code on the tenant's GitHub repo, in our cloud": the tenant assigns a task (dashboard, IDE, or MCP), bex runs a coding agent in an isolated sandbox with the repo checked out, and the result comes back as a draft PR — the shape of GitHub Copilot cloud agent, OpenAI Codex cloud, Devin, Claude Code on the web, and Cursor cloud agents. This is pillar 5 territory (ADR008): the sandbox substrate exists (ADR042); this ADR is the product layer on top of it.

### What the exemplars verifiably do

An adversarially verified research pass (2026-08-01; 25 sources fetched, 21 claims confirmed, 4 refuted) found the exemplars converge on one architecture. The load-bearing findings, all 3-0 verified against primary docs:

- **Ephemeral sandbox per task, repo preloaded.** Copilot cloud agent runs in "its own ephemeral development environment, powered by GitHub Actions" ([GitHub docs](https://docs.github.com/copilot/concepts/agents/coding-agent/about-coding-agent)); Codex runs each task in an isolated cloud container checked out at the selected branch, where it edits files and runs tests/linters ([OpenAI](https://openai.com/index/introducing-codex/)), with a 12h container-state cache for follow-up turns.
- **Egress default-deny, opt-in internet.** Codex launched with agent-phase internet disabled (setup scripts run _with_ network to install dependencies; the agent phase then runs without), later adding opt-in domain allowlists ([Codex internet-access docs](https://developers.openai.com/codex/cloud/internet-access)). Copilot ships a customizable [agent firewall](https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/customize-cloud-agent/customize-the-agent-firewall). Network egress control — not in-sandbox policy — is the primary prompt-injection/exfiltration mitigation.
- **Git-native delivery, in-band steering.** Copilot: exactly one branch and one PR per task, steered by `@copilot` PR comments. Codex: commits inside the sandbox with citations of terminal logs and test outputs as verifiable evidence; the user promotes to a PR. Codex shipped _without_ mid-task steering and had to add it — a steering channel is table stakes.
- **Per-request billing is dead.** GitHub retired premium requests effective 2026-06-01, replacing them with token-passthrough "AI credits" (1 credit = $0.01, metered on input/output/cached tokens at published per-model rates), explicitly because "a quick chat question and a multi-hour autonomous coding session can cost the user the same amount" ([GitHub blog](https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/)). GitHub now dual-meters agentic features: tokens (AI credits) **plus** compute (Actions minutes) ([pricing docs](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing)). Devin (ACU quota + overage, [billing docs](https://docs.devin.ai/admin/billing)) and Cursor (plan-included usage pool, arrears overage, [pricing](https://cursor.com/pricing)) fit the same subscription-plus-metered pattern. AWS AgentCore Runtime prices the compute leg per-second on actual CPU ($0.0895/vCPU-hr) + peak memory ($0.00945/GB-hr) with idle CPU-free ([pricing](https://aws.amazon.com/bedrock/agentcore/pricing/)).

Research caveats: no claims about the _internal_ architecture of Devin, Claude Code on the web, or Cursor cloud agents survived verification (only their billing did), so the architectural convergence rests on Copilot + Codex evidence. The ACP adopter ecosystem (below) was fetched but not adversarially verified. Claude Code resale/ToS constraints produced no surviving claims and remain an open question.

### ACP as the interface

The [Agent Client Protocol](https://agentclientprotocol.com/protocol/overview) (Zed-originated) is JSON-RPC 2.0 — request/response methods plus one-way notifications — covering sessions, prompt turns, tool-call streaming, permission requests, file ops, and terminals. Verified transport state ([spec](https://agentclientprotocol.com/protocol/transports)): stdio is the transport agents SHOULD support; a Streamable HTTP transport is a **draft**; custom transports MAY be implemented — so a WebSocket carrying newline-delimited JSON-RPC is architecturally sanctioned but not standardized, and **auth, reconnection, and multi-client attach are left to the implementation**. Session resumption (`session/load`) is an **optional** agent capability: clients MUST check `loadSession` in the `initialize` response and MUST NOT call `session/load` if absent ([session setup](https://agentclientprotocol.com/protocol/session-setup)). Ecosystem signals (Claude Code via ACP in [Zed](https://zed.dev/blog/claude-code-via-acp), Gemini CLI, [OpenHands](https://www.openhands.dev/blog/use-any-coding-agent-in-openhands-with-acp), community bridges like [acp-ws-bridge](https://github.com/ytthuan/acp-ws-bridge)) confirm the bridging pattern is being pursued; verify per-agent maturity before committing to a binary. Notably, an ACP provider for the Vercel AI SDK ([`@mcpc-tech/acp-ai-provider`](https://github.com/mcpc-tech/mcpc/tree/main/packages/acp-ai-provider), listed in the [AI SDK community-provider directory](https://ai-sdk.dev/providers/community-providers/acp)) adapts spawned ACP agents to `LanguageModelV3`/`streamText` — stdio-spawn only, stateful sessions via `persistSession`/`existingSessionId` (→ `session/load`), no token-usage reporting; D3 builds on it.

### What bex already owns

Every exemplar capability decomposes onto shipped primitives:

| Primitive | Where | State |
| --- | --- | --- |
| Sandbox substrate: create/list/get/suspend/resume/terminate, per-workspace `<ws>-sandbox` namespaces, per-workspace OpenSandbox tenant keys | ADR042/ADR043; `lego/backend/internal/sandbox/client.go` | shipped (w3/m32) — **gVisor** (`runsc`/systrap) on the tainted `bex.co/sandbox` pool; Kata remains a future candidate pending KVM nodes (ADR042's "Kata/microVM" reads aspirationally) |
| Exec brokering: bex-api authorizes + HMAC-signs, gateway alone holds `pods/exec` | one-shot SSE: `internal/sandboxexec/ticket.go`, `internal/sshgateway/sandboxsse` (w3/m33); long-lived WebSocket + DB-backed nonces: web shell (w2/m55, ADR035) | shipped — the web-shell path is the precedent for long-lived agent sessions |
| GitHub App: installation tokens (1h TTL, `contents:read` + `metadata:read`) minted per deploy, managed clone Secrets, push webhook | ADR026; `internal/github/client.go`, `internal/deploys/service.go` | shipped (w2/m8–m9) |
| Agent-client auth: OAuth 2.1 + DCR + PKCE at Hydra, per-verb OpenFGA checks | ADR012 §7, ADR025 | shipped (w4/m9) |
| Metering → billing: `usage_hourly` per-meter cursors → sealed-row outbox → Stripe meter events, paid-intent gate | ADR023/ADR040/ADR046 | shipped — **sandboxes are the one resource not yet metered** |

Known substrate defect: **suspend is currently broken in production** — OpenSandbox's snapshot-commit Job mounts `hostPath`, forbidden under the tenant-namespace PSS baseline; the fix (run the Job outside the tenant namespace) is upstream work (ADR042 D5 notes). Memory snapshots (CRIU) remain a watch item; snapshots are rootfs-only.

---

## Decision

### Session architecture

```mermaid
flowchart TB
  dev@{ shape: tri, label: "developer" }

  subgraph clients["tenant clients"]
    dash["dashboard (TanStack, useChat)"]
    ide["IDE / editor (native ACP client)"]
  end

  stripe["Stripe"]
  gh["GitHub"]

  subgraph cluster["bex cluster"]
    api["bex-api"]
    fga["OpenFGA"]
    cpdb[("control-plane Postgres: sessions, transcripts, usage_hourly, ticket nonces")]
    oss["OpenSandbox server (create / hibernate / resume, rootfs snapshots)"]
    subgraph gwz["isolated ssh-gateway — sole session ingress, sole holder of pods/exec"]
      gw["agent-session proxy (SSE + WebSocket)"]
    end
    subgraph sbns["&lt;ws&gt;-sandbox namespace — gVisor, default-deny (ingress: gateway only)"]
      driver["session driver (AI SDK + acp-ai-provider)"]
      agent["agent binary (ACP server, spawned child)"]
      helper["git credential helper (no token on disk)"]
    end
  end

  dev --> dash
  dev --> ide
  dash -->|"POST /v1/agent-sessions"| api
  dash -->|"SSE: AI SDK UI-message stream + HMAC ticket"| gw
  ide -->|"WebSocket: raw ACP JSON-RPC + HMAC ticket"| gw
  api -->|"authorize can_operate"| fga
  api --> cpdb
  api -->|"meter events"| stripe
  api -->|"mint installation token, open draft PR"| gh
  api -->|"create / hibernate / resume"| oss
  oss --> sbns
  gw -->|"nonce claim + transcript tee"| cpdb
  gw -->|"proxy both streams"| driver
  gw -->|"proxied token mint"| api
  driver -->|"stdio (ACP JSON-RPC)"| agent
  agent --> helper
  helper -->|"token refresh"| gw
  agent -->|"clone + push bex-agent/* branch"| gh
```

### D1 — Sandbox-per-session on the existing substrate

One OpenSandbox sandbox per agent session in the workspace's `<ws>-sandbox` namespace, from an agent template image (agent binary + ACP shim + git + language toolchains), created through the existing lifecycle client with the same reserved-metadata stamping (`bex.co/owner`, `bex.co/workspace`, `app.bex.co/regime=sandbox`). Setup phase (clone + dependency install, network open per D5) is separated from the agent phase, Codex-style. Idle sessions hibernate via rootfs snapshot; resume restores the rootfs and restarts the agent process (see D3 for conversation state). No new substrate: kubernetes-sigs/agent-sandbox and similar were considered and rejected — they would rebuild w3/m32 for no capability gain.

### D2 — Repo access: per-session installation token, gateway-refreshed, branch-confined

- bex-api mints a **per-session** GitHub App installation token from the existing ADR026 integration, scoped to the target repo, `contents:write` added (sessions must push).
- The token never lands on disk in the sandbox. A **git credential helper** inside the sandbox fetches it on demand through a gateway internal endpoint, which proxies an authorized re-mint to bex-api — solving the 1h-token-TTL vs multi-hour-session mismatch without giving the sandbox standing credentials. (The operator's deliberate never-refresh stance for build clones does not transfer: builds are minutes, sessions are hours.)
- **Branch confinement is server-side**: bex-api only mints push-capable tokens for `bex-agent/*` session branches and recommends tenant branch protection on default branches; the in-sandbox agent is never the enforcement point (Copilot's one-branch/one-PR model, enforced outside the agent).
- **Snapshot hygiene**: credentials (and the OpenSandbox tenant key) are scrubbed before any rootfs snapshot; a resumed sandbox re-fetches through the helper.

Implementation contract (w3/m38): OpenSandbox session metadata carries only the session id plus SHA-256 label digests of the repository and branch. The helper presents the exact non-secret values; the gateway resolves the request's direct source IP to one Pod in the claimed `<ws>-sandbox` namespace and verifies every digest before it makes a domain-separated HMAC call to bex-api. bex-api then checks the `bex-agent/*` policy again, verifies the repository owner against the workspace's GitHub installation, and asks GitHub for a token narrowed to one repository with `contents:write` and `metadata:read`. GitHub installation tokens expire after one hour and support repository/permission narrowing, but do **not** encode branch scope ([GitHub installation-token documentation](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)). Tenants therefore should protect their default/release branches so only the intended `bex-agent/*` delivery branch is writable by this integration.

The OpenSandbox tenant API key is platform-side and is never injected into an agent sandbox. `/usr/local/bin/bex-pre-snapshot` still removes its enumerated emergency location along with Git credential files/cache state before a rootfs snapshot and fails closed if any location survives. Resume needs no restored secret: the next Git operation invokes the helper and re-mints.

### D3 — ACP is the agent interface; a sandbox-internal AI SDK driver fronts it

- The agent runs as an **ACP server over stdio** inside the sandbox (the SHOULD-support transport every candidate binary already speaks). It is spawned and owned by a **session driver**: a small Node process in the sandbox built on the Vercel AI SDK with the [`@mcpc-tech/acp-ai-provider`](https://ai-sdk.dev/providers/community-providers/acp) community provider (`createACPProvider({command, args, session, persistSession, existingSessionId})`), which adapts any ACP agent — `claude-code-acp`, `gemini --experimental-acp`, `codex-acp` — to `LanguageModelV3`. The provider is **stdio-spawn only** (no remote transport), which is precisely why the driver lives inside the sandbox rather than on any server. Agent pluggability (D7) collapses to a `command`/`args` config.
- The driver exposes two local listeners, and the sandbox's NetworkPolicy admits ingress **only from the gateway**:
  1. **AI SDK UI-message stream (SSE)** — `streamText` over the provider, consumed by the dashboard's `useChat`/AI Elements. Plans, diffs, and terminal output reach the provider only as server-side `raw` chunks (`includeRawChunks: true`), which `toUIMessageStream()` never forwards — so the driver maps them into typed **`data-acp` parts** before publishing (shipped: `lego/agent-image/driver/src/session.mjs`), and the browser renders those parts with session UI components.
  2. **Raw ACP JSON-RPC (WebSocket)** — pass-through to the agent's stdio for native ACP clients (Zed-class IDEs), phase 2+.
- bex-api exposes `POST /v1/agent-sessions` (+ GraphQL/MCP twins): authorize `can_operate` via OpenFGA, create/resume the sandbox, mint an **HMAC session ticket** on the web-shell pattern (`BEX_SHELL_TICKET_SECRET` design: DB-backed single-use nonce for long-lived streams, short expiry, claims bind subject + sandbox pod + workspace), and hand the client the gateway origin. Shipped in w3/m39: the durable `agent_sessions` row, first-class `agent_session → workspace` OpenFGA tuple, create/resume/list/get/cancel adapters, reserved sandbox metadata, and ticket mint. `BEX_AGENT_SESSION_GATEWAY_URL` supplies the returned origin; the phase-2 gateway consumes the ticket and claims its nonce through the existing shared `shell_ticket_nonces` store.
- The **isolated ssh-gateway** grows a third listener that verifies the ticket, claims the nonce, and **proxies both streams** to the driver — the same reverse-proxy shape as the existing sandbox-exec SSE path, replacing the previously planned bespoke WebSocket⇄JSON-RPC bridge for the dashboard. bex-api never touches the sandbox network path, and the gateway remains the sole session ingress and sole holder of `pods/exec` (lifecycle/debug) — unchanged trust design (ADR035).
- The gateway **tees the session stream into a transcript** in the control-plane DB. Reattach and multi-surface attach (dashboard + IDE on one session) replay the transcript, then resume live. Agent-side conversation resume after hibernation maps to the provider's `existingSessionId` → ACP `session/load`, used **only when the agent advertises `loadSession`** (the agent's own on-disk session state survives in the rootfs snapshot); transcript replay is the universal fallback. Reconnection and multi-client-attach semantics are bex-defined (the spec leaves them open).
- Accepted trade-offs of the driver: the browser-facing wire protocol is the AI SDK UI-message stream, not ACP itself (native-ACP attach stays available on listener 2); the provider reports **zero token usage** (ACP does not carry it), so token metering cannot come from this path (D6's metering proxy was already the plan); ACP permission requests surface no client callback — the agent runs auto-approve inside the sandbox, which is the exemplar posture (the sandbox + egress policy is the safety boundary, not permission prompts); the provider is young (v0.2.x, AI SDK v6, dynamic `acpTools()` marked experimental) — pin and vendor-test it, and keep the raw-ACP listener as the escape hatch if it stalls.

**The UI-message stream is the end-to-end session API (verbatim-forward guarantee).** The reason `acp-ai-provider` + the AI SDK sit inside the sandbox at all is so that the tenant-facing streaming API of an agent session **is** the standard [AI SDK UI-message stream](https://ai-sdk.dev/docs/ai-sdk-ui/stream-protocol) (`v1`) — the format the frontend Vercel AI SDK (`useChat`) consumes natively, with zero client-side adaptation. This is already true at the source: the shipped driver emits `streamText → toUIMessageStream()` plus the `data-acp` mapping, and serves it in-sandbox at `GET /stream` with `content-type: text/event-stream` **and the protocol marker `x-vercel-ai-ui-message-stream: v1`** (`lego/agent-image/driver/src/server.mjs`). The following invariants bind every hop between that endpoint and the browser (i.e. the gateway attach listener, `w3/m43`):

1. **Byte-transparent forwarding.** The gateway reverse-proxies the stream verbatim — it MUST NOT re-encode, filter, reorder, translate, or inject stream parts, and MUST preserve the `x-vercel-ai-ui-message-stream: v1` header end to end (exposing it cross-origin via `Access-Control-Expose-Headers`, with the dashboard origin CORS-allowlisted). "No protocol bridging" is a contract, not a convenience: any transformation would silently fork the wire format away from what `useChat` validates.
2. **Transport-level additions only.** The gateway may add exactly three things, none of which alter stream bytes: (a) **authentication** — the D3 HMAC ticket (90s TTL, DB-backed single-use nonce, claims binding subject + session + exact sandbox pod + workspace namespace), carried in a header, never the URL (web-shell precedent); (b) the **transcript tee** — a read-only copy into the control-plane store; (c) **replay-then-live splicing** — on (re)attach, previously teed parts are replayed first and the live tail spliced after, so the client sees one continuous `v1` stream (`useChat`'s `messages`-seed + `resumeStream` consume this directly).
3. **Single safe path.** The sandbox NetworkPolicy admits ingress only from the gateway; the gateway is the sole session ingress and bex-api never joins the stream path (unchanged ADR035 trust design). TLS terminates at the platform edge onto the gateway exactly as for the web shell. Reattach after ticket expiry mints a fresh ticket via the attach verb (gap #8).
4. **Version lockstep.** The driver is pinned to the AI SDK **v6** line by `acp-ai-provider` (`0.2.9` + `ai@6.0.237` today); the dashboard pins the v6-paired client (`@ai-sdk/react@3.x`). The two ends upgrade together — never independently — because later majors add part types older clients don't know, and the `v1` marker alone does not guarantee part-level compatibility across majors.

Net effect: any AI SDK client — the bex dashboard's `useChat`, or a tenant's own AI SDK app pointed at the attach endpoint with a valid ticket — can consume an agent session as a standard streaming chat API.

### D4 — Delivery: draft PR + evidence

A session ends (or checkpoints) by pushing its `bex-agent/*` branch with the session token and opening a **draft PR** via the GitHub App from bex-api. The session record attaches Codex-style verifiable evidence — command log, test output tails — sourced from the transcript. Steering channels: new ACP prompt turns on an attached session (phase 2), and a PR-comment loop (webhook → resume session) as a follow-on.

**Shipped completion mechanism (w3/m41).** Delivery is deterministic and enforced outside the agent (the Copilot model): after the headless turn, the **sandbox driver** stages the working tree, commits, pushes the `bex-agent/*` branch through the m38 credential helper, and writes the head SHA + a bounded evidence extract into its machine-readable status file. A **bex-api background Completer** polls each running session's status file through the existing gateway sandbox-exec boundary (a trusted system seam — no per-tick tenant authorization, since it acts only on durable sessions the platform owns; bex-api still never holds `pods/exec`), and on a pushed successful turn opens (or idempotently reuses) the **draft PR** via the GitHub App, records `headSha`/`prUrl`/`prNumber`/`evidence` on the session, and tears the sandbox down. A failed turn, a lost sandbox, or a failed PR-open becomes a `failed` session with a named reason — never a hang. **Phase-1 steering** (D8) re-dispatches a fresh sandbox on the same branch with the new prompt (a new prompt cannot ride the original sandbox until live attach exists); the Completer then updates the same draft PR, and `turns` + `deliveryMode` record the multi-turn history. Live E2E is proven by `scripts/agent-session-verify.sh`.

### D5 — Egress: default-deny with per-session opt-in

The `<ws>-sandbox` default-deny NetworkPolicy stays. Baseline allowlist for the agent phase: GitHub (clone/push via the helper) and the model API endpoint only; setup phase may open package registries. Tenants may widen per session with an explicit allowlist (Codex pattern). When a metering LLM proxy exists (D6 phase 2), the model-API allowlist narrows to the proxy — one mechanism then provides token metering **and** the exfiltration choke point.

Wave-1 implementation (w3/m40): `internal/sessionegress` creates one namespaced `CiliumNetworkPolicy` keyed by the immutable `bex.co/agent-session` Pod identity before OpenSandbox creates the Pod. Setup admits the fixed package registry catalog (`BEX_AGENT_SETUP_REGISTRIES` can replace the defaults), GitHub, the HTTPS model endpoint derived from that session's selected agent/provider config, the identity-authenticated gateway credential hop, and validated exact tenant hostnames. The one-way transition updates that same policy to remove package registries; the allowlist hash must remain unchanged. The clusterwide `sandbox-egress-default-deny` contains only structural node, API, metadata, and private/rebinding denies, while its former positive rules moved to a legacy policy that explicitly excludes agent-session Pods. Thus policy creation, transition, and deletion never remove the underlying deny. API-server admission confines bex-api's dynamic Cilium authority to canonical `<ws>-sandbox` namespaces and rejects CIDR/entity, wildcard, broad-selector, and non-TLS public rules. Tenant entries are exact public DNS names only; invalid, duplicate, private, wildcard, URL-shaped, or excessive entries return the named `AGENT_SESSION_EGRESS_ALLOWLIST_INVALID` 400.

The D6 phase-2 proxy is a required narrowing, not an optional alternative: once it exists, the session agent/provider resolver must return only that proxy endpoint and direct vendor model endpoints must disappear from newly rendered policies. The proxy then owns vendor selection and token metering behind the single egress choke point.

### D6 — Billing: two meters, quota + overage — explicitly no per-request meter

Two new ADR023 meter kinds through the existing sealed-outbox → Stripe pipeline, gated by ADR046's `PaymentGate`:

1. `sandbox_compute_seconds` — per-second sandbox lifecycle compute (vCPU/GB-weighted, AgentCore-style; hibernated sessions do not accrue). Needed regardless: sandboxes are currently unmetered.
2. `agent_token_units` — model-token passthrough at per-model rates (AI-credits-style), phase 2, sourced from agent usage reporting or the metering proxy.

Packaging: plan-included agent quota + metered overage (Devin/Cursor pattern) via Stripe metered Subscriptions with included quantities. **Per-request/premium-request billing is rejected** — the exemplar that invented it retired it because autonomous sessions break the unit economics.

### D7 — Agent binary: BYO-API-key first, binary pluggable

v1 is **bring-your-own-API-key**: the tenant supplies their model credential (stored in OpenBao per ADR013, injected at session start, scrubbed per D2). This sidesteps the unresolved Claude Code resale/ToS question, matches bex's open-source positioning, and reduces v1 billing to the compute leg. The template image treats the agent as pluggable; selection criteria are verified ACP maturity and `loadSession` support (candidates: Claude Code via its ACP adapter, Gemini CLI, OpenHands, opencode). Bundled-token billing and a default bundled agent are revisited once the metering proxy exists and the ToS question is answered.

### D8 — Phasing

- **Phase 1 — fire-and-forget** (Copilot shape, no interactive attach): `POST /v1/agent-sessions` → the same D3 session driver runs the task headless (one `streamText` turn, no client stream) → draft PR + evidence. Needs only D1/D2/D4/D5 + the compute meter. Steering = new prompt turns that re-dispatch the sandbox on the same branch. **Shipped in w3/m41** (delivery, evidence, steering, live E2E; the compute meter is D6/w7 follow-on).
- **Phase 2 — live attach**: the gateway session proxy + transcript tee/replay, dashboard session UI on `useChat`/AI Elements (integration, not protocol work), token metering via the LLM proxy.
- **Phase 2+ — native ACP IDE attach**: the driver's raw-ACP WebSocket listener proxied through the same ticket path, for Zed-class clients.

This ordering ships tenant value while the ACP network-transport draft and adopter ecosystem mature, and keeps phase 2 purely additive on the same session model.

### D9 — Dashboard session surface: one render layer, two swappable data sources

The dashboard surface (formerly held note `.pm/w5/035.md`, materialized as `w1/m64`; deep-research pass 2026-08-02) is the Devin-shaped session UI: a sessions list + new-session composer, a session detail page with an activity timeline, a draft-PR/evidence card, and a steering composer. Its architecture is chosen so the phase-1 (fire-and-forget) and phase-2 (live attach) products share **one rendering layer** and differ only in data source:

```mermaid
flowchart TB
  dev@{ shape: tri, label: "developer" }

  subgraph dash["dashboard (TanStack Start) — features/agent-sessions"]
    routes["routes: /agents (list + composer), /agents/$id (detail)"]
    render["UIMessage[] render layer (vendored AI Elements: Conversation / Reasoning / Task / Tool / PromptInput)"]
    poll["phase-1 data source: Apollo hooks (create / steer / cancel mutations + agentSession poll: phase, evidence, PR)"]
    chat["phase-2 data source: useChat + DefaultChatTransport (transcript replay, then live SSE)"]
  end

  api["bex-api (GraphQL agent-session verbs + HMAC ticket mint)"]
  cpdb[("control-plane Postgres: agent_sessions, ticket nonces, phase-2 transcripts")]
  gw["isolated ssh-gateway — phase-2 attach listener (verify ticket, claim nonce, reverse-proxy SSE, transcript tee)"]

  subgraph sbns["&lt;ws&gt;-sandbox namespace"]
    driver["session driver GET /stream (AI SDK UI-message SSE, data-acp parts)"]
  end

  dev --> routes
  routes --> render
  render --> poll
  render --> chat
  poll -->|"GraphQL over Kratos session cookie"| api
  chat -->|"1: mint attach ticket (GraphQL)"| api
  chat -->|"2: cross-origin SSE + HMAC ticket"| gw
  api --> cpdb
  gw -->|"nonce claim + transcript tee/replay"| cpdb
  gw --> driver
```

- **Two data planes, one page.** Control-plane metadata (session list, `phase`, PR card, evidence, `failureReason`) stays on the house GraphQL data layer — `dashboard/src/features/agent-sessions/` (NOT `sessions/` — that name is taken by Kratos login-session auth artifacts), flat routes `agents.tsx` + `agents.$agentSessionId.tsx`, typed hooks with `cache-first` + 30s poll tightened to ~5s while `phase` is non-terminal, mutations `no-cache` with typed domain errors and inline `AGENT_SESSION_*` mapping. The **conversation column** runs on `useChat` over the target-shape stream endpoint and nothing else: the stream's replay mode is the single history source for terminal and running sessions alike. _(Superseded 2026-08-02, before build: an earlier plan had the dashboard synthesize a `UIMessage[]` timeline from polled data plus localStorage-persisted prompts as a pre-gateway interim — discarded with the target-API-shape decision so the interim never ships; `w1/m64` builds directly on the `w3/m43` contract, with vendored AI Elements — hook-agnostic, React 19 + Tailwind 4, both already the dashboard's stack — rendering the parts either way.)_
- **Transport.** `useChat` + a transport pointed at the same-origin stream path (`api.bex.co/v1/agent-sessions/{id}/stream` — no CORS, no second origin in dashboard config): ticket rides a header, never a URL; `prepareReconnectToStreamRequest` re-mints via the `attach-ticket` verb per reconnect (the 90s TTL makes per-connect mint mandatory); `dataPartSchemas` validate the driver's `data-acp` parts, rendered as collapsible Reasoning/Task/Tool/Terminal groups (the Devin "Worked/Thought for Xs" shape). Steering routes by state: live session ⇒ chat `POST` (`sendMessage`); idle ⇒ the redispatch verbs. Client disconnect ≠ cancel — Cancel stays an explicit verb. Degraded states are honest per the house 503 pattern: stream endpoint unconfigured/unavailable ⇒ the conversation column says so while the metadata views keep rendering.
- **Version pin.** The in-sandbox driver is held to the AI SDK **v6** line by `@mcpc-tech/acp-ai-provider` (`ai@^6`; no v7 support yet), while `ai@7` is current. The dashboard pins `@ai-sdk/react@3.x` (the v6-paired line) to match the driver's stream, and both sides upgrade together — the UI-message stream protocol (`v1`) is shared, but skew is not tested territory.
- **Backend gaps the frontend exposes** (fold into the phase-2 milestone): a ticket-only **attach verb** for running sessions (today only create/resume/steer mint, so a page reload on a running session cannot reattach); **per-turn history** (`turns` is an integer counter — steering prompts and prior-turn evidence are not stored, so a reloaded phase-1 page loses turn wording); optional +/− diff stats in evidence for the PR card.
- **Exemplar table stakes held to** (verified against Devin/Copilot cloud agent/Codex cloud, 2026-08-02): grouped/collapsible activity rather than a flat log, a diff-first deliverable card with the PR link, a mid-run-capable steering composer, explicit stop/cancel with preserved-work semantics, and status/duration metadata. Devin's Shell/Browser/Editor workspace tabs are differentiators, not table stakes, and are out of scope.

**Target API shape (decided 2026-08-02) — two planes, one public origin.** The session API converges on a control plane and a conversation plane, split by required authority, not by protocol:

| Plane | Served by | Endpoints |
| --- | --- | --- |
| Control (JSON; REST/GraphQL/MCP parity) | bex-api process | `POST /v1/agent-sessions` (create, +ticket) · `GET /v1/agent-sessions[/{id}]` (list/get: phase, PR, evidence) · `POST /{id}/cancel` · `POST /{id}/resume` (revive idle session, +ticket) · `POST /{id}/attach-ticket` (reconnect mint, new — gap #8) |
| Conversation (Vercel AI SDK UI-message contract) | gateway process | `POST /v1/agent-sessions/{id}/stream` (submit a prompt turn on a live session; verbatim-forwarded to the driver) · `GET /v1/agent-sessions/{id}/stream` (reconnect: transcript replay then live tail; **terminal sessions: replay-only then `[DONE]`**) — ticket-authenticated |

- **Public origin.** Both planes publish under the primary API origin (`api.bex.co/...`): the platform edge routes the `/stream` path to the **gateway process** by path rule. External developers see one API product with one origin and no CORS second origin; which process serves which path is an internal implementation detail (the GitHub pattern: log downloads are `api.github.com` API, bytes come from a blob host). Two recorded caveats: this is a deliberate, documented exception to the "everything on the API origin is served by the bex-api process" audit assumption, and the gateway MUST ignore any cookies arriving on that path — the ticket is the only accepted credential. A neutral dedicated origin remains a fallback if edge path-routing proves operationally troublesome.
- **Absorbed into the chat contract**: live steering (the chat `POST` _is_ the prompt turn — the `steer` verb narrows to idle-session redispatch and is slated for deprecation into `resume` + chat `POST` once live attach ships; the MCP `steer_agent_session` tool survives as a one-shot alias for non-streaming clients); transcript reading (the stream endpoint's replay mode serves reattach **and** terminal-session history — an optional JSON transcript read may be added for MCP/REST parity); the dashboard consumes this contract directly — the once-planned client-side prompt-persistence interim was discarded before build and never ships.
- **Explicitly not absorbed**: `create`/`cancel`/`list`/`get`/`resume` stay JSON control verbs. Creation precedes the driver's existence (authz, sandbox provisioning, egress policy, credential wiring happen before there is anything to forward to); client disconnect ≠ cancel (AI SDK semantics — `stop()` only closes the connection); listing and result metadata (phase, PR, evidence, billing) are control-plane authority written by the Completer; and non-streaming consumers (MCP tools, CLI scripts, CI) need the poll-shaped surface — GitHub's public agent-tasks API is poll-only, validating that shape as a product in itself.

**Shipped in w3/m43.** The conversation plane is built:

- **Transcript store** (`agent_session_transcripts`, migration 0067): parts stored as **`text`, not `jsonb`** — jsonb canonicalizes (reorders keys, adds whitespace) and would corrupt the verbatim-forward guarantee. `seq` is the driver's emission ordinal (0-based); `PRIMARY KEY (session_id, seq)` + `ON CONFLICT DO NOTHING` makes the tee idempotent across gateway replicas and re-attaches. `ON DELETE CASCADE` with the session; retention prune rides the audit sweep.
- **Gateway attach listener** (`internal/sshgateway/agent_attach.go`, browser-facing `:8083`): verifies the agent-session ticket (reusing `BEX_SHELL_TICKET_SECRET`), single-use-claims its nonce in the shared `shell_ticket_nonces` store, then — on `GET` — replays the durable transcript to the client and splices the live driver stream, teeing new parts; on `POST`, forwards a live prompt turn. The listener resolves the ticket's pod → IP and **dials the in-sandbox driver's stream port directly** (no `pods/exec`; the sandbox NetworkPolicy admits only this one gateway ingress). Byte-transparent: it preserves the `data:` payloads and the `x-vercel-ai-ui-message-stream: v1` header, and ignores cookies. A gone pod (terminal session) yields transcript replay + `[DONE]`. **CORS:** although the endpoint lives on the api origin, the dashboard calls it cross-subdomain (`dashboard.bex.co → api.bex.co`), so the handler answers the OPTIONS preflight and echoes the matched Origin + `Access-Control-Expose-Headers: x-vercel-ai-ui-message-stream` (reusing `BEX_API_CORS_ORIGIN`) — without it the browser blocks the stream even though `curl` works. _(This gap was caught by a live prod probe, 2026-08-02, not by the unit tests, and fixed in place.)_
- **Driver live turn** (`POST /turn`, `lego/agent-image/driver`): runs another turn on the persistent session with the UI-message stream kept open, single-flighted (concurrent `POST` → 409), mirroring parts to both the hub (attached `GET` clients) and the `POST` response.
- **Steer decision (recorded):** live steering is the chat `POST` to the stream; the `steer` verb keeps the idle/terminal **redispatch** path and returns a **documented 409** (`AGENT_SESSION_TURN_IN_FLIGHT`) on a running session — no silent absorption. The MCP `steer_agent_session` tool stays a one-shot alias.
- **`attach-ticket` verb** is three-surface (REST `POST /{id}/attach-ticket`, GraphQL `attachAgentSession`, MCP `attach_agent_session`) — the Render-parity twin of the other mint verbs. The **stream endpoint itself is intentionally REST-only**: a raw byte stream is not a GraphQL/MCP shape, so those surfaces expose the ticket (to attach) rather than duplicating the stream.
- **Sandbox ingress:** the gateway→driver dial (bex-system → `<ws>-sandbox` :8787) is granted by the cluster-wide Cilium policy `sandbox-agent-driver-ingress` (deploy/gitops), NOT a per-namespace k8s allow — the ADR045 tenant-namespace admission control confines bex-api to converging default-deny only, so a duplicate k8s policy would be rejected (found live on prod, w3/m43). The k8s layer stays default-deny.
- **Edge routing:** a Traefik `IngressRoute` (`config/ssh/ingressroute-agent-attach.yaml`) path-routes `api.bex.co/v1/agent-sessions/{id}/stream` to the gateway with priority over bex-api's `/` Ingress, reusing the `bex-api-tls` cert.

The live-substrate E2E leg (extending `scripts/agent-session-verify.sh` with attach/replay/turn/reattach) shares the m41 operator-run gate; unit + real-Postgres + driver coverage is green.

---

## Consequences and gaps to close

1. **Gateway session proxy** (phase 2, materialized as `w3/m43`): a long-lived SSE + WebSocket reverse proxy to the driver — same shape as the existing sandbox-exec SSE path, no protocol bridging for the dashboard, **bound by D3's verbatim-forward guarantee** (byte-transparent, `x-vercel-ai-ui-message-stream: v1` preserved and CORS-exposed, additions limited to auth/tee/replay). Reconnect + multi-client fan-in semantics are still bex-designed. The `@mcpc-tech/acp-ai-provider` dependency is young (v0.2.x): pin it, and keep the raw-ACP listener as the fallback interface.
2. **Sandbox suspend is broken under PSS** (hostPath commit Job) — upstream OpenSandbox fix required; hibernation economics (D6) depend on it.
3. **Sandbox metering does not exist** — new meter kind + emitter codepath + `pricing.yaml` + Stripe catalog entry (`scripts/stripe-billing-setup.py`).
4. **Credential-helper refresh path** — new gateway internal endpoint + bex-api mint verb; audit-logged.
5. ~~**Session transcript store**~~ — **shipped in w3/m43** (`agent_session_transcripts`, migration 0067): durable, verbatim (`text`), ordinal-keyed idempotent tee; serves reattach replay + terminal-session history.
6. **OpenFGA modeling for agent sessions** — sandbox authz is code-level today; sessions should get first-class tuples.
7. **CRIU memory snapshots** stay a watch item — transcript replay makes them non-blocking.
8. **Frontend-exposed API gaps** (D9, resolved into the target API shape): the `attach-ticket` reconnect mint; the transcript store, whose stream-endpoint replay mode serves both reattach and terminal-session history (per-turn prompt/evidence history — today `turns` is a bare counter); optional diff stats in evidence — schedule with the phase-2 gateway milestone per the D9 target API shape (including the `api.bex.co` edge path-routing and the steer-verb deprecation path).
9. **Open questions carried**: Devin/Claude-web/Cursor internals (memory-snapshot resume), Claude Code resale ToS ([legal page](https://code.claude.com/docs/en/legal-and-compliance) to be verified at build time), the Streamable HTTP draft's trajectory, exemplar branch-confinement enforcement detail.

## Alternatives considered

- **Per-request ("premium requests") billing** — rejected; retired by its inventor effective 2026-06-01 (four stale premium-request claims were refuted 0-3/1-2 in verification; any source describing multipliers as current is outdated).
- **Bespoke session protocol instead of ACP** — rejected; ACP buys IDE clients (Zed, JetBrains direction) and agent pluggability for the cost of supplying transport semantics bex must build either way.
- **AI SDK driver outside the sandbox** (dashboard SSR spawning or dialing the agent remotely) — rejected; `@mcpc-tech/acp-ai-provider` is stdio-spawn only with no remote transport, and a remote-transport fork would recreate exactly the bespoke bridge the in-sandbox driver avoids.
- **Bespoke WebSocket⇄JSON-RPC gateway bridge as the dashboard path** — superseded by the D3 driver (2026-08-01 amendment); survives only as the raw-ACP listener for native IDE attach.
- **kubernetes-sigs/agent-sandbox or E2B-hosted substrate** — rejected; duplicates w3/m32.
- **Bundled model spend at flat ACU-style rates in v1** — deferred, not rejected; requires the metering proxy and margin/ToS clarity first (D7).
- **Replacing the JSON control verbs with the AI-SDK stream endpoint (SSE-only session API)** — rejected (2026-08-02, D9 target-API-shape decision): only in-conversation actions (prompt turns, stream parts, history replay) fold into the chat contract; create/cancel/list/get/resume require control-plane authority the verbatim-forwarded driver path cannot exercise, and non-streaming consumers (MCP tools, CLI scripts, CI) need the poll-shaped JSON surface — GitHub's public agent-tasks API is poll-only. What **was** absorbed: live steering (chat POST) and transcript reading (stream replay mode).
