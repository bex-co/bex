# a861 — Obsolete dotenv file reads replace newer choices and survive Cancel

**Severity:** major (the wrong configuration can be saved and deployed). **Observed:** 2026-10-17 UTC. **Research:** `b467887eb4b276a404d576268130d33b1fa8cea4`; importer/caller blobs unchanged after fetching `3d73a5515bcc0f9e9e1ed6b42056396cdabaa450`. Filing only.

## Reproduce

Use the owned Free image-web fixture and two small synthetic files in [evidence.json](evidence.json). The exact replay factories call the original native File.text and delay only the first file's completion; file bytes and backend responses are unchanged. This is controlled latency, not a measured natural frequency.

1. Environment → Edit → Add variable → Import from .env. Select first.env, hold its read, then select second.env. In the primary desktop/mobile replays the selections are over 1.2s apart. The preview first correctly shows second. Release first: the preview becomes first. **Add variables** imports the obsolete first values.
2. In a separate dialog, select first.env and hold its read. **Cancel** closes the dialog. Release the read only after closure, then reopen Import: the canceled file reappears. An ordinary second Cancel/reopen with no outstanding read produces the expected empty dialog.
3. Both patterns reproduce at 1440/390px, including fresh-page replay. Every primary receipt has trusted native file-change events, actual read/return ordering and zero API mutations. The first desktop attempt used a roughly 7ms selection gap; it is supporting evidence. That initial case was followed through one real Save and deploy, not merely a preview comparison.

The real `PatchServiceEnvironment` request sends source/text from first.env although second.env was the latest selection. The complete response reports a rollout. A separate four-key GraphQL read and public HTTPS200 both return `source:"first", text:"first 雪"` at rev-2. A normal import of second.env then sends the correct values, reaches rev-3 Live, and fresh stored/public responses return second. The initial service response contained null for all four selected test keys.

The viewed paced desktop screenshots `.playwright-mcp/qa-a861-reorder-paced-1440-before.png` and `…-after.png` show second then first. They are local aids; tracked scripts, complete requests/responses and public bodies survive in evidence.json.

## Root cause and target behavior

