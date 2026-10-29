# Overlapping native file selections add a duplicate and block Save

**Severity:** minor. Rapid overlapping selections create an invalid draft with clear duplicate errors; no stored-data loss or failed server save is claimed. **Research:** main `0235ae759`, React/React DOM **19.2.3**. [Complete receipts, input recipe/hashes, controls and cleanup](evidence.json).

## Native reproduction

An owned, empty, unlinked group `qa-20261013-a362-upload` / `evg-db754dc479ic73bbj6q0` starts at revision `egr1_AAAAAAAAAAE`, with no variables/files/service links. The create-response capture was unavailable, was not retried, and the fresh route/heading plus independent empty-group read are retained.

1. Edit. Through the real multiple-file input, select **64 native files**, `qa-a362-00.txt` through `qa-a362-63.txt`, each **6,144 bytes**. Contents are the UTF-8 prefix `qa-a362-native-file-XX\n` followed by ASCII x bytes to that length. The batch is **393,216 bytes**.
2. Immediately select a second native file from another directory, also named `qa-a362-00.txt`, containing the **25-byte** text `qa-a362-second-selection\n`. Combined content is **393,241 bytes**, below the **512 KiB** aggregate limit; 65 rows remain below the 500-file limit and every file is below 1 MiB.
3. Desktop 1440px: first selection call returns **15:51:53.532Z**, second returns **15:51:53.537Z**. Settled observation **15:51:54.339Z** has **65 rows**, with `qa-a362-00.txt` twice, exactly two **Each secret file name must be unique** alerts and Save disabled.
4. Cancel, fresh-load at **390×844**, Edit and repeat. First/second returns **15:57:03.276Z / 15:57:03.282Z**; observation **15:57:04.134Z** repeats the same 65 names, two duplicate alerts and disabled Save, with viewport/page widths both 390.
5. At **15:59:45.794Z**, the independent saved group still has revision 1 and zero files. No failing Save was submitted. Viewed screenshot `.playwright-mcp/qa-upload-duplicate-a362-mobile-detail.png` (58,534 bytes) shows the owned group, duplicate error, 65 pending file operations and disabled Save. The complete filename arrays establish the duplicate; the screenshot shows one of the offending rows.

All reads are native File.arrayBuffer; no browser/file-read stubs, artificial latency or production request interception were used. The two input-change calls are 5–6ms apart after the first selection returns; this is evidence of the overlapping completion defect, not a measured human-picker frequency or a promise that every smaller batch reproduces.

## Controls and excluded observations

- Waiting for all 64 rows to appear before selecting the duplicate passes at **16:05:02.959Z**: 64 unique names remain, a skipped-file notice appears and Save stays enabled.
- Discard that whole draft, then upload only the 25-byte control file and Save only. At **16:07:42.046Z**, one exact file operation succeeds, revision 1 → 2, `rolledOut:false` and no affected/failed/pending services. Independent read at **16:07:43.684Z** returns the exact name/content.
- A 12 × 32 KiB rapid-selection control also skips the duplicate and remains saveable.
- The initial 12 × 128 KiB diagnostic exceeded the aggregate limit; it is excluded from this finding. No over-limit result is used to claim that valid uploads cannot save.
- A later harness cell referenced an unavailable local `files` variable before submitting its browser step. It changed nothing; the settled control was subsequently executed with the stored explicit path list.

## Cause, consumer and scoped correction

`dashboard/src/features/services/components/service-environment-editor.tsx:507–547` captures occupied trimmed filenames from the render's **draft** before any reads. It then awaits each native File.arrayBuffer. Two selections can start against the same empty draft and independently accept the same name.

At **:550–555**, the functional setter sees the updated **current** draft but unconditionally appends all previously accepted additions. It never checks those additions against names committed by the other batch. At **:557**, a separate uploadError setter also reports only the starting batch's rejected flag. The button/input at **:860–873** allow another selection while reads are outstanding. This is a stale duplicate decision, not a broken decoder.

The consumer correctly refuses this state: `environment-draft.ts:219–229` validates normalized active-row names and marks both duplicates; `isDraftValid` (:314–320) requires zero file errors. Preserve that guard. Existing count/aggregate limits are 500 files/512 KiB, and the decisive inputs satisfy them.

**Target behavior:** commit a decoded batch against the current active draft, atomically accepting only names not already present and retaining already accepted contents. Set the corresponding skipped-duplicate notice from that same decision. Keep every distinct decoded file, normalized identity, masked originals, deleted-name replacement and mixed valid/rejected behavior.

