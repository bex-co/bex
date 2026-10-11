# w4 · m178 — Refresh source project and environment membership after moves

**Worker:** worker4 **Goal:** a completed project move removes the resource from the source project's and environment's row and count without a reload, while preserving authoritative target placement and existing navigation caching. **Status:** todo (t007 ready; t001–t002 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refresh the source Project before publishing the loader snapshot — **DONE** | 30m | — |
| t002 | Audit the shared move family and preserve passing refresh controls — **DONE** | 20m | w4/m178/t001 |
| t007 | Refresh the source Environment after project move and removal | 45m | w4/m178/t001, w4/m178/t002 |
| t008 | Audit the three-kind Environment refresh and preserve passing consumers | 25m | w4/m178/t007 |
| t003 | Render parity (rerun for Environment refresh) | 15m | w4/m178/t002, w4/m178/t008 |
| t004 | Simplify (rerun after the added fix) | 10m | w4/m178/t003 |
| t005 | Test coverage with real Router, Apollo and Environment consumer | 40m | w4/m178/t003, w4/m178/t004, w4/m178/t008 |
| t006 | Closeout | 10m | w4/m178/t005 |

## Definition of done

Repeat the owned Free image-web journey in [finding.md](finding.md), creating fresh qa-prefixed fixtures and cleaning them afterwards:

- From a fresh source project's Unassigned page, Actions → Move to project → the other owned project succeeds. Once its refresh settles, the source row and Services count are zero without reload, and GraphQL shows source empty / target assigned. Repeat in the opposite direction.
- Reload of the source agrees with its already refreshed display; it is no longer needed to remove the row.
- Remove from project still updates the source table without reload, leaves the service ungrouped, and does not perform an extra membership write.
- The fixture keeps Running revision 1 and its public marker returns HTTP200 through the placement-only operations. No unrelated resource is moved.
- Delete the service/projects/environments, verify logical REST404 and no owned cluster objects, and revoke this QA session. Only mark done after the live replay passes.

Added pass-155 acceptance from [finding-environment.md](finding-environment.md):

- Fresh A/Stage → row Actions → Move to project → B: after a truly post-write source Environments read settles, Stage has zero row/count without reload; GraphQL says B/null environment and REST Stage-filtered list is []. Repeat twice with exactly one membership write.
- Fresh A/Stage → Remove from project: source Environment row/count become zero after refresh, service is ungrouped, no second membership write or redundant singular Project read.
- Preserve the captured A/Unassigned→B control (zero row/count after the Project refresh) and source-only scope. Old matching list responses must settle before final post-write data wins; test refresh rejection separately from write refusal.

Sibling kinds, Overview, refresh-rejection and prefetch controls were not exercised by this live finding; t002/t005 verify them without representing them as already observed failures.

## Source + Goal linkage

- **Source:** continuous qa-find-bugs, 2026-10-07 UTC sweep 8, muse.env, workspace bex; [researched finding and complete probe](finding.md). Research main 2892c2091.
- **Goal linkage:** ADR008 dependable hosting; ADR032 truthful Project/Environment placement.
- **Expected outcome:** successful moves change the displayed membership automatically, with unchanged backend placement and runtime.
- **Why now:** the old source-refresh guarantee from bb2ce9d74 regressed when retained loaders became cache-first. The UI toast reports the move while the old project still claims the resource; repeated live captures isolate that disagreement.
- **Sizing:** ~3h 15m including the original scope plus 45m Environment refresh and 25m family audit across a bounded fix, three-kind family audit, real Router/Apollo integration regression, parity, simplification and live closeout. The integration must reproduce cache publication ordering; the existing mocked invalidate-call tests do not cover it.
- **Render parity:** included (t003), because displayed project membership changes. REST/GraphQL/MCP remain canonical; compare the UI to those reads and the documented Render move surface without inventing a different grouping model.
- **Scope:** source Project and source Environment freshness after project Move/Remove. Keep the shared title policy's 16 call sites in 15 files and the single target membership write.
- **Unverified:** sibling resource-kind live moves, Overview rendering, refresh rejection, other title-loader consumers, MCP and authenticated Render timing; see finding.md.

## Progress (2026-10-08)

t001–t005 done. After a cross-project move, `useMoveToProject` re-reads the source `Project` network-only (`client.query`, the existing typed `ProjectDocument`), alongside the existing `Projects` refetch. `router.invalidate()` runs only after both settle, so the retained project page's cache-first loader republishes the source's fresh membership. Still exactly one membership write (w6/036). A rejected re-read still invalidates and never turns the successful move into a failure toast. Remove-from-project needs no extra read: its mutation returns the source itself. The shared title-loader policy (cache-first on `stay`) is untouched.

Family: the hook is shared by the three row-action mounts (service, database, key-value), and `ProjectDocument` carries all three ID lists, so every kind gets the same refresh. Only the service kind is exercised by the integration test. The Overview reads the live `Projects` watcher and is unchanged.

Tests: the new `move-refreshes-source-project.test.tsx` uses a real `ApolloClient`/`InMemoryCache`, a real TanStack router with a cache-first-on-stay loader, and a scripted single-valued membership server. It **fails on the old hook** (the moved row stays) and passes on the fix. The unit tests add the network-only source read ordered before invalidate, re-read-rejection, and no-read-on-remove cases. Dashboard `yarn lint` (typecheck, eslint, knip) and the full `yarn test` (487 files, 4395 tests) pass. ADR018 Projects row updated.

## Added live finding (2026-10-11, pass 155)

The shipped singular Project refresh is retained: the fresh A/Unassigned→B control passed in 0.791 seconds. Selected A/Stage still showed the moved row/count 20 seconds after the source Project refresh, and selected-environment Remove reproduced the same list gap. Extend this existing owner instead of filing a duplicate: [complete finding and probes](finding-environment.md), research main 1cc56f66a. t007/t008 are added before the standing closing tasks; t003/t004/t005 are reopened for the added implementation. Their original 2026-10-08 completion remains recorded in the historical Progress section above. t001/t002 remain done; Closeout stays last. The milestone is reactivated from blocked because implementation work is now ready.

Why now: the Project fix leaves the selected Environment's separate Apollo membership list to its next poll. Goal and expected outcome remain truthful placement and automatic source row/count publication, now covering both consumers. Render parity is rerun for the UI change; REST/GraphQL/MCP placement and documented deletion semantics remain canonical.

Pass 155 also verified project/environment creation placement, authoritative environment clearing, and linked-group project deletion retaining service/link/value. These are partial m187 controls, not its closeout. All six owned fixture APIs and the public URL returned404; six baseline ID sets exact; own session revoked (whoami401, cookies0). Cluster remnants and unrun sibling/inverse/error cases remain outstanding as the finding states. This ships the filing only.
