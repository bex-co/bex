# w8 · m57 — Preserve literal string lists in interactive CLI forms

**Worker:** worker8 **Goal:** applying unchanged log filters preserves their literal terms and selected result set. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Prepare an upstream lossless string-list form correction | 40m | — |
| t002 | Verify shared form callers and neighboring value types | 20m | t001 |
| t003 | Adopt the accepted upstream correction through the CLI pin | 30m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 25m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- On a fresh owned Free fixture emitting `LEFT,RIGHT`, `LEFT only`, and `RIGHT only`, native Bex and the adopted unmodified Render client use whole-second start/end bounds. `--text '"LEFT,RIGHT"'` initially returns only the combined row. Opening `/` and applying unchanged filters retains one literal `text` query term and that same row; editing an unrelated filter also preserves the term.
- Explicit `--text LEFT,RIGHT` continues to send two OR terms and return all three rows. Preserve ordinary multi-term values, embedded quotes, Unicode, empty/default distinctions where accepted, and CRLF bytes. A CSV reader that removes carriage returns is not a complete correction.
- On a fresh validly created service renamed to a comma-containing display name, native quoted `--resources` resolves its exact owned ID before and after unchanged Apply. Retain the complete name; do not look up its fragments or report the trailing fragment missing.
- Verify the counted shared callers and relevant resource selectors. Preserve enum multi-select and numeric-list conversion, and do not rewrite server log search, resource IDs, wire schemas or dashboard filters to compensate for a client defect.
- The correction is upstream, accepted and included in a reviewed CLI pin/release. A prepared patch alone is not Done. External submission and release need their own authorization; until those dependencies are satisfied, retain this milestone's evidence and park dependent work accurately.
- The failing journey, controls and shared-call tests pass; every fresh live fixture is deleted with detail/list/runtime absence and quota convergence. Complete standing tasks and move the milestone only when the observable behavior holds.

## Source + Goal linkage

- **Source:** user-requested continuous functional `$qa-find-bugs-cli`, w8, 2026-10-11, sweeps88–89. [Finding](finding.md) and [native wire evidence](evidence/native-ui.json) preserve both client reproductions and complete log responses.
- **Goal linkage:** ADR008 reliable hosting, ADR010 precise log inspection, ADR018 and the imported Render CLI contract. A filter must retain the literal values the CLI already accepted.
- **Expected outcome:** Apply preserves selected strings rather than silently widening one comma-containing term into two OR terms.
- **Why now:** both released Bex and the exact pinned upstream binary reproduce this on a healthy owned service; four normal noninteractive controls prove the server handles both query shapes correctly.
- **Render parity included:** the user-facing CLI changes; verify the actual upstream contract and unchanged REST/GraphQL/MCP/dashboard semantics. No Render production account was tested.
- **Scope:** researched filing only, seven tasks / 2h35m. No Bex fork/vendor patch, backend workaround, external message, product fix, commit or push is authorized by this QA filing.

## Unverified

Live text and a comma-containing owned service display-name selector were exercised in both native clients. Path filters and workflow environment forms share the conversion mechanism but were not live tested. The existing offline CRLF control passes before any correction. The external upstream issue index is not exhaustively searched; dedupe again before separately authorized submission.
