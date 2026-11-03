# w9 · m168 — ADR095 phase 1 — attachment storage, upload, binding and sandbox delivery

**Worker:** worker9 **Goal:** a REST or GraphQL caller can upload a file, attach it to a new session or a follow-up, and the agent sees it in a fresh sandbox under `/attachments`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Attachments bucket, S3 client and credentials | 45m | — |
| t002 | `att` id kind, attachments table and store methods | 40m | — |
| t003 | Attachment core: upload, read, list, delete, presign, sweep, cascade | 90m | t001, t002 |
| t004 | REST and GraphQL surfaces + capabilities | 60m | t003 |
| t005 | Bind attachments to turns on create, steer and live turns | 60m | t003 |
| t006 | Dispatch copy into `/attachments` and ACP prompt blocks | 90m | t005 |
| t007 | Render parity (Devin reference) | 25m | t004, t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 45m | t007, t008 |
| t010 | Closeout | 10m | t009 |

## Definition of done

- `POST /v1/agent-attachments` streams a file (≤ 25 MiB) to the attachments bucket, sniffs its type, hashes it and returns an `att-` id; oversized, over-quota and invalid uploads return the ADR095 D8 codes and leave nothing behind.
- `attachmentIds` on REST/GraphQL create and steer bind ready attachments to the turn atomically; foreign or cross-workspace ids are refused.
- Every dispatch copies all session inputs to read-only `/attachments` outside `/workspace`; the driver sends `resource_link` blocks to every agent and `image` blocks only when the agent supports them.
- `GET …/content` 302s to a presigned GET on the storage origin with pinned type and disposition; delete purges rows and objects; unbound uploads expire after 24 h.
- Live: on production with the real bucket, a session given a log file and a screenshot reads both and references them in its answer; download works from the dashboard origin via the presigned URL. Fixtures cleaned up.
- Backend real-DB suite, driver tests and `make lint` green.

## Source + Goal linkage

- **Source:** [ADR095](../../../docs/ADR095-agent-attachments.md) (accepted for implementation by user 2026-11-03: "/pm ADR095 to w9"); Devin gap list in [ADR094](../../../docs/ADR094-coding-agent-mcp.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 5 (cloud coding agents); Devin parity for agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)).
- **Expected outcome:** customers can hand the cloud agent screenshots and logs through the API, the core Devin "attach and fix" flow.
- **Why now:** every later phase (dashboard, outputs, MCP) builds on this storage and binding; ADR095 D12 phase 1.
- **Gate to anticipate:** the live DoD needs the production Wasabi bucket `bex-agent-attachments` and its writer/reader IAM users, which only the user can create (ADR019 out-of-band credentials). Code ships behind the 503-when-unconfigured path; if the bucket is still missing at Closeout, park the milestone in `blocked/` naming that action.
- **Parity:** included: REST/GraphQL surfaces; Devin is the reference (no Render counterpart).
