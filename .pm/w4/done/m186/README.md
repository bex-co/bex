# w4 · m186 — Keep project names current after a fast rename

**Worker:** worker4 **Goal:** a completed project rename publishes the saved name in the ready heading, breadcrumb/sidebar and document title even when a pre-save Projects read completes late. **Status:** done (2026-10-09, live closeout)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Order rename refresh after older project reads — **DONE** | 30m | — |
| t002 | Audit both rename entry points and shared read controls — **DONE** | 20m | w4/m186/t001 |
| t003 | Render parity — **DONE** | 10m | w4/m186/t002 |
| t004 | Simplify — **DONE** | 10m | w4/m186/t003 |
| t005 | Test coverage with a delayed read and real Router/Apollo — **DONE** | 30m | w4/m186/t003, w4/m186/t004 |
| t006 | Closeout — **DONE** | 20m | w4/m186/t005 |

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

## Implementation (2026-10-09, w4 /loopx)

t001–t005 are done; see the m186 commit.

- **t001.** `useRenameProject` sends one RenameProject. On success it drains every active Projects refetch (each result settled on its own), then re-reads `ProjectDocument` by ID `network-only` with a per-request `queryDeduplication:false`, and only then runs `router.invalidate()`. Busy holds until that finishes. A refused rename keeps its error. A refresh failure after an accepted save shows a warning whose "Try again" re-reads only, never re-writes. A newer server name is shown as-is. The Overview and Settings callers no longer invalidate early.
- **t002 audit.** Fixed callers: Overview and Settings (`useRenameProject`). Drained when mounted: eight `useProjects` watchers (home overview, selector, global search, sidebar, three breadcrumbs, plus the move hook as a reader). The 16 `titleLoaderFetchPolicy` calls in 15 files and the w4/m178 move refresh are unchanged. **Residual gaps (follow-up candidates, not this milestone's live race):**
  - a one-off Projects read (home loader, service-detail loader) or an `EnvGroupScopeIndex` read started before the save is not drained when no matching watcher is mounted;
  - a hover-preloaded pre-save by-ID read that lands after the final read could still write the old name;
  - `use-move-to-project.ts` runs its list refetch and by-ID re-read concurrently, the same ordering exposure, left out of scope (w4/m178).
- **t003 parity.** The dashboard publishes the saved API name, and both title formats are unchanged. The REST/GraphQL/MCP name contract is unchanged (all three go through `projects.Service.Rename`). ADR018's Projects & environments row records it.
- **t004 simplify.** A three-agent review was applied: a local `attempt()` retry, and test tidy-ups.
- **t005 tests.** `rename-publishes-saved-name.test.tsx` covers 9 cases with a real Apollo cache and a real TanStack router (real routes/loaders/heads), a real sidebar watcher, and held Projects responses: the Overview and Settings races, busy while draining, a settled control, no watcher, a rejected list result, a refused rename, refresh-failure retry with one write, and a concurrent newer name. **Mutation-checked:** the original code fails 6/9 (heading stays r3), concurrent final read fails 4/9, and the aggregate promise fails 1/9. `yarn test` (4462) and `yarn lint` pass.

Remaining: **t006**, the live production closeout after deploy.

## Live closeout (2026-10-09, w4 /loopx, pin `5cd4d4346`)

Fixtures: project `prj-db474k2oq5rs73822660` with environments `m186-prod`/`m186-stage` and a Free BusyBox web service `srv-db474kaoq5rs7382268g` (marker `qa-m186-marker`). A web service stood in for the DoD's static site; the membership and serving checks are the same.

- **Race (two runs).** Fresh Overview load, with Edit retried until the dialog opened, then an immediate rename to r2 and then r3. In both runs a pre-save `Projects` read **overlapped** the mutation (it started 404 ms and 326 ms before RenameProject and finished after it). Heading, title (`<name> ・ bex Dashboard`) and breadcrumb all showed the saved name without a reload. Exactly **one** RenameProject per save.
- **Settled control** (r4 after the page settled) passes, and a fresh reload agrees.
- **API.** The project, both environment IDs and membership (`m186-prod` → the service, `m186-stage` → none) are unchanged, and the service still serves `qa-m186-marker` with HTTP 200.
- **Cleanup.** The service, both environments and the project were deleted (REST by-ID 404), and the Kratos session was revoked.
