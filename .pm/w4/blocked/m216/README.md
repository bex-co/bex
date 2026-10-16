# w4 · m216 — Keep deploy log search on the established literal contract

**Worker:** worker4 **Goal:** searching loaded deploy logs preserves the same Unicode lowercase/literal matches as service history **Status:** blocked — t001, t002, t004, t005 done 2026-10-16; t003 live replay and t006 need the release pipeline

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define a browser matcher for the backend's simple lowercase contract — **DONE** | 45m | — |
| t002 | Apply the matcher to both deploy entrypoints and verify reader controls — **DONE** | 40m | t001 |
| t003 | Render parity and live functional replay | 25m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 30m | t003, t004 |
| t006 | Closeout | 15m | t005 |

## Definition of done

- Recreate the owned Free fixture from [finding.md](finding.md). Its complete four-record second-deploy window is present before search. At1440px and after a fresh390px load, deploy search `QA_A743 οσ` returns uppercase `ΟΣ` and lowercase `οσ`; `QA_A743 ος` returns final-sigma `ος` alone; `qa_a743 alpha` returns `Alpha` alone.
- Repeat the captured same-window GraphQL requests with display mode: they remain200/no-errors with counts2/1/1, unchanged original messages and completed coverage. Service Logs continues to show the same casing selections. No backend/global-fold rewrite is part of this repair.
- A reproducible Go-derived mapping fixture and browser matcher agree on simple lowercase, including dotted-İ, Kelvin, Cyrillic, supplementary letters and identity for characters outside the pinned table. Contextual final sigma does not replace ordinary sigma during matching. Keep literal punctuation and whitespace, original rows/keys and current ANSI display behavior.
- Replay the existing passing ASCII control and verify no-search/type/instance, loading/error and loaded-window/pagination behavior on the changed component. Source-check both service/static route aliases and the full seven-type family; live surfaces not exercised remain explicit verification gaps.
- Delete every owned fixture and its child deploys; confirm API/public404, original resource ID sets and exact-session logout before closeout. The filing's cleanup is evidence for this hunt, not a substitute for implementation acceptance.

## Source + Goal linkage

- **Source:** ordinary functional QA a743,2026-10-16; [researched finding](finding.md), [complete native/API evidence](evidence/capture.json), source synchronized to f317cd712. User requested w4 and shipping the filing while the QA loop continues.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [ADR010](../../../../docs/ADR010-observability.md) and [ADR018](../../../../docs/ADR018-render-parity.md): users and agents must find their actual application records reliably.
- **Expected outcome:** deploy-local filtering returns the backend's existing simple-lowercase literal result set for the same loaded records.
- **Why now:** the mismatch repeats on desktop and a fresh mobile page, independently of ingestion, ANSI interpretation, time bounds or paging. Backend controls pass; the remaining client predicate is isolated.
- **Sizing:** more than one hour across mapping compatibility and reader integration/verification; the Go/browser Unicode-table difference needs an explicit contract and drift fixture.
- **Parity:** included because a tenant-facing search changes. Preserve REST/GraphQL/MCP/CLI contracts; document Render comparison limits and deliberate literal-search divergence.

## Scope

Ship the filing only in this QA run. Implementation belongs to the scheduled work. Keep the repair within deploy displayed-text matching; no product-wide lowercase rewrite, backend/CLI semantic change, parser changes from w4/327, paid fixture, external drain or new search syntax.

## Implementation record (2026-10-16)

- **t001:** `lego/backend/internal/logs/testdata/lowercase-runs.json` holds Go's simple lowercase mapping as 186 `[start, end, stride, delta]` runs (1,488 mappings, Go 1.26 / Unicode 15.0). `TestLowercaseRunsMatchGo` regenerates it with `BEX_UPDATE_LOWERCASE_RUNS=1` and pins it to `unicode.ToLower` over every scalar value. The dashboard's `goLowercase` (`dashboard/src/features/logs/lib/go-lowercase.ts`) maps code point by code point through the same runs. Its test pins the TS table to the JSON and checks Σ→σ (no final sigma), İ→i, Kelvin, Cyrillic, Deseret, ǅ, identity for unmapped text and punctuation, and unchanged code-point length.
- **t002:** `DeployLogPanel`'s displayed-text filter folds the needle and `stripAnsi(message)` with `goLowercase`. Its one caller (`DeployDetailPage`) serves both the service and static deploy routes. The type/instance predicates, debounce, ANSI display, original rows and paging are unchanged.
- **t003 (parity part):** ADR010 records the shared fold. Server contracts and other dashboard searches are unchanged. The live a743 replay is outstanding.
- **t004/t005:** a one-helper change, reviewed inline. The panel test runs the a743 terms (`QA_A743 οσ` → ΟΣ+οσ, `QA_A743 ος` → ος, `qa_a743 alpha` → Alpha); both sigma cases fail with the old `toLowerCase` filter, and the ASCII control passes either way. Dashboard `yarn test` (5,044) and `yarn lint` pass, as do the backend logs package and lint.
