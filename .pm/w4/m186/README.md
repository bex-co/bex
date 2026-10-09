# w4 · m186 — Keep project names current after a fast rename

**Worker:** worker4 **Goal:** a completed project rename publishes the saved name in the ready heading, breadcrumb/sidebar and document title even when a pre-save Projects read completes late. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Order rename refresh after older project reads | 30m | — |
| t002 | Audit both rename entry points and shared read controls | 20m | w4/m186/t001 |
| t003 | Render parity | 10m | w4/m186/t002 |
| t004 | Simplify | 10m | w4/m186/t003 |
| t005 | Test coverage with a delayed read and real Router/Apollo | 30m | w4/m186/t003, w4/m186/t004 |
| t006 | Closeout | 20m | w4/m186/t005 |

## Definition of done

Repeat [finding.md](finding.md) with fresh owned qa-prefixed Free fixtures:

- From a fresh project Overview, rename as soon as Edit is available, capturing an overlapping pre-save Projects read. Once the save and resource table settle, the heading, breadcrumb/sidebar and title show the name returned by the authoritative post-save project read without reload. Repeat with another distinct name. A capture without overlap does not prove the race repaired.
- The already-settled rename remains a passing control, and fresh reload agrees with the ready display.
- Project/environment IDs and resource membership remain unchanged by renaming; the owned static site continues to serve its marker/expected fixture body. Exactly one rename mutation is issued per save.
- Delete the owned site, both environments and project; verify logical REST404, baseline lists, and public404. Revoke only this run's Kratos session.

Settings, refresh failures, mobile/zh, other title families and authenticated Render timing were not reproduced live here. t002/t005 must verify the bounded implementation on those relevant callers and failure classes without presenting them as already observed bugs. This milestone restores the specific m54 rename guarantee; it does not claim a complete new head-policy audit.

## Source + Goal linkage

- **Source:** continuous qa-find-bugs, 2026-10-08 LA / 2026-10-09 UTC, cycle 6, muse.env; [researched finding and complete durable probes](finding.md). Research main 6c50f19c3.
- **Goal linkage:** ADR008 dependable hosting and ADR032 truthful project/environment navigation; w2/done/m54/t003's saved-name title guarantee.
- **Expected outcome:** after a successful rename, late pre-save list data cannot remain published as the completed project name.
- **Why now:** two natural fresh-load reproductions isolate a successful mutation followed by an older list result. Polling later repairs watcher labels while the loader/head remains stale; reload is currently needed.
- **Sizing:** about 2h across the bounded ordering fix, dedicated caller/writer audit, real Router/Apollo race and failure tests, parity, simplification and live replay. Existing m178 test setup can be reused, but its synchronous membership server does not test this race.
- **Render parity:** included (t003); UI labels change, while the backend REST/GraphQL/MCP name contract remains authoritative. Render's documented rename/navigation surface is linked in finding.md; authenticated race comparison is unverified.
- **Scope:** two useRenameProject callers. Preserve the sixteen titleLoaderFetchPolicy calls in fifteen files and the eight useProjects callers' passing behaviours. No global cache/dedup policy change.
- **Dedupe:** related w4/blocked/m178 repairs useMoveToProject, not useRenameProject. This is a separate residual writer; no mutation of that milestone's status is required.
- **Cleanup:** all four owned resources deleted and the session revoked before filing. No product fix or production leftover is included.
