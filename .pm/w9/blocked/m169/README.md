# w9 · m169 — ADR095 phase 2 — attachments in the `/agents` dashboard

**Worker:** worker9 **Goal:** a dashboard user can paste, drop or pick files when starting or steering a session and sees them in the conversation. **Status:** blocked — t001, t002, t004–t006 done; t003 live check + t007 closeout wait for the production attachments bucket (see Blocked)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Upload client and attachment chips in both composers — **DONE** | 90m | — |
| t002 | Render user-turn attachments: thumbnails, file cards, lightbox — **DONE** | 60m | t001 |
| t003 | Live dashboard verification at desktop and narrow mobile — **BLOCKED** (production bucket) | 40m | t002 |
| t004 | Render parity (Devin reference) — **DONE** | 25m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004, t005 |
| t007 | Closeout — **BLOCKED** (production bucket) | 10m | t006 |

## Definition of done

- Both composers accept picker, drag-and-drop and pasted clipboard images; chips show progress, errors in the server's words and removal; send is disabled while uploading; limits are read from capabilities.
- User turns show raster thumbnails (lightbox) and download cards for every other type; SVG/HTML/PDF never render inline.
- Verified live at desktop and narrow-mobile widths with screenshots in `evidence/`.
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
