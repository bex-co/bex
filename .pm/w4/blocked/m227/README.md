# w4 · m227 — Explain unavailable project resource lists

**Worker:** worker4 **Goal:** a failed resource-list read cannot tell a customer that a populated environment is empty. **Status:** blocked — t001–t005 done 2026-10-17; t006 (owned replay on a deployed build at desktop and 390, then cleanup) open

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Carry resource read availability into Project Overview — **DONE** | 35m | — |
| t002 | Audit selected, Unassigned and membership-editor consumers — **DONE** | 25m | w4/m227/t001 |
| t003 | Render parity — **DONE** | 15m | w4/m227/t002 |
| t004 | Simplify — **DONE** | 10m | w4/m227/t003 |
| t005 | Test coverage — **DONE** | 35m | w4/m227/t003 |
| t006 | Closeout | 20m | w4/m227/t004, w4/m227/t005 |

Six tasks, 140m total (2h20m). **Major:** a native read failure hides an existing service and Key Value behind `0 resources` and the normal empty-state invitation. Implemented 2026-10-17; awaiting the deployed-build replay (t006).

## Definition of done

Repeat the [recorded journey](finding.md) with a fresh owned Project, Blue/Green environments, Free image web service and Free/private Key Value:

- On a fresh desktop Project load, a successful membership read containing the service and KV combined with the recorded failed Services/Databases/KeyValues outcomes displays resource availability failure and a localized read-only Retry. It does not claim `0 resources`, successful zero type counts or “No resources in this environment yet.” Preserve the selected environment and filter intent. Pending reads use the same card geometry as the ready view, without asserting an empty result before they settle.
- When the reads recover, the same selected Blue view shows two actual rows, All(2), Services(1) and Key Values(1), with their original links. Retry reads metadata only; it creates, deletes and moves nothing. Record a failed retry separately from successful recovery. A successful empty Green read still displays the ordinary zero counts and empty message.
- Preserve the recorded healthy desktop and fresh390×844 controls, filtered select-visible-all selecting only the service, native two-kind Move to Green, mobile service-only Move back, and resulting fresh membership. Source and destination counts settle to their actual memberships. Deleting the two nonempty environments retains the resources in Project/Unassigned; deleting the Project retains the resources. The public marker and surviving KV data remain correct; an already expired TTL key must not be diagnosed as loss.
- Complete t002/t005's partial-family, Unassigned, EnvGroup, mounted-draft and error-class work without presenting those unrun cases as prior live failures. Preserve existing parent-route auth/not-found handling and background-refresh mounting. Relevant dashboard checks pass. Delete all owned fixtures and exact cluster children, restore seven baseline ID sets and revoke only that replay's session before closeout.

## Source + Goal linkage

