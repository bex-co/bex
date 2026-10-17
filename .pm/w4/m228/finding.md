# Display-only log changes skip the line being read

Why: inspecting older output is interrupted when a display preference replaces the current reading point.

**Severity:** minor; no backend log loss. Functional a896, 2026-10-17 UTC. Research HEAD `7e1de0a67fc95ce3bd8e3bd6a08b15c6fdea38d4`. Filing only.

## Reproduction and controls

Owned Free Existing Image service `qa-20261017-a896-log-options`, `srv-db9qrbkflioc739mcp90`, uses `node:24-alpine`, port 8096. The [complete evidence](evidence/a896-log-display-position.json) retains its native CreateService request and 676-character command: 180 numbered long Unicode lines over about four seconds, two newline-separated marker pieces, then a small JSON HTTP server. Single deploy `dep-db9qrbkflioc739mcp9g`. Display actions do not change the program or redeploy.

Open the completed deploy, choose Maximize, scroll up with the real wheel and align a fully visible numbered row at the pane's top using ordinary DOM scrolling. Both filed positions have enough following content to remain reachable.

| Fresh native case | Reading row before | Action | First intersecting numbered row after | Pane geometry |
| --- | --- | --- | --- | --- |
| Desktop 1440×1000 | ROW=130 at +0.5px; timestamps hidden | Show timestamps | ROW=140 at −6px;130 no longer visible | unchanged 1392×842px, client 840 |
| Mobile 390×844 | ROW=128 at +0.5px; wrapping/timestamps on | Disable Wrap lines | ROW=134 at −14px;128 no longer visible | unchanged 358×654px, client 652 |

Desktop before/after **17:25:47.649Z /17:26:05.269Z**. Mobile **17:24:02.923Z /17:24:26.545Z**; the wrong scroll position persists at **17:25:40.943Z**. Another mobile mid-window run skips 099→105. Wrapped mobile rows are 179.5px, unwrapped 23.5px. Neither filed example requires bottom clamping: the remaining numbered rows alone exceed the pane height.

Two complete GraphQL pages retain **182 unique records**, all numbered 001–180 and both marker pieces. Native search retrieves130 after it is skipped and001 after older history loads automatically. Unicode remains correct. Jump to latest and a subsequent pinned timestamp toggle keep the final live marker at the bottom; Exit full screen restores the inline viewer. Whole-buffer export/copy is deliberately unsupported and is not filed.

Four screenshots were opened and inspected. After-action captures emphasize the log pane; they do not establish toolbar loss or a second outer-layout bug. Durable viewport-relative bounds above are equal before/after. The JSON preserves selected geometry fields and complete native/API bodies; only repeated message/time strings are omitted from geometry projections.

## Producer, consumer and actual dependency

`dashboard/src/features/deploys/components/deploy-log-panel.tsx:104–112` owns display state; `:195–199` passes both flags to LogLineList. Checkbox handlers around`:293–309` change those local flags.

`features/logs/components/log-line-list.tsx:128–141` keys measurements by stable logical log keys. Wrapping at`:304` and the timestamp column at`:307–313` change row height without changing the records. The viewport/spacer/measurement construction at`:225–250` has no local native-scroll-anchoring policy.

The component captures a key plus pixel offset on scroll (`:150–168`). Its restoration effect (`:179–199`) takes the restoration branch only when `previousLines.current !== lines`. It does not reconcile display-only changes, invalidate old-mode geometry or bound an offset against a newly shorter row. Its other branch can adopt partially remeasured geometry as the new anchor. This missing application policy is shared by both traced triggers; stable IDs alone are insufficient.

