# w9 · m169 — ADR095 phase 2 — attachments in the `/agents` dashboard

**Worker:** worker9 **Goal:** a dashboard user can paste, drop or pick files when starting or steering a session and sees them in the conversation. **Status:** todo — t001, t002, t004–t006 done; t003 live check once m168 has provisioned the bucket

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Upload client and attachment chips in both composers — **DONE** | 90m | — |
| t002 | Render user-turn attachments: thumbnails, file cards, lightbox — **DONE** | 60m | t001 |
| t008 | Allow the attachments storage origin in the dashboard CSP img-src — **DONE** | 20m | t006 |
| t003 | Live dashboard verification at desktop and narrow mobile | 40m | t002 |
| t004 | Render parity (Devin reference) — **DONE** | 25m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- Both composers accept picker, drag-and-drop and pasted clipboard images; chips show progress, errors in the server's words and removal; send is disabled while uploading; limits are read from capabilities.
- User turns show raster thumbnails (lightbox) and download cards for every other type; SVG/HTML/PDF never render inline.
- Verified live at desktop and narrow-mobile widths, recorded as text in this README (screenshots, if any, go to object storage with sha256 + link — never into git, `.pm/AGENTS.md`).
- Dashboard `yarn test`, typecheck and lint green.

## Source + Goal linkage

- **Source:** [ADR095](../../../docs/ADR095-agent-attachments.md) (accepted for implementation by user 2026-11-03: "/pm ADR095 to w9"); Devin gap list in [ADR094](../../../docs/ADR094-coding-agent-mcp.md).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 5 (cloud coding agents); Devin parity for agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)).
- **Expected outcome:** the "paste a screenshot and say fix this" flow works in the product UI, matching Devin's web app.
- **Why now:** depends on m168's API; ADR095 D12 phase 2.
- **Dependency:** m168 (REST upload, GraphQL attachment fields, capabilities.attachments).
- **Parity:** included: dashboard surface over the m168 API; Devin web app is the reference.

## Outcome (2026-11-03)

t001, t002 and t004–t006 are done.

**Composers.** Both composers accept the paperclip, drag-and-drop (with a drop overlay) and paste, and show chips with progress, a typed error and removal.
- Send waits for uploads and shows a "waiting for uploads" hint.
- Paste, drop and the paperclip are disabled while sending.
- A file added mid-send survives `clear`, and a removal that races a finished upload deletes the file.
- Limits come from the session workspace's capabilities. The paperclip is hidden when attachments are disabled, so nothing changes in production until the bucket exists.

**User turns.**
- Raster images render as thumbnails with a lightbox and repeatable URL renewal, falling back to "unavailable" when the query settles empty.
- Every other type (SVG, HTML, PDF…) is a download card and is never inlined.

**Parity (t004), Devin web app as the reference.** The composer attach, paste and drop flow and the user-turn file display match Devin's web app. The dashboard uses the m168 API: REST upload and delete, GraphQL `agentAttachment.downloadUrl`, `attachmentIds` on create and steer. No drift.

**Simplify (t005).** Two reviewers; the findings were applied:
- the race fixes;
- memoized capabilities;
- progress throttled to whole percents;
- unmount cleanup;
- typed and translated upload errors;
- one `AttachmentView` type;
- a11y status and description text;
- mixed-content paste keeps the text.

Not done: moving chip state below the composer (typing still re-renders the chips), and showing chips in the optimistic steer echo.

**Verification.** `yarn lint` (typecheck + eslint + knip) and `yarn test` pass: 590 files, 5,785 tests.

## Blocked (2026-11-03): t003 live verification and t007 closeout

**Why it couldn't run here.** The local kind/CAPD cluster is down. Its containers exited with 137, and the app cluster's API server did not recover after a restart. A full local run would also need MinIO and a model key.

**Gate.** The live check runs on production once the attachments bucket exists. That is the same user action as [m168](../m168/README.md): `bash scripts/agent-attachments-s3-credentials.sh provision && … install`.

**To clear.** Create the bucket and deploy, then `/loopx w9` verifies at desktop and narrow-mobile widths as the QA user and saves screenshots under `evidence/`.

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

## Live acceptance evidence (2026-11-04, production dashboard, QA user)

Screenshots were removed from git in the 2026-11-04 history rewrite; the text above is the record.

**Passed**
- **Composer.** The paperclip appears once capabilities load.
  - Choosing two files shows chips with name and size, and Start session is enabled (desktop screenshot (removed from git in the 2026-11-04 history rewrite; text above is the record)).
  - At 390 px wide, a pasted clipboard PNG becomes a chip (mobile screenshot (removed from git in the 2026-11-04 history rewrite; text above is the record)). Removing it sends `DELETE` → 204, and the chip disappears.
- **Follow-up composer.** It attaches a file, and the file binds to turn 2 (desktop screenshot (removed from git in the 2026-11-04 history rewrite; text above is the record)).
- **User turns.** Desktop and mobile both render the attachment row, with a download card for the log and no horizontal overflow (desktop screenshot (removed from git in the 2026-11-04 history rewrite; text above is the record), mobile screenshot (removed from git in the 2026-11-04 history rewrite; text above is the record)).

**FAILED: raster thumbnails do not load in production.** The dashboard CSP is `img-src 'self' data: https://*.stripe.com` (`dashboard/vite.config.ts:356`). The browser blocks the presigned `https://s3.eu-central-2.wasabisys.com/...` URL, so the thumbnail and lightbox show a broken image. Downloads still work, because they navigate.

**Fix.** Add the attachments storage origin (`https://s3.eu-central-2.wasabisys.com`, or derive it from config) to `img-src`, then re-verify a thumbnail loads.