A concrete correction is to keep draft plus upload notice in a small **component-local state/reducer** and dispatch a pure completed-batch action. That action checks current names and returns the updated draft/notice together. Carry the active resource/edit identity so obsolete completions cannot enter a later edit; retain the existing form/save protocol and persist only the existing draft. Avoid a generic queue/state framework. This coordinates two existing UI fields without changing API or stored configuration types.

Do not invoke another setter from inside a functional updater or depend on a local rejected variable mutated by an updater. Installed React DOM `react-dom-client.development.js:7934–7936` runs functional actions against state. [React's useState reference](https://react.dev/reference/react/useState#set-functions-like-setsomethingnextstate) specifies queued pure updaters, including development purity checks. The [File API arrayBuffer algorithm](https://w3c.github.io/FileAPI/#dom-blob-arraybuffer) returns a promise after reading bytes; the live overlap follows that boundary. The standards draft is cited for this API algorithm, not as a new product requirement.

Caller census: **one production uploadFiles invocation** at `service-environment-editor.tsx:872`; **two EnvironmentEditor adapters**, service at **:124** and group at `env-group-editors.tsx:30`. The service route `services.$serviceId.env.tsx:58` also serves the static alias through `static.$serviceId.env.tsx:12–14`; the group route mounts its group adapter. New-service/new-group creation forms do not call this handler. Preserve their independent behavior.

## Required verification

Use real mounted editor interactions and File objects with controllable arrayBuffer promises in tests. Start two same-name batches from an empty draft, complete them in both orders and assert the final unique map, exact accepted contents, skipped notice and enabled Save. Nonoverlapping distinct files, one-batch duplicates, settled cross-batch duplicates, padded current names, masked originals and a deleted-name replacement remain controls.

Cover Cancel → new Edit, resource switch and unmount during unresolved reads, with obsolete results ignored. Those live failures were not attempted here. Verify both adapters, actual Save only payload/result and unchanged save/deploy/rebuild contracts. Existing upload tests at `service-environment-editor.test.tsx:543–566` resolve their file reads immediately, covering valid/refused staging but not overlapping completion. A pure-helper-only name test is insufficient.

[Render's Environment documentation](https://render.com/docs/configure-environment-variables#secret-files) establishes file configuration as a hosting workflow. Its authenticated overlap/duplicate behavior was not replayed; Bex's settled skip control and final unique-name rule establish this correction's target. No REST/GraphQL/MCP mutation or decoder-policy change is needed.

## Dedupe and prior guarantees

Whole-board open/blocked/done search found seven overlap/race/duplicate-upload text matches across six files. The current **60 active milestone titles**, last 40 product commits and target-file history were reviewed. No pending item owns the asynchronous occupied-name snapshot. DO_NOT_DO has no conflicting anti-goal.

- **w5/done/m44:** its complete DoD covers masked entry/Edit, bulk variable/file changes, import/upload/generate/export, whole-draft discard, three save effects, opaque unchanged values, linked groups, responsive/accessibility behavior, tests and durable docs. Upload and discard controls plus group Save only pass here; overlap is the uncovered case. Export/generation, service rollout/rebuild, link lifecycle, en/zh/keyboard completeness and full suites were not rerun. No whole-milestone regression claim.
- **w4/done/234:** its complete acceptance names an opaque file move plus replacement at the freed filename, CRLF/empty/Unicode values, unrelated-value preservation and a true-conflict refusal, with helper/adapter coverage. None of that combined backend batch was submitted here. This duplicate arises before Save from two independent native decodes; its map patch fix is not the owner.
- **w4/done/243:** fixes a deleted-row plus replacement protocol. This case has no deletion and no server refusal; validation is correctly blocking a newly duplicated draft.
- **w4/285:** fixes BOM decoding in the same uploader. These inputs contain no BOM and the issue is name admission after awaiting reads. Preserve that separately scheduled decoder correction.
- **w4/286:** fixes raw-versus-trimmed generated-variable allocation. Here current names are already trimmed; they become stale while reads overlap. No automatic variable name is generated.
- **w4/done/260:** stabilizes action-menu presentation through refresh. The file selections were accepted and completed; no menu closure causes this duplicate. Other search matches concern unrelated transfer mechanisms.

Live failure is an unlinked group only. Service/static adapters, other browsers, other completion orders, unresolved-read cancellation, content precedence and Render overlap are source-traced/test requirements, not independently failed product surfaces.

## Cleanup

The exact group delete succeeds at **16:09:31.179Z**. Its REST detail is 404 at **16:09:49.965Z**, and all seven inventory ID sets equal baseline. Own helper logout succeeds; at **16:12:40.667Z** owners returns 401, cookies are zero and the browser has one blank page. Exact handles were removed; the shared handle was removed only after matching its recorded identity. No foreign resource was changed. This hunt implements no product fix.
