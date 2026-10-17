# w4 · m222 — Ignore obsolete dotenv file reads

**Worker:** worker4 **Goal:** an active dotenv import follows the latest user choice, and canceled reads cannot repopulate a later dialog. **Status:** todo

**Estimate:** 2h15 (6 tasks). **Severity:** major. Filing only; no product fix included.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Own pending dotenv reads and invalidate canceled work | 35m | — |
| t002 | Audit both importer consumers and preserved controls | 20m | t001 |
| t003 | Render parity | 15m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test ordering, cancellation and pending-read neighbors | 40m | t003 |
| t006 | Replay owned runtime proof and close out | 15m | t004, t005 |

## Definition of done

- Recreate the owned Free web fixture and synthetic first/second files from [evidence.json](evidence.json). Repeat the paced 1440/390 file-order probes: after second is selected and previewed, releasing first leaves the exact second content. Add variables retains second's Unicode and empty values.
- Repeat the recorded Cancel probes at both widths: close while first is pending, then release it; reopening is empty. The ordinary no-pending Cancel/reopen control remains empty. Import-only probes create no API writes.
- Save and deploy the intended second values. The native patch, independent four-key read and public HTTPS200 agree on source second/text second 雪; KEEP is preserved and EMPTY remains an empty string. Preserve the recorded normal import, malformed-line atomicity and existing-image deployment controls.
- Delete the owned service; service/deploy reads and public host return 404, all seven starting inventories match, exact App/serving/build children are absent and only the sweep session is revoked.
- t002/t005 explicitly verify the source-enumerated other hosts and lifecycle neighbors. They are not additional observed failures. Close only after the fixed-build evidence and required dashboard checks exist.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs w4`, a861, 2026-10-17 UTC; [finding](finding.md), [exact scripts and complete safe receipts](evidence.json).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) reliable hosting, [ADR013](../../../docs/ADR013-secrets.md) configuration delivery and [ADR018](../../../docs/ADR018-render-parity.md) existing Environment UI parity.
- **Expected outcome:** older file-read success/error callbacks cannot overwrite later selection, manual text or cancellation; saved runtime configuration follows the intended file.
- **Why now:** two independent controlled-latency failures reproduce at desktop/mobile; a real native save plus stored/public reads proves the wrong file can reach the running process. Normal import/recovery pass.
- **Sizing:** read ownership/pending UI, two-consumer nested-host audit and meaningful asynchronous component regressions exceed one hour; standing parity/review/live closeout are included.
- **Scope:** one shared dialog, two JSX sites, two EnvironmentEditor adapters and two NewEnvGroupDialog mounts. No parser, secret-file uploader, global Input or API contract change.
- **Dedupe:** 89 canonical open/blocked milestones, whole-board terms, anti-goals and history checked. m205 owns a different secret-file batch reader/merge policy; 273/159 own parsing. See the finding for limits and passing controls.
- **Render parity:** included because import behavior is user-facing. Primary bulk-import docs were read; authenticated asynchronous behavior remains unmeasured. Structured REST/GraphQL/MCP saves keep their contracts.
