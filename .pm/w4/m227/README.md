# w4 · m227 — Explain unavailable project resource lists

**Worker:** worker4 **Goal:** a failed resource-list read cannot tell a customer that a populated environment is empty. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Carry resource read availability into Project Overview | 35m | — |
| t002 | Audit selected, Unassigned and membership-editor consumers | 25m | w4/m227/t001 |
| t003 | Render parity | 15m | w4/m227/t002 |
| t004 | Simplify | 10m | w4/m227/t003 |
| t005 | Test coverage | 35m | w4/m227/t003 |
| t006 | Closeout | 20m | w4/m227/t004, w4/m227/t005 |

Six tasks, 140m total (2h20m). **Major:** a native read failure hides an existing service and Key Value behind `0 resources` and the normal empty-state invitation. This schedules a UI fix; no product code is included.

## Definition of done

Repeat the [recorded journey](finding.md) with a fresh owned Project, Blue/Green environments, Free image web service and Free/private Key Value:

- On a fresh desktop Project load, a successful membership read containing the service and KV combined with the recorded failed Services/Databases/KeyValues outcomes displays resource availability failure and a localized read-only Retry. It does not claim `0 resources`, successful zero type counts or “No resources in this environment yet.” Preserve the selected environment and filter intent. Pending reads use the same card geometry as the ready view, without asserting an empty result before they settle.
- When the reads recover, the same selected Blue view shows two actual rows, All(2), Services(1) and Key Values(1), with their original links. Retry reads metadata only; it creates, deletes and moves nothing. Record a failed retry separately from successful recovery. A successful empty Green read still displays the ordinary zero counts and empty message.
- Preserve the recorded healthy desktop and fresh390×844 controls, filtered select-visible-all selecting only the service, native two-kind Move to Green, mobile service-only Move back, and resulting fresh membership. Source and destination counts settle to their actual memberships. Deleting the two nonempty environments retains the resources in Project/Unassigned; deleting the Project retains the resources. The public marker and surviving KV data remain correct; an already expired TTL key must not be diagnosed as loss.
- Complete t002/t005's partial-family, Unassigned, EnvGroup, mounted-draft and error-class work without presenting those unrun cases as prior live failures. Preserve existing parent-route auth/not-found handling and background-refresh mounting. Relevant dashboard checks pass. Delete all owned fixtures and exact cluster children, restore seven baseline ID sets and revoke only that replay's session before closeout.

## Source + Goal linkage

- **Source:** functional qa-find-bugs w4, a883, 2026-10-17 UTC; [finding and source references](finding.md), [durable observations, exact safe request/response receipts, capture sources and cleanup](evidence.json). Research main: 01798d4959c933508d4b6eee061295adaa9ebf6d; serving Git SHA unestablished.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) dependable hosting and [ADR032](../../../docs/ADR032-environments.md) truthful resource grouping. [ADR006](../../../docs/ADR006-bex-api.md) keeps API outcomes authoritative.
- **Expected outcome:** distinguish pending, unavailable, partial, stale-after-refresh and successfully empty inventories while preserving user intent and offering bounded read recovery.
- **Why now:** two ordinary observations, including a fresh desktop reload, show false zero counts. The native batch returned QUERY_TIMEOUT for EnvGroups and OPERATION_NOT_STARTED for the three resource lists while successful Environments data still listed both owned members. Later healthy reads and real moves prove those members existed.
- **Scope:** allowlist availability rendering to Project Overview and its selected/Unassigned/membership-editor consumers. Retain the four hooks' existing error/refetch contracts and current GraphQL batch deadline. Dedicated t002 covers the shared-callers census and compatibility controls.
- **Relationship:** a new consumer gap beyond w4/blocked/m211's structured batch outcomes, w4/m221's creation selector and w4/blocked/m218's plan catalogs. It is not their closeout or a claimed regression of their repairs. Earlier grouping/polling guarantees are dispositioned in the finding.
- **Render parity included:** the tenant-facing inventory changes. [Render documents project resource visibility](https://render.com/docs/projects); authenticated Render timeout/retry behavior was not tested. Keep Bex's resource-preserving grouping-deletion semantics, which differ from Render's current documented cascading deletion.

## Evidence limits

The original native failed-list recorder retained exact operation documents and error arrays but coalesced absent/null data. Those entries are explicitly projections, not complete original response bodies. Complete independent membership and native mutation entries are retained separately; a later complete direct read batch passed. Mobile and second-desktop reloads were healthy controls. No production fault was injected and no runtime or membership loss was caused by the read failure.