- **Source:** functional qa-find-bugs w4, a883, 2026-10-17 UTC; [finding and source references](finding.md), [durable observations, exact safe request/response receipts, capture sources and cleanup](evidence.json). Research main: 01798d4959c933508d4b6eee061295adaa9ebf6d; serving Git SHA unestablished.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) dependable hosting and [ADR032](../../../../docs/ADR032-environments.md) truthful resource grouping. [ADR006](../../../../docs/ADR006-bex-api.md) keeps API outcomes authoritative.
- **Expected outcome:** distinguish pending, unavailable, partial, stale-after-refresh and successfully empty inventories while preserving user intent and offering bounded read recovery.
- **Why now:** two ordinary observations, including a fresh desktop reload, show false zero counts. The native batch returned QUERY_TIMEOUT for EnvGroups and OPERATION_NOT_STARTED for the three resource lists while successful Environments data still listed both owned members. Later healthy reads and real moves prove those members existed.
- **Scope:** allowlist availability rendering to Project Overview and its selected/Unassigned/membership-editor consumers. Retain the four hooks' existing error/refetch contracts and current GraphQL batch deadline. Dedicated t002 covers the shared-callers census and compatibility controls.
- **Relationship:** a new consumer gap beyond w4/blocked/m211's structured batch outcomes, w4/m221's creation selector and w4/blocked/m218's plan catalogs. It is not their closeout or a claimed regression of their repairs. Earlier grouping/polling guarantees are dispositioned in the finding.
- **Render parity included:** the tenant-facing inventory changes. [Render documents project resource visibility](https://render.com/docs/projects); authenticated Render timeout/retry behavior was not tested. Keep Bex's resource-preserving grouping-deletion semantics, which differ from Render's current documented cascading deletion.

## Evidence limits

The original native failed-list recorder retained exact operation documents and error arrays but coalesced absent/null data. Those entries are explicitly projections, not complete original response bodies. Complete independent membership and native mutation entries are retained separately; a later complete direct read batch passed. Mobile and second-desktop reloads were healthy controls. No production fault was injected and no runtime or membership loss was caused by the read failure.

## Additional live evidence — a889, 2026-10-17

[Shared durable receipt](../../evidence/333-m227-a889.json) adds the Unassigned consumer's native failure on a fresh390×844 view of the existing QA workspace project `prj-d9dgeo0bd9nc73a0vh1g`. This pass is read-only; it creates no fixture and changes no resource.

The eight-operation native request2075 completes in25.282790s. Projects in that same batch successfully retains `srv-d9bkcspg9s7c73d0n8ug` and `srv-d9e40ei9086p3l1jri30`, and Environments successfully returns an empty list. EnvGroups returns QUERY_TIMEOUT; Services, Databases and KeyValues each return OPERATION_NOT_STARTED. At14:27:20.014Z, after the request has completed, the Unassigned card nevertheless exposes All(0), Services(0) and “No resources in this environment yet.”

These are **complete decoded relevant operation entries with explicit `data:null`**, improving on a883's coalesced failure recorder. Original request/response positions and operation order are retained; unrelated BillingReadiness and Workspaces response entries are explicitly omitted. This is not a claim to retain an unmodified whole eight-entry response body. Internal stage cost is unmeasured, and the existing serial-batch timeout classifications remain correct.

A later healthy native batch2085 resolves all resource families. The14:27:52.592Z snapshot has both real rows and All(2) without an intervening filter action; an independent canonical read corroborates the project/Unassigned membership. The screenshot attempted after the false-empty capture caught this recovered view and is not offered as failure proof. Subsequent native type/search/URL controls pass and separately close w4/333; they do not close m227.

This extends the already scheduled Unassigned work in t002, without adding a duplicate task or changing the six-task scope/status. The exact66-request native audit contains112 query operations and zero mutations; seven baseline sets are unchanged, and the sole QA session is revoked with whoami401/zero cookies/one blank page at14:33:42.601Z. No runtime, membership, move, explicit Retry, partial-family failure or named-environment closeout is claimed by this read-only case. Original a883 evidence is byte-unchanged.

## Implementation record (2026-10-17)

- **t001:** a new `useProjectResourceCatalog` hook (used by the Project route) wraps the Services, Databases and Key Values list hooks and reports which have not answered (`pending`) or failed (`failed`), with a `retry` that re-reads only the failed lists. It keeps each list's last good answer for the workspace through a failed refresh. EnvironmentsPanel adds Env Groups (whose error it used to drop) and owns a busy-aware read-only Try again (`useCatalogRetry`). EnvironmentCard and Unassigned compute the unread families they actually depend on: the shared `ReadUnavailable` alert explains a failure ("may be incomplete or out of date"), a pending read shows a neutral loading line, and the empty invitation is suppressed. That family's filter count, the All count and the header total read as unknown ("—", "Count unavailable"/"Counting…") instead of zero. Healthy families keep their rows and counts. URL scope canonicalization now also waits for failed lists, Unassigned stays offered while a list is failing, and a Project with no Environments shows the notice. A successful empty read keeps the ordinary zero view. The list hooks, cache policy and mappers are unchanged.
- **t002 census/disposition:** one EnvironmentsPanel, EnvironmentCard, UnassignedResources and ManageResourcesDialog mount, all on `/project/$projectId/`. ManageResourcesDialog seeds its full-replace drafts from membership ids, not the lists, so saving never dropped unread members; a tab whose list is unread now shows the notice instead of an empty checklist and Save. Mounted card selection, the move target and dialog drafts are component state and survive failures and retries. Home already excludes list errors from its own empty predicate and keeps its wiring (it needs polling controls the new hook does not expose). Error classes (QUERY_TIMEOUT, OPERATION_NOT_STARTED, transport) all surface as a failed list; auth/not-found stay with the parent route.
- **t003:** ADR018's Projects row records the behavior; no API change.
- **t004:** three /simplify passes (shared family type in the dialog, `environmentMemberIds`/`unreadFamilies`/`NOTHING_UNREAD` helpers in `environments/lib/unread-resources.ts`, derived loading prop, the scope/Unassigned/zero-environment fixes, last-good lists, header copy, a shared `blankSelection` test helper).
- **t005:** `features/projects/__tests__/project-catalog-unavailable.test.tsx` runs the real list hooks, catalog, panel and card on a real Apollo client with the app's defaults (only the network is faked). It covers failed Services+Key Values → alert, unknown counts, no "0 resources"/empty invitation, then Try again recovering to two rows with All(2)/Services(1)/Key Values(1), one re-read per failed list and no other operation; busy Try again and a failed retry; a failed refresh keeping the last rows; mixed healthy/failed families; Databases/Env Groups failures irrelevant to the environment; a successful empty read; no scope rewrite from failed lists; Unassigned; and a Project with no Environments. The two core cases fail against the old card. The dialog test covers a blocked tab with Try again and healthy tabs; the route test checks the catalog reaches the panel. Dashboard suite (5179 tests), lint and typecheck pass. Unverified live: PG/Env Group members, other roles, locales.
