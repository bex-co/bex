# w4 · m222 — Ignore obsolete dotenv file reads

**Worker:** worker4 **Goal:** an active dotenv import follows the latest user choice, and canceled reads cannot repopulate a later dialog. **Status:** done 2026-10-29 — live DoD replayed on pin `d66f96c06991`: the latest selected .env file owns the import preview even if an older read finishes later, Cancel discards a pending read, and Save and deploy writes the second file's values (loopx w4).

**Estimate:** 2h15 (6 tasks). **Severity:** major. Implemented 2026-10-17; awaiting the deployed-build replay (t006).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Own pending dotenv reads and invalidate canceled work — **DONE** | 35m | — |
| t002 | Audit both importer consumers and preserved controls — **DONE** | 20m | t001 |
| t003 | Render parity — **DONE** | 15m | t002 |
| t004 | Simplify — **DONE** | 10m | t003 |
| t005 | Test ordering, cancellation and pending-read neighbors — **DONE** | 40m | t003 |
| t006 | Replay owned runtime proof and close out | 15m | t004, t005 |

## Definition of done

- Recreate the owned Free web fixture and synthetic first/second files from [evidence.json](evidence.json). Repeat the paced 1440/390 file-order probes: after second is selected and previewed, releasing first leaves the exact second content. Add variables retains second's Unicode and empty values.
- Repeat the recorded Cancel probes at both widths: close while first is pending, then release it; reopening is empty. The ordinary no-pending Cancel/reopen control remains empty. Import-only probes create no API writes.
- Save and deploy the intended second values. The native patch, independent four-key read and public HTTPS200 agree on source second/text second 雪; KEEP is preserved and EMPTY remains an empty string. Preserve the recorded normal import, malformed-line atomicity and existing-image deployment controls.
- Delete the owned service; service/deploy reads and public host return 404, all seven starting inventories match, exact App/serving/build children are absent and only the sweep session is revoked.
- t002/t005 explicitly verify the source-enumerated other hosts and lifecycle neighbors. They are not additional observed failures. Close only after the fixed-build evidence and required dashboard checks exist.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs w4`, a861, 2026-10-17 UTC; [finding](finding.md), [exact scripts and complete safe receipts](evidence.json).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) reliable hosting, [ADR013](../../../../docs/ADR013-secrets.md) configuration delivery and [ADR018](../../../../docs/ADR018-render-parity.md) existing Environment UI parity.
- **Expected outcome:** older file-read success/error callbacks cannot overwrite later selection, manual text or cancellation; saved runtime configuration follows the intended file.
- **Why now:** two independent controlled-latency failures reproduce at desktop/mobile; a real native save plus stored/public reads proves the wrong file can reach the running process. Normal import/recovery pass.
- **Sizing:** read ownership/pending UI, two-consumer nested-host audit and meaningful asynchronous component regressions exceed one hour; standing parity/review/live closeout are included.
- **Scope:** one shared dialog, two JSX sites, two EnvironmentEditor adapters and two NewEnvGroupDialog mounts. No parser, secret-file uploader, global Input or API contract change.
- **Dedupe:** 89 canonical open/blocked milestones, whole-board terms, anti-goals and history checked. m205 owns a different secret-file batch reader/merge policy; 273/159 own parsing. See the finding for limits and passing controls.
- **Render parity:** included because import behavior is user-facing. Primary bulk-import docs were read; authenticated asynchronous behavior remains unmeasured. Structured REST/GraphQL/MCP saves keep their contracts.

## Implementation record (2026-10-17)

- **t001:** `EnvImportDialog` renders its state in an inner `EnvImportForm` inside `DialogContent`, which Radix unmounts however the dialog closes (Cancel, Escape, outside click, or the parent forcing `open=false`). Each opening starts empty and a read still in flight lands nowhere. Within one session a generation ref gives ownership to the latest file choice or textarea edit: an earlier `File.text` success or failure that completes later is ignored (File.text cannot be aborted). While a replacement file is being read, Add and Cmd/Ctrl+Enter wait and a localized "Reading file…" status shows; after the latest choice fails (oversized or unreadable), they do not import the earlier preview until an explicit edit takes ownership back. Cancel stays usable. The parser, the size limit, input reset for re-choosing the same file and side-effect-free draft staging are unchanged.
- **t002 census:** one definition, two direct JSX sites: `service-environment-editor.tsx` (EnvironmentEditor, behind ServiceEnvironmentEditor for the service Environment page of all five App kinds and their Render aliases, and EnvGroupEditors for group detail) and `new-env-group-dialog.tsx` (mounted from `/env-groups` and the service env-groups panel). Both pass `open`/`onOpenChange`/`onImport` unchanged; the dialog still closes itself after an import. Postgres/Key Value do not use the importer. Only the owned web service was live-driven by a861; the rest is source-traced.
- **t003:** ADR018's env-vars row records the ordering, pending and failed-choice rules; parsing and structured REST/GraphQL/MCP saves are unchanged; Render's asynchronous import behavior was not measured.
- **t004:** three /simplify passes (failed latest choice now blocks importing the earlier preview; comment wording).
- **t005:** `env-import-dialog.test.tsx` uses controllable `File.text` promises under a parent that owns `open`: later file wins when the earlier read finishes last (Unicode-safe, empty value kept) or fails last; Cancel and a parent-forced close while pending reopen empty; a pending replacement blocks Add and Ctrl+Enter then imports the replacement; a text edit supersedes a pending read; an oversized later choice blocks the earlier preview until an edit. Six of these fail against the old dialog. The w4/159 parser cases pass unchanged; editor and group suites pass. Dashboard suite (5155 tests), lint and typecheck pass.

## Live gate recheck (2026-10-17, a899)

**Still blocked; t006 remains open.** The owned Free web replay repeats the paced1440/fresh390 ordering and pending-Cancel failures, with no API mutation during those probes. The loaded `index-eS0UcioS.js` (SHA256 `39e3781b6d73744c23947b3fd8b876f09c6eb2861213ab4200f532cf451905fe`) contains the old single-form unguarded File.text callback, while main retains the generation/unmount fix. No serving commit is inferred. The normal second-file import/save/deploy succeeds and stored/public values agree at rev-2, but it does not satisfy the failed ownership/cancellation gate. Original evidence.json is unchanged. [Complete replay receipts, source/asset distinction and cleanup](../../evidence/337-restored-draft-copy-a899.json). The same sweep files a separate recovery-copy issue in337; no duplicate importer task was created. The owned service/two deploys and all23 runtime/snapshot scopes are absent, inventories restored and exact session revoked.