**Locked code was inspected.** The lock pins react-virtual 3.14.20/core 3.19.5; local node_modules had core 3.17.7 and React 19.2.3 and was excluded from the mechanism claim. Cached locked sources/hashes show keyed sizes, mounted-row remeasurement and per-item scroll compensation. The public `measure()` resets sizes; the adapter coordinates notifications. Neither receives the consumer display mode. [TanStack's API](https://tanstack.com/virtual/latest/docs/api/virtualizer#measure) exposes reset and scrolling methods; an adjustment callback, if needed, is an instance property rather than a useVirtualizer option.

A standalone diagnostic bundles the **unchanged current LogLineList** with locked React/ReactDOM 19.3.1, scheduler 0.28.0, adapter 3.14.20 and core 3.19.5, captured production CSS and a constant array around captured messages. It uses a stable labels-only translation shim to avoid Start SSR; no list/scroll code is replaced. Other UI helpers use installed modules, clocks are fixed and two build bookends are synthetic. This is a supporting renderer experiment, not a complete production-bundle/Compiler attestation. Mobile127→133 and desktop130→140 reproduce without API/SSE/polling or data-array changes.

Local controls constrain the fix: disabling native overflow anchoring retains the desktop index but still loses mobile. A separate in-memory prototype adds display-triggered `measure()` to that CSS control and still loses the mobile target to the end. Neither partial control is a product fix or acceptance. The original diagnostic build is checked to bundle unchanged renderer bytes.

## Target and shared scope

Retain the logical key, intra-row offset and pinned intent **before** reflow; reconcile the affected measurements and conflicting browser/virtualizer adjustments; restore the key once new measurements are usable. Clamp the offset to the new row and position to the reachable scroll range. During settling, do not overwrite the saved target with transient geometry or treat a temporary clamp as a user jump to latest. Pinned readers still follow the end. A genuine near-end clamp is allowed without artificial blank padding.

Existing key/offset refs, boolean props, DOM sizes and public measure/offset/index APIs can express this in the shared component. Do not patch vendor internals, change logical identities, unvirtualize the buffer or equate raw scrollTop restoration with logical continuity. CSS-only or unconditional remeasurement already has a failing control.

Whole-source JSX census finds **three** production consumers: service LogViewer (`log-viewer.tsx:280`), DatastoreLogViewer (`datastore-log-viewer.tsx:258`) and DeployLogPanel (`deploy-log-panel.tsx:195`). The first two use default display flags. **One** DeployLogPanel mount is in `deploy-detail-page.tsx:136`, with **two** DeployDetailPage routes: `routes/services.$serviceId.deploys.$deployId.tsx:29` and `routes/static.$serviceId.deploys.$deployId.tsx:24`. App deployment families are web/private/worker/cron/static; PostgreSQL/Key Value use the datastore reader. Only web/image was live-probed; other-family controls belong to the audit.

Preserve data membership, chronology/identity, history/cursors, 5,000-row cap, bounded DOM, scroll-up/pin/jump, horizontal scrolling, chips, filters, pagination, pending/error states and visible-window selection. No REST/GraphQL/MCP payload or surrounding loading-geometry change is required.

## Dedupe and parity

The pre-filing scan covers **9,711 board Markdown files**, **92 open/blocked milestone titles**, DO_NOT_DO, last 40 product commits and targeted history. No open display-transition owner or later restoration fix was found. `4b6c73230` introduced the lines-change guard; `7510b1c79` introduced display controls. The exact current-source failure supports an unresolved source gap without assuming the serving SHA.

Shipping sync to `14abc7c604d8671b11e0c690000d4373e8cfa8a8` refreshed the scan to **112 pre-existing open/blocked milestones** and **9,948 board Markdown files**. New titles and renewed terms searches have no display-transition owner. All twelve researched source hashes still match, including the renderer, display owner and dependency lock. Incoming log map/types work concerns m217 identities; the constant-array case bypasses that mapping.

[w9/m83](../../w9/done/m83/README.md) owns virtualization/wrap rendering; [w4/m151](../done/m151/README.md) and [w4/m192](../done/m192/README.md) cover window/data/paging/chronology continuity. Preserve them. This uncovered display-only case is not claimed as an exact historically fixed regression. Agent-turn latency/Settings geometry search matches, m217 identities and m204 navigation filters are separate.

[Render's log documentation](https://render.com/docs/logging) covers runtime/deploy inspection, searching and filtering, without establishing this exact transition policy. No authenticated Render replay was run. The target follows Bex's unpinned reader behavior and preserves [ADR010](../../../docs/ADR010-observability.md)'s data contract.

Unverified live: other App families, datastore/service defaults, same-reader resize/rotation, disconnected/pending/error variants, 5,000-row busy tails, other browsers and the corrected implementation. These are audit/regression work, not observed failures. The early near-end clamp and an attempted Load older click after automatic paging are excluded. The actual restore button is Exit full screen, not Minimize.

## Cleanup

Native DeleteService returned true **17:35:33.869Z**. Service/deploy REST 404 and seven exact baseline inventories pass **17:36:28.435Z**. **21** runtime scopes/public 404 pass **17:36:25.197Z**; two metadata-only Secret/ConfigMap snapshot scopes across namespaces are empty **17:39:54.729Z**. The Free app was normally Hibernated by deletion, not a defect.

Exact logout: whoami 401, zero cookies, one blank page **17:36:34.726Z**; handles removed, wrapper exit 0. Both local server handles exited0 after explicit shutdown. Browser remains zero-cookie/one blank page **17:43:58.700Z**. No direct object-store listing is claimed.

Network metadata retains aborted DeployActions and two closed ViewerCapabilities reads without attributing an incident. Later 401/403 reads cluster in intentional logout-to-blank cleanup; the flow query id is redacted. Final selected-page console errors: 0. No product implementation or product-suite execution. Evidence contains **31 selected records, 12 repository source excerpts, pinned-dependency references and 37 passing checks**.
