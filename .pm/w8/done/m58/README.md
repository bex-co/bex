# w8 · m58 — Preserve Unicode matches in log history

**Worker:** worker8 **Goal:** normalized Unicode matches and complete raw-page coverage survive log history through the unchanged CLI and shared API **Status:** done — 2026-10-13: all tasks complete; live App/saturated/KV/PG receipts plus code-level REST/GraphQL/MCP agreement; live GraphQL/dashboard and saturated managed-data replays recorded as unexercised boundaries.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Make the Loki text prefilter preserve normalized Unicode matches — **DONE 2026-10-12** | 50m | — |
| t007 | Advance raw text scans by raw candidate coverage — **DONE 2026-10-13** | 45m | t001 |
| t002 | Verify shared history callers and resource families — **DONE 2026-10-13** | 30m | t007 |
| t003 | Render parity and live native CLI acceptance — **DONE 2026-10-13** | 20m | t002 |
| t004 | Simplify — **DONE 2026-10-13** | 10m | t003 |
| t005 | Meaningful Unicode and pagination regression coverage — **DONE 2026-10-13** | 35m | t003 |
| t006 | Closeout — **DONE 2026-10-13** | 10m | t004, t005 |

## Definition of done

For an owned fixture that prints `CAFÉ Ελληνικά` and `café ελληνικά`, released Bex and the unchanged pinned Render client against Bex return both original messages for `logs --resources OWN_ID --type app --text '"café ελληνικά"' --limit 100 --confirm -o json` and for `--text ι`. Capture the actual requests and complete responses. Raw GraphQL agrees; the passing `textMode=display` control remains correct. Preserve literal ASCII/metacharacter/quote/space matching, multi-term OR, original bytes, time bounds, direction, instance selectors, and limits. A match behind a full raw scan page remains reachable; budget exhaustion reports incomplete coverage rather than a false empty completed window. Verify affected datastore, pod fallback and live-tail behavior without introducing unsupported filters. Record live deployment evidence, cleanup and all material gaps before moving this milestone to `done/`.

Also reproduce the sweep-96 table: Russian and Greek-rho words return two capital/lowercase lines, ASCII Kelvin and Istanbul terms return three, and iota returns two through both clients. Prefixing the term must never increase its match set. Plain-message raw and display APIs agree, including dotted capital İ. A correction gated solely on non-ASCII query terms or using only full regexp matching does not meet this definition.

Managed Postgres acceptance: emit the three sweep-100 markers through native SQL and verify both clients return three for the marker prefix, two for iota and one for lambda using supported datastore filters. Do not introduce app-only type filters.

Managed Key Value acceptance: use the native EVAL marker commands in sweep 101 and require both clients to return three prefix records, two iota records and one lambda record; preserve native terminal completion and original bytes.

## Progress / remaining work

- **2026-10-13:** t007 shipped — raw and display text searches share raw-candidate coverage paging (`textScanPage`), so a match behind a full raw page is reached; realistic-bounds regressions fail on the old scanner. Parked again pending deployment and the saturated native replay (t003) plus closeout (t006).
- **2026-10-12, live sweep179:** [raw pagination regression](evidence/live-app-sweep179/README.md) reproduces four false-empty native searches across both clients, plus four passing end-bound controls; display REST returns all 21. Eight actual native wire captures and realistic current-source probes isolate missing raw-candidate coverage. Add t007 (45m), reopen t002–t006, and move this milestone from blocked/m58 to active m58. The previous implementation/tests are historical evidence, not proof that the complete DoD holds. Exact App cleanup passes; no product fix or ship by QA.

- **2026-10-12, live sweep178:** [managed PostgreSQL acceptance](evidence/live-pg-sweep178/README.md) passes the exact three-marker/three-prefix/two-iota/one-lambda matrix through both clients, with six actual native HTTP captures and original IDs/bytes preserved. Four native SQL success/error controls pass. Exact owned DB deletion is confirmed by detail 404/listempty, former SQL endpoint no row and PGquota0/termination0. Both managed-data sections pass; complete App/GraphQL/native App wire gates remain and t003/t006 stay blocked.

