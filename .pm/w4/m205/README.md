# w4 · m205 — Keep overlapping file uploads unique

**Worker:** worker4 **Goal:** overlapping secret-file selections produce a saveable draft with unique filenames and an honest skipped-duplicate notice **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Commit decoded files and duplicate feedback against current editor state | 35m | — |
| t002 | Preserve shared editor lifecycle and upload neighbors | 20m | t001 |
| t003 | Render parity | 15m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 35m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

- On an owned unlinked group, the [recorded valid 64-file batch](finding.md) followed by its overlapping same-name selection leaves **64 unique names**, no duplicate-name validation error and Save enabled. A file already accepted into the active draft keeps its contents; the arriving duplicate is skipped with the existing notice. Distinct incoming files survive either completion order.
- Waiting for the first batch to settle before selecting the duplicate retains its existing successful skip behavior. Single-file Save only succeeds, stores exact decoded content and causes no service rollout.
- A completion applies only to its active resource/draft. Cancel, a later edit, resource changes and unmount do not apply obsolete results. Preserve masked existing files, deleted-name replacement, name normalization, format/size guards, mixed rejected/accepted selections and revision/save-mode behavior. Lifecycle cases are required tests, not claimed live failures.
- Both shared adapters use the same correction; service/static/group route aliases are reviewed. Actual component tests deterministically control file-read completion order and fail on the current implementation. Updaters/reducers remain pure.
- Required dashboard checks pass. Fresh desktop/mobile owned-fixture replay records the final draft and saved control, followed by exact fixture deletion, seven inventory checks and own-session revocation.

## Source + Goal linkage

- **Source:** continuous user-requested functional `$qa-find-bugs w4`, production pass a362, 2026-10-13 UTC; [finding and complete safe evidence](finding.md).
- **Goal linkage:** ADR013 hosting configuration, ADR018 Environment parity and ADR008 dependable core hosting.
- **Expected outcome:** file decoding can overlap without the uploader creating a draft its own Save refuses.
- **Why now:** native desktop and fresh 390px selections within all size/count limits produce duplicate names and disabled Save; settled selections and a normal save pass.
- **Sizing:** 130m across coordinated draft/notice state, two shared adapters, asynchronous lifecycle checks, parity, Simplify and live closeout. This is over an hour and requires more than one task.
- **Render parity included:** user-facing upload behavior. Preserve the existing REST/GraphQL/MCP save contracts; authenticated Render overlapping-selection behavior remains unverified.
- **Scope:** filing only in this hunt. No product implementation, API change, decoder/BOM fix or backend replacement-protocol work.
