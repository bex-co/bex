# w9 · m168 — ADR095 phase 1 — attachment storage, upload, binding and sandbox delivery

**Worker:** worker9 **Goal:** a REST or GraphQL caller can upload a file, attach it to a new session or a follow-up, and the agent sees it in a fresh sandbox under `/attachments`. **Status:** blocked — t001–t009 done; t010 live acceptance waits for the production attachments bucket (see Blocked)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Attachments bucket, S3 client and credentials — **DONE** | 45m | — |
| t002 | `att` id kind, attachments table and store methods — **DONE** | 40m | — |
| t003 | Attachment core: upload, read, list, delete, presign, sweep, cascade — **DONE** | 90m | t001, t002 |
| t004 | REST and GraphQL surfaces + capabilities — **DONE** | 60m | t003 |
| t005 | Bind attachments to turns on create, steer and live turns — **DONE** | 60m | t003 |
| t006 | Dispatch copy into `/attachments` and ACP prompt blocks — **DONE** | 90m | t005 |
| t007 | Render parity (Devin reference) — **DONE** | 25m | t004, t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007, t008 |
| t010 | Closeout — **BLOCKED** (production bucket) | 10m | t009 |

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

## Outcome (2026-11-03)

t001–t009 are implemented, reviewed through `/simplify` (three reviewers, all findings applied), and verified on top of `origin/main`.

**Verification**
- Real-deps backend suite: `GOWORK=off BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...`, 74 packages ok.
- `make lint`: four modules plus deadcode.
- Driver: `tsc` and 125/125 tests.
- Shellcheck, ADR registry and prettier all clean.

**Deviations** are recorded in [ADR095 § Implementation notes](../../../docs/ADR095-agent-attachments.md):
- **Copy-in identity:** the copy uses a platform `SystemSubject` ticket. The gateway only accepts it for the attachments path, and concurrency is limited per session.
- **Copy layout:** one tar archive per dispatch, extracted at `/var/lib/bex-agent/attachments`, with `/attachments` linking to it.
- **Read-only inputs:** files are 0444, but the directory is not root-owned, because the gateway extracts as the agent's uid.
- **Storage credentials:** one IAM user, because bex-api is the only reader and writer.
- **Deletion:** workspace and session deletes mark attachment rows deleted inside their own transactions, and the sweep then removes the objects.
- **Upload lifecycle:** each upload starts as a `pending` row, so no object is ever left without a row.
- **Turn records:** each turn stores an `attachments` snapshot.
- **Live-turn file parts** are dropped; that path is disabled in the dashboard.
- **New error class:** `core.ErrTooLarge` for 413s.

**Parity (t007), with Devin as the reference.**

| Devin | bex |
| --- | --- |
| Upload returns an id/url | `POST /v1/agent-attachments` returns `att-…` |
| `attachment_urls` on create and message | `attachmentIds` on REST and GraphQL create/steer |
| `get_attachments` | `GET /v1/agent-sessions/{id}/attachments` and GraphQL `agentSessionAttachments` |

- REST and GraphQL share one core, with the same codes and the same `turn` validation.
- MCP create/steer already accept `attachmentIds`. The MCP upload, get, list and delete tools belong to m171.
- Agent outputs (`source: devin`) belong to m170.
- No unintended drift was found.

## Blocked (2026-11-03): t010 live acceptance

**Gate:** the production Wasabi bucket `bex-agent-attachments` and its IAM user do not exist yet. Creating them is a user action:

```sh
bash scripts/agent-attachments-s3-credentials.sh provision
bash scripts/agent-attachments-s3-credentials.sh install
```

This uses the Wasabi root credentials in `.env`, as the other bucket scripts do.

Until then, every attachment verb answers `503 AGENT_ATTACHMENTS_UNAVAILABLE` and sessions behave exactly as before.

**To clear:** run those two commands, wait for a deploy, then `/loopx w9` runs the live check in the definition of done: a session given a log and a screenshot reads both, and the download works.
