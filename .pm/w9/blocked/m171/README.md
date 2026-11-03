# w9 · m171 — ADR095 phase 4 — attachment tools on the MCP server

**Worker:** worker9 **Goal:** an MCP client can upload, attach, list and read agent attachments, including seeing shared images inline. **Status:** blocked — t001–t005 done; t006 live MCP-client check waits for the production attachments bucket (see Blocked)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | MCP attachment tools on `/mcp` — **DONE** | 60m | — |
| t002 | `attachmentIds` on the MCP session tools — **DONE** | 30m | t001 |
| t003 | Render parity (Devin reference) — **DONE** | 25m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 45m | t003, t004 |
| t006 | Closeout — **BLOCKED** (production bucket) | 10m | t005 |

## Definition of done

- `upload_agent_attachment`, `get_agent_attachment`, `list_agent_session_attachments` and `delete_agent_attachment` work on `/mcp` with correct scopes and the shared error codes.
- `get_agent_attachment` returns an MCP image block for raster images ≤ 5 MiB and inline text for text ≤ 256 KiB.
- MCP create/steer accept `attachmentIds` with REST-identical binding.
- The tools are tagged for the `cloud-agents` toolset if w21 has shipped tagging; otherwise the follow-up is recorded on w21's milestone.
- Verified with a real MCP client (Claude Code) against a dev or production stack.

## Source + Goal linkage

- **Source:** [ADR095](../../../docs/ADR095-agent-attachments.md) (accepted for implementation by user 2026-11-03: "/pm ADR095 to w9"); Devin gap list in [ADR094](../../../docs/ADR094-coding-agent-mcp.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 5 (cloud coding agents); Devin parity for agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)).
- **Expected outcome:** orchestrating agents can attach a log and read back a shared screenshot without a browser, matching Devin's `get_attachments`.
- **Why now:** ADR095 D12 phase 4; depends on m168.
- **Coordination:** ADR094 (`/mcp/cloud-agents`, toolset tagging, canonical tool names) is owned by w21 (user decision 2026-11-03). This milestone targets the existing `/mcp` tools and joins w21's toolset when it exists — it must not re-implement ADR094.
- **Parity:** included: MCP surface; Devin is the reference.

## Outcome (2026-11-03)

t001–t005 are done.

**The tools.** `upload_agent_attachment` (≤ 4 MiB decoded), `get_agent_attachment` (metadata + `downloadUrl`, plus an inline image ≤ 5 MiB or text ≤ 256 KiB) and `list_agent_session_attachments` / `delete_agent_attachment`. All of them go through the m168 core.

**w21 coordination.** w21 shipped the ADR094 toolset as `CloudAgentsTools` in `agentsessions/mcp.go`. All four tools are in it, so they serve at both `/mcp` and `/mcp/cloud-agents` and are covered by `bex.cloud_agents`.

**Fixed m168 gap.** MCP `send_agent_session_message` / `steer_agent_session` silently dropped `attachmentIds`. They now bind like REST, and the test fails without the fix (mutation-checked).

**Parity (t003), with Devin as the reference.** `get_agent_attachment` / `list_agent_session_attachments` cover Devin's `devin_session_interact get_attachments`. Upload, get, list and delete share codes and authorization with REST and GraphQL. No drift.

**Simplify (t004).** Reviewed the diff directly (small: one core helper, four tool registrations). No findings.

**Verification**
- Backend `go test ./...` and the real-deps suite (74 packages) pass.
- `make lint` passes.
- `mcp_attachments_test.go` covers:
  - exact image bytes and inline text;
  - metadata only above the inline limit;
  - an SVG never returned as an image;
  - the 4 MiB upload edge;
  - bad base64;
  - binding through create and both follow-up names;
  - list order and bound-file delete refusal;
  - another member's unbound upload being invisible.

**Deviation.** An SVG sniffs as `text/xml`, so the MCP tool returns it as text, never as an image. The dashboard "download-only" rule is unaffected.

## Blocked (2026-11-03): t006 live MCP-client check

The DoD asks for a run with a real MCP client against a live stack. That needs the production attachments bucket, the same gate as [m168](../m168/README.md), which only the user can create (`bash scripts/agent-attachments-s3-credentials.sh provision && … install`).

**To clear:** create the bucket, deploy, then `/loopx w9` runs it.
