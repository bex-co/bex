# w4 · m138 — Blueprint detail: a sync never appears in Sync History, and the plan hides create vs update vs no-change

**Worker:** worker4 **Goal:** the blueprint detail page tells the truth about syncs: a sync that just succeeded appears in Sync History without a reload, and the pre-sync plan shows what will happen to each resource (create, update with the changed fields, no change, detach), the way the backend already classifies it **Status:** todo

## Tasks (in order)

| id   | title                                                                                         | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Sync History refreshes after a dashboard sync and while a blueprint is open                   | 30m | —          |
| t002 | The plan summary shows each action's operation and changed fields, and counts real changes     | 40m | —          |
| t003 | Sync dialog copy names what is applied: the reviewed repository commit, not "the stored render.yaml" | 10m | —          |
| t004 | Render parity across REST / GraphQL / MCP / UI                                                | 20m | t001, t002, t003 |
| t005 | Simplify                                                                                      | 15m | t004       |
| t006 | Test coverage                                                                                 | 30m | t004       |
| t007 | Closeout                                                                                      | 10m | t006       |

## Definition of done

Every bullet was probed at filing (pass 161) and can be repeated with a throwaway blueprint: Public Git URL `https://github.com/bex-co/bex`, branch `main`, path `examples/static-site/render.yaml`, name `qa-<date>-bp`. First confirm no service named `static-site` exists. Disconnect it and delete `static-site` afterwards.

- **A dashboard sync shows up.** On the blueprint page, click Sync, then Sync in the dialog, and stay on the page without reloading. Within one poll interval, Sync History gains a new top row for that run, matching `blueprintSyncs(id:)`'s newest entry. At filing: after a sync at `05:33:04Z` that the API recorded (`bsr-…`, `state: success`, 14 rows), the page kept showing 13 rows for a 60s watch while its other background polls fired. Only a reload showed the new row.
- **The plan says "no change" when nothing changes.** Open the Sync dialog on an unchanged blueprint. The summary says the resource is unchanged (for example "static-site: no changes"), not "1 resource to sync". At filing, `blueprintPreview` returned `actions: [{operation: "noop", changedFields: [], name: "static-site", resourceId: "srv-darlf1muag9c73eddbdg"}]`, `totalActions: 1`, and the dialog read "Blueprint file parsed successfully — 1 resource to sync. Services: static-site". The subsequent sync created no deploy (`deploys(serviceId:)` stayed `[dep-darlf1muag9c73eddbe0:live]`), so the backend's `noop` was right and the dashboard's wording was not.
- **Create and update are distinguishable before applying.** On the New Blueprint review step, a resource that does not exist yet is labelled as a create. An existing resource whose manifest differs is labelled as an update and lists `changedFields[].path`. t002 exercises the update case with a manifest variant.
- **The dialog describes the right source.** The Sync dialog no longer says "re-applies the stored render.yaml" while it computes and pins the plan from the repository commit it names ("Reviewed commit c827ec0b").

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 161, 2026-09-25 (w4-targeted, `muse.env` credentials), journey 13 (Blueprints). The fixture blueprint `qa-20260925-bp` (`blp-dan9g8rs0ils73bgp500`, a revived row per the deliberate `w4/120` design) and its `static-site` (`srv-darlf1muag9c73eddbdg`) were created and removed within the pass. The site served 200 before deletion and 404 after. Evidence (local): `.playwright-mcp/qa-bp-sync-1.png`.
- **Root cause, stale history:** `dashboard/src/features/blueprints/hooks/use-blueprint-syncs.ts:20-24` runs `BlueprintSyncsDocument` with `cache-and-network` and **no** `pollInterval`, unlike its siblings `use-blueprint.ts:25` and `use-blueprints.ts:31` (`RESOURCE_POLL_INTERVAL_MS`). `use-sync-blueprint.ts:43` calls `useMutation(SyncBlueprintDocument)` with no `refetchQueries` or cache update, and the route (`routes/blueprints.$blueprintId.tsx:198`) ignores the hook's `refetch`. Nothing re-reads history after a sync, so pushes that trigger auto-sync while the page is open are equally invisible (reasoned, not probed).
- **Root cause, plan summary:** `dashboard/src/features/blueprints/components/blueprint-plan-summary.tsx` renders only name groups (`plan.services`, `databases`, and so on) and uses `plan.actions` solely to find `detach` rows. It never shows `operation` or `changedFields`, although both are fetched (`features/blueprints/api/blueprints.graphql:100-110`) and mapped (`lib/views.ts:132`). The headline `blueprints.previewValid_*` ("{count} resource to sync", `locales/en.ts:216-224`) takes `plan.totalActions`, which counts `noop` actions (`lego/backend/internal/apps/blueprint.go:516`). Two call sites share the component: `routes/blueprints.new.tsx:369` and `routes/blueprints.$blueprintId.tsx:641`.
- **Not a regression, not a duplicate:** `w6/m125` made the backend classify create/update/noop correctly (its goal: "so a user can tell whether applying a manifest will provision new infrastructure or modify what they already have") and verified REST/GraphQL/MCP, but its DoD never covered the dashboard rendering. `w6/043` fixed Sync History's loading flash, not staleness. `w4/m133` added the detach alert, which is the one operation the dashboard does render.
- **Target behavior:** the backend's `operation` is authoritative. The UI shows per-resource create / update (+ changed field paths) / no change / detach. The headline counts only actions whose operation is not `noop` (say "no changes" when that is zero). History refreshes on mutation success **and** polls on the shared visible-tab cadence while the page is open.
- **Goal linkage:** ADR049 (render.yaml parity: the plan is bex's `terraform plan`), ADR006. Render's Blueprint sync preview lists per-resource changes before you confirm.
- **Expected outcome:** a user can tell from the dialog whether clicking Sync will change anything, and can see that it ran.
- **Why now:** the backend work (`w6/m125`, `w4/m133`) is done and live, and the dashboard is the last layer discarding it. Both fixes are small and local to one feature folder.
- **Render parity task included:** UI behavior changes. REST/GraphQL/MCP should not move, and t004 confirms `totalActions` semantics stay as they are for API clients.

## Unverified

- Auto-sync-on-push leaving history stale while the page is open was reasoned from the same hook, not observed (no push was made to the fixture branch during the watch).
- The update case (`operation: "update"` with non-empty `changedFields`) was not produced live this pass, because the fixture file cannot be edited. t002 must produce it (for example via `validateBlueprint` with an edited manifest).
