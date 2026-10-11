# w8 · m58 — Preserve Unicode matches in log history

**Worker:** worker8 **Goal:** normalized Unicode log matches survive raw and display history through the unchanged CLI and shared API **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Make the Loki text prefilter preserve normalized Unicode matches | 50m | — |
| t002 | Verify shared history callers and resource families | 30m | t001 |
| t003 | Render parity and live native CLI acceptance | 20m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Meaningful Unicode and pagination regression coverage | 35m | t003 |
| t006 | Closeout | 10m | t005 |

## Definition of done

For an owned fixture that prints `CAFÉ Ελληνικά` and `café ελληνικά`, released Bex and the unchanged pinned Render client against Bex return both original messages for `logs --resources OWN_ID --type app --text '"café ελληνικά"' --limit 100 --confirm -o json` and for `--text ι`. Capture the actual requests and complete responses. Raw GraphQL agrees; the passing `textMode=display` control remains correct. Preserve literal ASCII/metacharacter/quote/space matching, multi-term OR, original bytes, time bounds, direction, instance selectors, and limits. A match behind a full raw scan page remains reachable; budget exhaustion reports incomplete coverage rather than a false empty completed window. Verify affected datastore, pod fallback and live-tail behavior without introducing unsupported filters. Record live deployment evidence, cleanup and all material gaps before moving this milestone to `done/`.

Also reproduce the sweep-96 table: Russian and Greek-rho words return two capital/lowercase lines, ASCII Kelvin and Istanbul terms return three, and iota returns two through both clients. Prefixing the term must never increase its match set. Plain-message raw and display APIs agree, including dotted capital İ. A correction gated solely on non-ASCII query terms or using only full regexp matching does not meet this definition.

Managed Postgres acceptance: emit the three sweep-100 markers through native SQL and verify both clients return three for the marker prefix, two for iota and one for lambda using supported datastore filters. Do not introduce app-only type filters.

Managed Key Value acceptance: use the native EVAL marker commands in sweep 101 and require both clients to return three prefix records, two iota records and one lambda record; preserve native terminal completion and original bytes.

## Source + Goal linkage

- **Source:** [CLI sweep 94 finding](finding.md) and [durable evidence](evidence/native-wire.json), observed 2026-10-11 with human QA credentials in bex-canary; no product code or deployment changed by this hunt.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [ADR010](../../../docs/ADR010-observability.md) and [ADR018](../../../docs/ADR018-render-parity.md): customers must be able to find real application logs through the imported terminal client and shared API.
- **Expected outcome:** localized and ASCII searches no longer omit stored case-normalized matches or contradict stricter-prefix queries.
- **Why now:** both production native clients reproduce it; local Loki 3.6.7 and its exact regexp dependency isolate canonical-literal, first-byte and full-fold mismatches. w2/m167 explicitly left broad Unicode parity unverified; this is now a concrete ordinary-language failure.
- **Parity:** included because the fix changes tenant-facing history search. REST, GraphQL, MCP and dashboard consumers need scope checks; Greek display-mode REST passes, but plain dotted-İ display REST fails; the dashboard UI remains unverified.

## Scope

File and implement the backend history fix with bounded work. An upstream Loki contribution or dependency upgrade may be evaluated separately; neither is required to start the Bex fix. Do not alter the pinned Render CLI or add a launcher request rewrite. Raw CLI, raw GraphQL and the specified display-mode REST failure are live-confirmed; the subsequent native App live-tail Unicode control passes. Managed Postgres markers now reproduce the raw-history defect through both native clients (sweep 100). Managed Key Value markers reproduce the same raw-history defect through both native clients (sweep 101). Datastore display/convenience routes, MCP and dashboard UI remain unverified. Managed Postgres/Key Value live subscriptions are deliberately unsupported; preserve their refusal semantics rather than adding followers (the native WebSocket refusal gap is [083](../083.md)).