- `dashboard/src/features/services/components/env-import-dialog.tsx:68–79` awaits each File.text independently, then unconditionally writes text/error. There is no selection generation or dialog-session ownership check, so completion order wins over user intent.
- `:52–56` clears current text on close but does not invalidate the outstanding callback. Its later setText repopulates the component state before reopening.
- The textarea's `:102–106` edit handler also lacks read invalidation. Submit at `:58–65` parses the current text; its button at `:156` considers only text emptiness. Cmd/Ctrl+Enter calls the same submit function at `:109–114`. Pending-replacement submission and intervening manual edits are source-traced adjacent audit work, not additional live failures.
- The consumer is real: `service-environment-editor.tsx:466–495` upserts imported entries into its active draft; `use-environment-draft-save.ts` forwards the patch. The two native writes and independent runtime controls demonstrate that the API correctly stores the values the UI actually sends. No API/schema change is required.
- Opened the primary [File API text algorithm](https://w3c.github.io/FileAPI/#dom-blob-text): it returns a promise after reading/decoding bytes. HeadlessChrome 155 was observed. The application must ignore obsolete completions; this is not a browser or parser defect. Do not pretend AbortController cancels File.text.

**Target:** the active dialog's most recent selected file or explicit textarea edit owns its preview and imported entries. An older success or failure cannot change newer content/status. Close/Cancel invalidates the read, so reopening stays empty. Track pending ownership locally and keep submit—including the keyboard shortcut—from committing an earlier preview while a replacement read is unresolved. Keep Cancel available; a later explicit text edit must supersede that read. Use existing error messages and localized reading feedback as needed, without changing parser or backend contracts.

## Shared scope and controls

Production grep finds **one EnvImportDialog definition and two direct JSX sites**:

- `service-environment-editor.tsx:998` inside EnvironmentEditor, whose two adapters are ServiceEnvironmentEditor (`:129`) and EnvGroupEditors (`env-groups/components/env-group-editors.tsx:30`). Their hosts are the service Environment page and existing-group detail.
- `new-env-group-dialog.tsx:626` inside the creation form. NewEnvGroupDialog has two direct mounts: `routes/env-groups.tsx:124` and `services/components/env-groups-panel.tsx:388`.

The service route has services/static entries and web/worker/pserv/cron/static Render aliases. All five App kinds reach the shared service host by source; Postgres/Key Value do not call this importer. Only an owned Free image **web** service was live-driven. Group create/edit, other App types and aliases are dedicated audit/test work, not observed failures. Apply the local importer correction to both direct consumers; leave global Input, the parser, secret-file upload and structured REST/GraphQL/MCP inputs unchanged.

Passing controls were exercised as hard as the failure:

- A valid first assignment followed by malformed line2 raises the correct inline error, retains the exact text and imports no partial row; Cancel leaves only the existing KEEP draft.
- A normal second.env upload preserves its exact Unicode/empty values and merges into the existing draft. Recovery Save and deploy uses the existing image, with three total deploy rows across create, wrong-value save and recovery.
- KEEP's initially empty textarea after a fresh load is **masked/unchanged**, not cleared data. The recovery request omits KEEP; both independent reads and public responses retain `keep-unchanged`. Empty imported values remain empty strings.
- The standard beforeunload guard appeared during reload and was accepted. The page can restore an editor draft; the import dialog itself starts empty. No claim is made about whole-editor discard semantics.

## Dedupe, parity and limits

Reviewed 89 canonical open/blocked milestone titles, whole-board terms including done work, DO_NOT_DO, recent 40 product commits and targeted importer history. Two nested evidence READMEs are excluded from that count. The importer last changed at 29222fd52 for w4/159; no current selection/cancellation fix has landed.

**m205 is different:** it handles secret-file File.arrayBuffer batches, committing unique filenames against current editor state with a parent editor session. That policy keeps already accepted file content. This single dotenv preview must instead keep the latest selection. m205's upload-lifecycle DoD does not cover EnvImportDialog's independent File.text callback. Preserve its existing correction. w4/273/159 own parser whitespace/multiline/error semantics; their ordinary import/error controls pass here. m219 owns SSR readiness, whereas these dialogs have active handlers. m161 owns group save revisions, which are not involved in this one-service local race.

[ADR008](../../../../docs/ADR008-vision.md), [ADR013](../../../../docs/ADR013-secrets.md) and [ADR018](../../../../docs/ADR018-render-parity.md) establish reliable hosting configuration. [Render's primary docs](https://render.com/docs/configure-environment-variables#adding-in-bulk-from-a-env-file) describe valid dotenv bulk import and saving/deploying the resulting values. Authenticated Render file-read ordering/cancellation was not measured; public docs do not establish it.

Unverified live: group hosts, other App types/aliases, same-name files, manual editing during a read, stale errors, size/read rejection, external parent close/unmount and pending submit. The tests/audit must place these neighbors explicitly. No new authorization or not-found/error taxonomy is needed. No paid fixture, foreign mutation or security test was performed.

The timing probes restore the native function in finally; closeout also removed its own native-function shadow property. No held read/override remains. The wrong runtime proof follows the rapid supporting case; paced cases independently prove the same wrong preview/imported draft but were not separately deployed.

## Cleanup

Native service delete succeeded at 06:43:20 UTC. All four service/deploy reads return 404 and all seven baseline ID sets match (06:43:34). Own session revoked: whoami 401, cookies 0, one blank tab, own handles removed (06:43:42). Exact App plus serving/build/publish/purge/predeploy children are absent across namespaces (06:44:06), and the public URL is 404 with complete headers/body retained. No fixture or session remains.
