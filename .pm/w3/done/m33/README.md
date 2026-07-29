# w3 · m33 — Sandbox exec: `render ea sandbox exec`

**Worker:** worker3 **Goal:** implement the one remaining `render ea sandbox` verb — `exec` — so an unmodified Render CLI (and MCP agents) can run a command in a hosted sandbox and stream stdout/stderr, completing the `ea sandbox` surface m32 shipped (create/list/stop). **Status:** **DONE** 2026-07-29 — `render ea sandbox exec` works unmodified end-to-end on the prod gVisor substrate (`sh -c 'uname -r'` → `4.19.0-gvisor` from inside the sandbox; exit codes + multi-part commands; foreign-sandbox scoping). **Option A**: exec runs via k8s `pods/exec` confined to the isolated ssh-gateway; bex-api authorizes `can_operate` + signs a per-exec ticket + reverse-proxies the SSE, never gaining `pods/exec`. t002 (token endpoint) **dropped** — the real CLI is single-POST-reads-SSE. MCP `sandbox_exec` ships; cli-compat row 239 `[x]`.

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------------- |
| t001 | OpenSandbox execd exec transport: add `Client.Exec` (run command → stdout/stderr stream)               | 3h  | —                | — **DONE** |
| t002 | Two-step run-token endpoint: `POST /v1/sandboxes/{id}/runs/{operation}/token` → `{method, uri, token}` | 2h  | t001             | — **DEFERRED** |
| t003 | SSE exec-stream endpoint: token-verified, workspace-key-scoped bridge to execd (the CLI's stream)      | 3h  | t002             | — **DONE** |
| t004 | MCP `sandbox_exec` (+ GraphQL if applicable): direct exec via execd, no token dance (agents, ADR014 D3) | 2h  | t001             | — **DONE** |
| t005 | Validate unmodified `render ea sandbox exec` end-to-end; flip cli-compatibility-checklist row 239       | 2h  | t003, t004       | — **DONE** |
| t006 | Render parity (`ea sandbox exec` across REST + MCP + CLI)                                               | 1h  | t005             | — **DONE** |
| t007 | Simplify (`/simplify` over changed code)                                                                | 30m | t006             | — **DONE** |
| t008 | Test coverage (token mint/verify, stream bridge, authz, MCP exec)                                       | 2h  | t006             | — **DONE** |
| t009 | Closeout                                                                                                | 15m | t008             | — **DONE** |

## Investigation findings (2026-07-29, corrects the initial spec)

- **The real CLI exec is single-step, not the two-step token flow.** `render ea sandbox exec <id> -- <cmd>` (v2.21.0) does **`POST /v1/sandboxes/{id}/exec?ownerId=<ws>`** with body `{"command":"<cmd>"}` and reads the **HTTP response body as an SSE stream** — there is no `/runs/{operation}/token` handshake in this CLI. Each SSE `data:` line is JSON: `{"stdout":"…"}` / `{"stderr":"…"}` output chunks, then a terminal event carrying `exitCode` (accepts `exitCode` or `exit_code`). Missing the exit event → the CLI errors `no sandbox exec exit event found in SSE response`. **So t002 (token endpoint) is dropped; t003 becomes a single `POST …/exec` SSE endpoint.**
- **execd is gRPC.** OpenSandbox's in-sandbox `execd` (port 44772, started by `bootstrap.sh`) exposes a **gRPC** service (no published proto) — not a practical HTTP bridge target. The OpenSandbox lifecycle server has **no exec endpoint** (only create/list/get/delete/pause/resume/snapshots/diagnostics/proxy). So exec must run via **k8s `pods/exec`** into the sandbox pod (`{sandbox-id}-0` in `<ws>-sandbox`), the same mechanism as running-instance SSH.
- **Architecture decision:** Option A shipped. The isolated **ssh-gateway** alone holds namespace-scoped `pods/exec`; bex-api authorizes, signs a short-lived exact target/command ticket, and reverse-proxies the SSE without receiving Kubernetes exec authority.

## Definition of done

`render ea sandbox exec <id> -- <cmd>` works against bex unmodified (v2.21.0+): one `POST /v1/sandboxes/{id}/exec` response streams stdout/stderr and the terminal exit status as SSE. The core authorizes `can_operate`, verifies the sandbox's durable owner/workspace/security metadata (with the explicit workspace-admin override from m35), and signs a short-lived ticket binding the exact namespace, sandbox id, subject, and command. The isolated ssh-gateway verifies that ticket and alone performs namespace-scoped `pods/exec`. MCP `sandbox_exec` uses the same authorized path and buffers the SSE result. bex-api never gains `pods/exec`.

## Source + Goal linkage

- **Source:** the live-CLI review of the `ea sandbox` surface (2026-07-29) found that the real CLI uses a single POST-and-SSE contract, not the initially assumed two-step run-token flow. Wire contract and production evidence are captured in [docs/render-artifacts/ea-sandbox.md](../../../docs/render-artifacts/ea-sandbox.md) §exec.
- **Goal linkage:** pillar 5 — hosted agent execution environments ([ADR008-vision](../../../docs/ADR008-vision.md)); completes the `render ea sandbox` CLI compatibility surface re-opened by [ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md).
- **Expected outcome:** agents and the Render CLI can not just spawn/list/stop sandboxes but actually **run code in them** and see the output — the point of a sandbox. The `ea sandbox` compatibility surface becomes complete except for the file/logs sub-surfaces (separate follow-ups).
- **Why now:** m32 shipped the substrate + create/list/stop and exec was the remaining CLI verb. The implementation reuses the isolated ssh-gateway so bex-api does not acquire Kubernetes exec authority.
- **Security follow-up:** m35 was planned as a prerequisite but landed concurrently after this milestone closed. Its owner check is now applied to exec itself, while its production Cilium/RBAC/admission rollout remains an urgent gate before any further sandbox expansion.
- **Render parity:** INCLUDED — `ea sandbox exec` is green across CLI/REST/MCP ([cli-compatibility-checklist:239](../../../docs/cli-compatibility-checklist.md)).
- **Cross-theme note:** AI-native/sandbox work, placed in w3 alongside m31/m32 by the same 2026-07-27 user direction (DO_NOT_DO #18); the sandbox line continues here.
