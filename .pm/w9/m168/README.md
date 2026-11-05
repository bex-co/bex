# w9 · m168 — ADR095 phase 1 — attachment storage, upload, binding and sandbox delivery

**Worker:** worker9 **Goal:** a REST or GraphQL caller can upload a file, attach it to a new session or a follow-up, and the agent sees it in a fresh sandbox under `/attachments`. **Status:** todo — t001–t009, t011 done; t010: re-run the agent-reads-attachments live check once the t011 fix deploys

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
| t011 | Dispatch archive writes each `<turn>/` directory before its files — **DONE** | 20m | t009 |
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

## Live acceptance evidence (2026-11-04, production, QA user)

The production bucket `bex-agent-attachments` was provisioned and verified (put/get/delete allowed, `bex-tfstate` denied). The Secret was installed and bex-api was rollout-restarted at deployed SHA `fb6098dd2`.

**Passed**
- **Capabilities.** `GET /v1/agent-sessions/capabilities` reports `attachments.enabled: true` (25 MiB per file, 10 per turn, 50 / 250 MiB per session).
- **Uploads (bex-canary).** `build.log` → `text/plain`, `red-square.png` → `image/png`. HTML uploaded as `fake.png` was sniffed as `text/html`.
- **Size cap.** 25 MiB + 1 byte → `413 AGENT_ATTACHMENT_TOO_LARGE`, and no row was left behind.
- **Download.** `GET …/content` → 302 to `s3.eu-central-2.wasabisys.com`, and the bytes are identical for all three files.
  - The PNG is served `inline`; the log and the HTML are served `attachment`.
- **Binding (workspace `bex`).** A repo-less session created with 2 `attachmentIds` has both inputs bound to turn 1, and the transcript turn snapshot lists both. A dashboard follow-up bound a new file to turn 2.

**Not proven: the agent reading the files.** No sandbox could start. Every dispatch failed with `FailedCreatePodSandBox … runtime "io.containerd.runsc.v1" binary not installed "containerd-shim-runsc-v1"` on sandbox node `bex-sandbox-cgqq6-vlvqv` (created about 2 days ago). This breaks every agent session on production and is unrelated to attachments. The copy into `/attachments` and the ACP blocks therefore still need a working sandbox.

**Separate bug.** That runtime failure is reported to the customer as `SANDBOX_CAPACITY_LIMIT`, "the likely cause is this workspace's sandbox capacity (plan limit)". The node was nearly idle (3% CPU requested).

**Cleanup.** Fixtures were deleted and their absence verified: sessions → 404, attachments → 404, no BatchSandbox or pods left.

## Live acceptance (2026-11-05, production at `dcdc774c1`)

- **FAIL — blocker found.** Dispatching a session with attachments fails with `ATTACHMENT_COPY_FAILED`. The bex-api log says `sandbox file gateway answered 400`, and the ssh-gateway log says `invalid or unsupported sandbox archive: parent directory must precede its children`.
  - Root cause: the copy archive in `lego/backend/internal/agentsessions/attachments_dispatch.go:142-144` writes only `TypeReg` entries (`<turn>/<att-id>-<name>`) with no `TypeDir` header for `<turn>/`. The gateway's strict extractor (`lego/backend/internal/sshgateway/sandboxfiles/sandboxfiles.go:597`) rejects that.
  - The unit tests used a fake copier, so they never exercised the real archive.
  - Fix: emit a directory header for each turn directory before its files, and add a test that runs the real gateway extractor on the produced archive.
- **PASS:**
  - REST upload: `build.log` was sniffed as `text/plain` (165 B) and `shot.png` as `image/png` (339 B), with sha256 matching locally.
  - `attachmentIds` on create bound both files.
  - `GET …/content` returns a 302 to `s3.eu-central-2.wasabisys.com`. The downloads are byte-identical; the log is served as `attachment`, the PNG as `inline`.
- **Not exercised** (blocked by the copy bug): the agent reading the files, and a follow-up turn with a file.
- **Workspace.** Agent sessions ran in `bex` (`tea-d98210cbbpdc73dcrkvg`), the only QA workspace with `ready=true` and a model key. `bex-canary` reports `ready=false` / `modelKeyReady=false`.

## Fix after live acceptance (2026-11-05): t011

**Symptom.** Every agent session with an attachment failed `ATTACHMENT_COPY_FAILED`. bex-api logged `sandbox file gateway answered 400`; the gateway logged `parent directory must precede its children`.

**Cause.** `writeAttachmentTar` (`lego/backend/internal/agentsessions/attachments_dispatch.go`) wrote only `<turn>/<att-id>-<name>` file entries. The gateway extractor requires each parent directory entry to come first, and allows no repeated names. Unit tests used a fake copier and never built a real archive.

**Fix.** Each `<turn>/` directory is written once, before its first file. A new `sandboxfiles.ValidateArchive` exposes the gateway's real extraction rules, and `TestAttachmentTarIsAcceptedByTheGatewayExtractor` builds the archive for interleaved turns and checks it against them. Mutation-checked: without the fix, the test fails with the production error.

**Next (t010).** After deploy, re-run the agent-reads-attachments check: a log and a PNG on create, plus a follow-up file, the agent quotes and describes them, and content downloads byte-exact.
