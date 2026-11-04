# w9 · m171 — ADR095 phase 4 — attachment tools on the MCP server

**Worker:** worker9 **Goal:** an MCP client can upload, attach, list and read agent attachments, including seeing shared images inline. **Status:** todo — t001–t005 done; t006 live MCP-client check once m168 has provisioned the bucket

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | MCP attachment tools on `/mcp` — **DONE** | 60m | — |
| t002 | `attachmentIds` on the MCP session tools — **DONE** | 30m | t001 |
| t003 | Render parity (Devin reference) — **DONE** | 25m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 45m | t003, t004 |
| t007 | Align the MCP upload limit with the /mcp request body limit — **DONE** | 20m | t005 |
| t006 | Closeout | 10m | t005 |

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

## Unblock decision, 2026-11-04 (user decision D1, via `/unblock w9`)

- **Covers:** w9/m168, m169, m170, m171 (shared gate).
- **Decision.** The user authorizes the worker to create the production attachments bucket and its IAM user. The worker runs the existing idempotent script from the repo root:

  ```sh
  BEX_ENV_FILE=../bex9.env bash scripts/agent-attachments-s3-credentials.sh provision
  BEX_ENV_FILE=../bex9.env bash scripts/agent-attachments-s3-credentials.sh install
  ```

  - The root keys are `TF_STATE_ACCESS_KEY` / `TF_STATE_SECRET_KEY`, present by name in `../bex9.env`; never print their values.
  - The script uses the `hetzner-prod` context.
- **Reference.** Devin parity: upload, attach and returned files must work for real users.
- **Done signal.** `bex-system` has the attachments Secret, a deploy rolls bex-api with `BEX_AGENT_ATTACHMENTS_S3_*`, and `GET /v1/agent-sessions/capabilities` reports `attachments.enabled: true`.
- **Worker next step.** m168 runs the provisioning first. m169–m171 wait for that signal, then run their live checks as written in their definitions of done, signing in as the QA user (`scripts/qa-login.sh`).
- **If provisioning fails** (keys missing or rejected, Wasabi denies the call): stop, keep the error message without any secret, and return the item to `blocked/` with the exact failure.

## Live acceptance evidence (2026-11-04, production, real MCP client)

The client was the official `@modelcontextprotocol/sdk` 1.32.2 `StreamableHTTPClientTransport` against `https://api.bex.co/mcp/cloud-agents`, with a QA bearer.

**Passed**
- The four attachment tools are listed.
- `upload_agent_attachment` of a PNG returned an `att-` id, `image/png`.
- `get_agent_attachment` returned `text`+`image` blocks, and the image bytes are identical to the upload.
- For a log it returned `text`+`text`, with the marker line present.
- `list_agent_session_attachments` on a session in another of the caller's workspaces listed both inputs as `input`, turn 1.

**FAILED: the advertised 4 MiB upload cap is unreachable.**
- An upload of 1.4 MiB decoded succeeds; 1.6 MiB fails with a transport-level `413 request body exceeds 2097152 bytes`.
- **Cause:** `/mcp` sits under the global `BEX_MAX_BODY_BYTES` (2 MiB) body limit, and base64 inflates the payload by about 4/3.
- **Fix:** either exempt or raise the `/mcp` limit (about 6 MiB, enough for 4 MiB of base64 plus the JSON envelope), or lower `maxMCPUploadBytes` to about 1.5 MiB and update the tool description. Add a test at the real HTTP boundary.