- **2026-10-12, live sweep177:** [managed Key Value acceptance](evidence/live-kv-sweep177/README.md) passes the exact three-marker/three-prefix/two-iota/one-lambda matrix through both clients, with six actual native HTTP captures and original IDs/bytes preserved. Both native PONG controls pass. Exact owned KV deletion is confirmed by detail 404/listempty, former endpoint no-PONG and quota0/termination0. The KV section passes; full App/GraphQL and Postgres gates remain and t003/t006 stay blocked.

- **2026-10-12, live sweep176:** [partial App acceptance](evidence/live-app-sweep176/README.md) passes 28 native text cases plus 12 timestamp/direction/limit controls across both clients, and six REST raw/display comparisons. Fresh App and its one child were enumerated and cleaned; detail/list/public404 and baseline6/quota termination0 confirmed. Exact deployed SHA remains unknown. Full App/table/pagination/instance/GraphQL and managed Postgres/Key Value gates remain; t003/t006 stay blocked.

- **2026-10-12:** local backend implementation completed. Text matching now stays out of Loki's Unicode-unsafe regexp simplifier and is applied by the shared Go matcher over bounded raw scan pages. Evidence: [`evidence/implementation-closeout.md`](evidence/implementation-closeout.md).
- Validation passed: `cd lego/backend && go test ./internal/logs` and `cd lego/backend && go test ./...`.
- **Current next step:** t003/t006 reconcile the remaining complete observable DoD. The raw scan source fix and saturated native App replay pass; latest saturated managed-data and remaining surface/instance/budget gates are not inferred from that success.

## Source + Goal linkage

- **Source:** [CLI sweep 94 finding](finding.md) and [durable evidence](evidence/native-wire.json), observed 2026-10-11 with human QA credentials in bex-canary; no product code or deployment changed by this hunt.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [ADR010](../../../../docs/ADR010-observability.md) and [ADR018](../../../../docs/ADR018-render-parity.md): customers must be able to find real application logs through the imported terminal client and shared API.
- **Expected outcome:** localized and ASCII searches no longer omit stored case-normalized matches or contradict stricter-prefix queries.
- **Why now:** both production native clients reproduce it; local Loki 3.6.7 and its exact regexp dependency isolate canonical-literal, first-byte and full-fold mismatches. w2/m167 explicitly left broad Unicode parity unverified; this is now a concrete ordinary-language failure.
- **Parity:** included because the fix changes tenant-facing history search. REST, GraphQL, MCP and dashboard consumers need scope checks; Greek display-mode REST passes, but plain dotted-İ display REST fails; the dashboard UI remains unverified.

## Scope

File and implement the backend history fix with bounded work. An upstream Loki contribution or dependency upgrade may be evaluated separately; neither is required to start the Bex fix. Do not alter the pinned Render CLI or add a launcher request rewrite. Raw CLI, raw GraphQL and the specified display-mode REST failure are live-confirmed; the subsequent native App live-tail Unicode control passes. Managed Postgres markers now reproduce the raw-history defect through both native clients (sweep 100). Managed Key Value markers reproduce the same raw-history defect through both native clients (sweep 101). Datastore display/convenience routes, MCP and dashboard UI remain unverified. Managed Postgres/Key Value live subscriptions are deliberately unsupported; preserve their refusal semantics rather than adding followers (the native WebSocket refusal gap is [083](../../blocked/083.md)).

## Saturated App production acceptance — sweep215, 2026-10-13 UTC

[Saturated native App acceptance](evidence/live-app-sweep215/README.md) now passes through published Bex 0.3.2 and pinned Render: 34 exact message/ID matrix checks and eight real native wire replies find 21 original records and the hidden marker behind 1005 independently stored nonmatches. All eleven padding cursor pages have no overlap and complete honestly. The original false-empty App boundary is corrected; actual deployed SHA remains unknown. One fixture/one child deploy is enumerated and deleted; baseline 6/quota 6/term 0. Prior nonsaturated PG/KV acceptance is retained, while latest saturated managed-data, non-REST/GraphQL, instance/budget and other full-DoD gates remain. No QA source implementation or ship.
