# w2 · m167 — Preserve every requested log text filter

**Worker:** worker2 **Goal:** the pinned CLI and MCP search all supplied text values with OR semantics, consistently across stored and live log sources. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve and evaluate the complete text-filter set | 45m | — |
| t002 | Audit every adapter, log source, and paging boundary | 40m | t001 |
| t003 | Render parity and compatible scalar consumers | 25m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Behavioral regressions and live CLI acceptance | 45m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

- An owned service emits separate `qa_text_A` and `qa_text_B` lines plus an unrelated control. Both `bex logs -r <id> --text qa_text_A,qa_text_B -o json` and the reversed order return both matching lines exactly once and exclude the control. The unchanged same-pin Render binary gives the same result against Bex.
- REST repeated `text` parameters and MCP `list_logs(text:[...])` honor every value. Values OR within `text`, then AND with other filters. Single-value case-insensitive substring behavior, literal escaping, and scalar GraphQL/dashboard inputs remain compatible; no client fork or public GraphQL array migration is needed.
- The same normalized term set reaches Loki queries, pod-log fallback and supported live app/build tails, and the generic managed Postgres/Key Value query paths. Empty, duplicate, no-match and reversed-order cases are defined and tested without duplicate rows or accidental query widening.
- Existing authorization/resource scoping, time bounds, page limits, cursor continuation and the bounded broad-search budget remain correct. Label-value discovery keeps its documented stream-index limitation; unsupported tail filters still fail explicitly.
- Appropriate backend checks and live CLI/MCP acceptance pass. Record source coverage and gaps, and remove owned fixtures before closeout.

## Source + Goal linkage

- **Source:** User-requested infinite `$qa-find-bugs-cli` loop in w2, 2026-10-02, sweep 12. [Researched finding and reproduction](finding.md), [complete sanitized captures](evidence.json). Severity **major**; filing only, no implementation.
- **Goal linkage:** Render-compatible hosting and trustworthy incident diagnosis (ADR008, ADR006, ADR018 and ADR010).
- **Expected outcome:** Multi-term searches return the requested union instead of a plausible, incomplete success determined by argument order.
- **Why now:** Installed Bex, unmodified same-pin Render, raw REST and MCP all reproduce silent omission in production; the first-only reductions remain on current main. Existing multi-resource, log-level and timeout fixes cover different boundaries.
- **Scope:** One shared text-filter defect. About three hours including a dedicated blast-radius audit and standing closing tasks. Render parity is required because REST/MCP arrays and scalar GraphQL/dashboard consumers share the same query model. Full regex support, new logging transports and expanding tail capabilities are outside this repair.
