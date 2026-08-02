# w3 · m37 — Agent template image + in-sandbox session driver (ADR047 D1/D3/D7)

**Worker:** worker3 **Goal:** a sandbox created from a new agent template image runs a headless coding task on a checked-out repo and produces commits, driven by an in-sandbox session driver (Vercel AI SDK + pinned `@mcpc-tech/acp-ai-provider`) that fronts any ACP agent over stdio. **Status:** done — 2026-08-02 (see t009 closeout note: code shipped by an earlier session, this session audited every task against real code/CI/tests, closed the one genuine gap found (t004 — no OpenBao-sourced model key ever reached a real sandbox pod), and reconciled PM bookkeeping)

## Tasks (in order)

| id   | title                                                                                                | est | depends_on             | status |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------------------- | --- |
| t001 | Agent template image: git + toolchains + pluggable agent binaries (`command`/`args` config)          | 60m | —                      | — **DONE** |
| t002 | Session driver: Node + AI SDK + pinned `@mcpc-tech/acp-ai-provider`, stdio spawn, headless one-turn mode | 90m | t001                   | — **DONE** |
| t003 | Driver listeners: SSE UI-message stream + raw-ACP WebSocket (gateway-only ingress assumption)        | 60m | t002                   | — **DONE** |
| t004 | BYO model API key injection at session start (OpenBao-sourced), scrubbed pre-snapshot                | 45m | t002                   | — **DONE** |
| t005 | Vendor-test the provider: `existingSessionId`→`session/load`, `loadSession` probe, raw chunks        | 60m | t002                   | — **DONE** |
| t006 | Flip ADR047 → Accepted; record the D3 amendment as adopted                                           | 15m | —                      | — **DONE** |
| t007 | Simplify pass over the driver + image code                                                           | 20m | t003, t004, t005, t006 | — **DONE** |
| t008 | Test coverage: driver spawn/one-turn/listeners + provider vendor tests wired into CI                 | 45m | t003, t004, t005, t006 | — **DONE** |
| t009 | Closeout                                                                                             | 10m | t008                   | — **DONE** |

## Definition of done

- An agent template image exists (built from `lego/` or a sibling context) containing git, base language toolchains, at least one ACP agent binary (e.g. `claude-code-acp`), and the session driver; agent selection is pure `command`/`args` config (ADR047 D7 pluggability).
- The driver spawns the ACP agent over stdio via `createACPProvider`, runs a headless one-`streamText`-turn task against a pre-cloned repo inside a sandbox on the existing w3/m32 substrate, and the agent's commits land in the sandbox worktree — demonstrated end-to-end on the prod or dev-3 substrate.
- The driver exposes the two ADR047 D3 listeners (SSE AI SDK UI-message stream with `includeRawChunks: true`; raw-ACP JSON-RPC WebSocket) bound so only gateway-originated ingress can reach them.
- A tenant-supplied model API key (D7 BYO) is injected at session start from OpenBao (ADR013 pattern) and never written into a rootfs snapshot (scrub verified).
- `@mcpc-tech/acp-ai-provider` is version-pinned with vendor tests covering session persistence (`persistSession`/`existingSessionId` → `session/load` only when `loadSession` is advertised) and raw-chunk passthrough, running in CI.
- `docs/ADR047-cloud-coding-agent-sessions.md` is `Status: Accepted`.

## Source + Goal linkage

- **Source:** [docs/ADR047-cloud-coding-agent-sessions.md](../../../docs/ADR047-cloud-coding-agent-sessions.md) D1/D3/D7 + `/pm-brainstorm` decomposition 2026-08-01. Materializing this milestone is the ADR047 adoption decision (user-confirmed 2026-08-01).
- **Goal linkage:** pillar 5 (ADR008 AI-native) — the product layer on the shipped w3/m32 sandbox substrate; critical-path head for cloud coding-agent sessions.
- **Expected outcome:** the core session mechanism (image + driver + provider) proven end-to-end headless, de-risking the young v0.2.x provider dependency before any API/UX work builds on it.
- **Why now:** ADR047 wave 1; every other session milestone integrates against this driver's contracts (helper path, listener shapes), so it starts first. Render parity omitted: no REST/GraphQL/MCP/UI surface changes — internal image + in-sandbox driver only (the API surface is w3/m39).
