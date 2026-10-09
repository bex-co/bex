# w4 · m189 — Search the visible service log message

**Worker:** worker4 **Goal:** the service Logs search finds a literal visible message across ANSI styling while existing API/CLI raw searches and raw returned messages stay compatible. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit shared readers and specify the opt-in display matcher | 25m | — |
| t002 | Add a raw-preserving terminal-display search mode | 45m | t001 |
| t003 | Preserve scan coverage and paging for display search | 60m | t002 |
| t004 | Opt service history and live search into display matching | 45m | t003 |
| t005 | Verify Render/API compatibility and live controls | 25m | t004 |
| t006 | Simplify the changed code | 15m | t005 |
| t007 | Test display matching and incomplete scans | 35m | t005 |
| t008 | Verify the live reproduction and close out | 35m | t006, t007 |

## Definition of done

- On an owned Free BusyBox web service emitting the colored line in [the finding](finding.md), service Logs Last hour with text `QA_A31_ANSI_red_end` displays that record after settlement and after a fresh URL load, with Live off and on. Prefix `QA_A31_ANSI_` and word `red` still find it. Its returned raw message, timestamp, instance and styled red text stay intact.
- Fresh deploy detail search for the complete visible string continues showing the same record. Timestamp, wrapping and maximized controls remain usable.
- The finding's exact GraphQL probes without the new mode keep their existing raw responses: complete visible string matches nothing; prefix/word return the original ANSI bytes. Default raw compatibility is preserved.
- Delete the owned fixture, verify its API/public absence and the baseline resource ID sets, and revoke only the QA session. Do not close on unit tests alone.

## Source + Goal linkage

- **Source:** live qa-find-bugs cycle31, 2026-10-09 UTC, muse credentials; user requested w4 filing. [Researched finding and complete probes](finding.md). Local ignored evidence: `.playwright-mcp/qa-logs-a31-ledger.json` and the two qa-log-search-a31 screenshots. Severity major; filing only.
- **Goal linkage:** ADR008 usable Render-alternative hosting and ADR010 reliable observability; the current deploy viewer already demonstrates displayed-text search.
- **Expected outcome:** copying the visible colored runtime message into Search logs returns the real line, with raw bytes retained for CLI and API consumers.
- **Why now:** the live empty result can mislead diagnosis; a backend/display-reader mismatch cannot be repaired by filtering one already-capped page. Preserve w2/m167 raw terms, w4/m140 bounded scans, and w9/m63/m83 rendering/performance.
- **Scope:** one observed service-view defect, 8 tasks, about 4h 45m. Optional display-search semantics across exposed adapters are an explicitly labeled Bex extension. Dashboard opt-in is scoped to LogViewer; datastore consumers are controls to audit, not separately asserted live failures. Render parity is included because the query model and dashboard change. New transports, raw-byte rewriting, regex features and unrelated filter discovery are excluded.
- **Unverified:** colored build/datastore/sibling service flows, live-only future arrival, >100-record and timeout scans, dark/mobile themes, foreign-resource refusals and authenticated Render ANSI behavior. These are implementation verification obligations, not observations made by this sweep.
