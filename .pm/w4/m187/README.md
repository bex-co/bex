# w4 · m187 — Restore environment lifecycle on project moves and deletion

**Worker:** worker4 **Goal:** a project move clears incompatible environment placement, and authorized deletion detaches surviving resources even when an environment group is linked. **Status:** todo

**Size:** 270m (4h 30m), eight tasks. Two independently reproduced major bugs, with separate [move](finding-move.md) and [deletion](finding-delete.md) records. This ships the filing only.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Clear incompatible environment placement on incoming project moves | 45m | — |
| t002 | Detach linked group scope during authorized grouping deletion | 55m | — |
| t003 | Audit shared callers, admission and passing lifecycle controls | 30m | w4/m187/t001, w4/m187/t002 |
| t004 | Repair provably incompatible existing App placement safely | 40m | w4/m187/t001, w4/m187/t003 |
| t005 | Render parity | 25m | w4/m187/t002, w4/m187/t003, w4/m187/t004 |
| t006 | Simplify | 15m | w4/m187/t005 |
| t007 | Test coverage | 45m | w4/m187/t005, w4/m187/t006 |
| t008 | Closeout | 15m | w4/m187/t007 |

## Definition of done

Recreate the owned Free BusyBox web, two projects, their environments and the synthetic group from the findings. Keep the workspace's preexisting resources untouched.

- From a fresh source environment page, Actions → Move to project → the other owned project performs one membership write. Source project/environment omit the service; target project contains it; service GraphQL Get/List show target project and null environment. REST/MCP omit environmentId, and the captured REST filter for the former environment returns []. Repeat with the group linked and then unlinked. Fresh service Environment does not offer the former environment's group.
- Explicit environment assignment still joins its parent project and reports that environment everywhere. The captured Stage→Prod move, environment-delete-retains-project, Unassigned A→B/B→A and Remove from project controls still give their stated memberships and public HTTP200 marker; fresh source rows/counts match the settled reads.
- With service and linked group correctly in the target environment, fresh Project Settings → Delete Project succeeds without a preliminary unlink. Project and child environment GET return404; service and group survive with null environment, service project is null, group links and synthetic value/revision survive, and the service remains reachable. REST delete_project equivalent and MCP delete_project have successful protocol outcomes on fresh equivalent fixtures. The separately captured direct REST environment DELETE also succeeds with a linked group while retaining the service's parent project.
- Ordinary Move group with incompatible linked services still refuses with ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES and does not change group scope/content/links. Moving an unlinked group still preserves its value and content revision. These are deliberate guards, distinct from deletion's planned detach.
- Delete the owned service, group, projects and environments. By-id API reads and public URL return404; the six owner-filtered ID sets match the starting inventory; revoke only the run's session and remove its cookie/state files. Record the replay before closeout.

Unobserved cases, native database/CR invariants, recovery and failure injection are explicit verification work in t003/t004/t007, not claims that this production pass exercised them.

## Source + Goal linkage

- **Source:** continuous qa-find-bugs, user-selected w4, 2026-10-09 UTC cycle24; muse.env authentication via qa-login.sh. Research main b272b9c577a7762c8ab5d982b490982a2cc48ced. Complete probes are embedded in the two findings; local ledger .playwright-mcp/qa-project-environment-placement-a24-ledger.json (46 captures plus rejected harness attempts).
- **Goal linkage:** ADR008 reliable, machine-readable hosting; ADR003 committed control-plane state; ADR006 consistent APIs; ADR032 project/environment lifecycle and environment-scoped configuration; ADR018 grouping parity.
- **Expected outcome:** project membership cannot retain another project's environment, and grouping deletion preserves/detaches resources as Bex documents instead of misapplying the public group-move guard.
- **Why now:** fresh repeats through the live dashboard and all three API read surfaces expose persistent incompatible placement. Correctly placed linked resources then block project/environment deletion through the shared cleanup path. Fixing display caching or asking users to unlink would leave the authoritative lifecycle defects.
- **Render parity included:** REST/GraphQL/MCP/UI behavior changes. [Render projects documentation](https://render.com/docs/projects) describes environments inside projects and environment-scoped groups. Bex's preserve-and-detach deletion deliberately differs from Render's cascading dashboard deletion and empty-only REST rule; preserve the recorded ADR032 divergence.
- **Sizing:** two separate write/cleanup defects, shared caller and admission coverage, already-persisted drift recovery, native and composed regressions, and four standing closing tasks. No product changes made by this hunt.

## Dedupe and prior guarantees

Scanned all 74 non-done README files (including workstream/dev files, not74 milestones), distinctive terms across open and done notes/tasks, DO_NOT_DO.md, last40 dashboard/lego commits and targeted symbol histories. No open item owns these producer/teardown defects. w9/m89 and m92 are filing precedents, not duplicates.

- **w4/m32:** first DoD clause (departure clears apps.environment_id and stamped layer) fails for incoming cross-project assignment at the membership-read level; live CR-layer enforcement was not inspected. Its project-delete clause cannot be fully revalidated here because the linked-group teardown refuses before deleting the row; CR fields and both datastore kinds remain unverified. Its pre-m28 enforcement/backfill and sweep-idempotency clauses were not exercised. ADR032 and w1/m56 retired those migration-only sweeps at c5d31d1c6; t004 must repair current provable drift without reinstating obsolete migration behavior. t003/t007 explicitly cover all current successors of these guarantees.
- **w4/m131:** all five DoD clauses were reviewed. Committed Get/List/REST filtering/MCP share apps.readViews and expose the same bad source pair, so this is not evidence of a CR-read-lag regression. Clear/deletion reads and optional omission worked in the controls. Missing-source, ownership/failure handling, bounded batched reads and unmanaged/storeless fallback were inspected in source or left to regression coverage; no live foreign/error/fallback probe is claimed. Keep those guarantees.
- **w6/036:** d247d210e correctly replaced remove-then-add with one target write. A second UI mutation would reintroduce its orphaning window. The store must handle the old-environment departure itself.
- **w4/m178:** live Unassigned moves in both directions and removal refreshed correctly with one write. Its cache publication fix does not correct incompatible API placement. Do not close its cluster-cleanup DoD from this API-only cleanup.
- **w7/m151/m152:** separate Postgres/KeyValue placement and authorization-preflight corrections; keep resource-owned rules and conditional cleanup. No live datastore mutation or role-denial probe this pass.
- **w4/m97 and w2/m73:** ordinary group moves must validate current links under CAS and enforce exact scope. Keep that public guard; deletion needs a narrow authorized detach protocol. No concurrency drill was performed here.
- **w4/217 and220:** deployed UI group filtering now follows the service's environmentId. Its incorrect available group in this pass is a traced consequence of the bad backend placement, not the previously fixed cache-map defect.

## Cleanup and limits

All seven created logical IDs returned404 at10:04:28Z; public URL returned404 at10:04:30Z. Final six-family inventory ID sets equal the valid initial baseline (5services,4databases,2key-values,3projects,1group,1blueprint). The run's session was revoked; its browser cookies then got whoami401, browser was cleared to0cookies/1login page, and jar/state files were removed at10:05:15Z.

No Kubernetes workload/PVC/child-object absence claim: production kube credentials were not used. The public body was a constant fixture marker, not evidence that a group variable reached its process. Protected/deny-all/isolation-on rules, sibling App kinds, concurrency, recovery, mobile/zh and authenticated Render mutations were not live exercised.
