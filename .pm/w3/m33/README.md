# w3 · m33 — Sandbox exec: `render ea sandbox exec`

**Worker:** worker3 **Goal:** implement the one remaining `render ea sandbox` verb — `exec` — so an unmodified Render CLI (and MCP agents) can run a command in a hosted sandbox and stream stdout/stderr, completing the `ea sandbox` surface m32 shipped (create/list/stop). **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------------- |
| t001 | OpenSandbox execd exec transport: add `Client.Exec` (run command → stdout/stderr stream)               | 3h  | —                |
| t002 | Two-step run-token endpoint: `POST /v1/sandboxes/{id}/runs/{operation}/token` → `{method, uri, token}` | 2h  | t001             |
| t003 | SSE exec-stream endpoint: token-verified, workspace-key-scoped bridge to execd (the CLI's stream)      | 3h  | t002             |
| t004 | MCP `sandbox_exec` (+ GraphQL if applicable): direct exec via execd, no token dance (agents, ADR014 D3) | 2h  | t001             |
| t005 | Validate unmodified `render ea sandbox exec` end-to-end; flip cli-compatibility-checklist row 239       | 2h  | t003, t004       |
| t006 | Render parity (`ea sandbox exec` across REST + MCP + CLI)                                               | 1h  | t005             |
| t007 | Simplify (`/simplify` over changed code)                                                                | 30m | t006             |
| t008 | Test coverage (token mint/verify, stream bridge, authz, MCP exec)                                       | 2h  | t006             |
| t009 | Closeout                                                                                                | 15m | t008             |

## Definition of done

`render ea sandbox exec <id> -- <cmd>` works against bex unmodified (v2.21.0+): the CLI's two-step flow — `POST /v1/sandboxes/{id}/runs/stream/token` → `{method, uri, token}`, then open that uri with the token to stream stdout/stderr as SSE — round-trips a command in the caller's `<ws>-sandbox` sandbox and returns its output + exit status. Authorized (`can_operate` on the sandbox's workspace) and scoped by the per-workspace OpenSandbox key (m32 t006) so a caller can only exec into their own sandboxes. `cli-compatibility-checklist` row 239 (`ea sandbox exec`) goes `[-]`→`[x]` with captured evidence; MCP `sandbox_exec` runs the same command for agents without the browser two-step. bex-api never gains `pods/exec` — exec rides the OpenSandbox server/execd, exactly as create/list/stop do.

## Source + Goal linkage

- **Source:** the live-CLI review of the `ea sandbox` surface (2026-07-29): running the real `render` CLI found create/list/stop work (fixed under m32) but `exec` returns 404 — `/v1/sandboxes/{id}/runs/{operation}/token` is unimplemented. Wire contract captured in [docs/render-artifacts/ea-sandbox.md](../../../docs/render-artifacts/ea-sandbox.md) §exec.
- **Goal linkage:** pillar 5 — hosted agent execution environments ([ADR008-vision](../../../docs/ADR008-vision.md)); completes the `render ea sandbox` CLI compatibility surface re-opened by [ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md).
- **Expected outcome:** agents and the Render CLI can not just spawn/list/stop sandboxes but actually **run code in them** and see the output — the point of a sandbox. The `ea sandbox` compatibility surface becomes complete except for the file/logs sub-surfaces (separate follow-ups).
- **Why now:** m32 shipped the substrate + create/list/stop + per-tenant multi-tenant security and validated them on prod; exec was the one deferred verb. It depends on nothing new — the substrate, the per-workspace key plumbing (m32 t006), and the execd image (`opensandbox/execd:v1.0.16`, already in the cluster TOML) are all in place.
- **Render parity:** INCLUDED — `ea sandbox exec` is a CLI/REST/MCP surface ([cli-compatibility-checklist:239](../../../docs/cli-compatibility-checklist.md), today `[-]` 404).
- **Cross-theme note:** AI-native/sandbox work, placed in w3 alongside m31/m32 by the same 2026-07-27 user direction (DO_NOT_DO #18); the sandbox line continues here.
