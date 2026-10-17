# Service and deploy log searches drop pre-handler typing

Why: both log-search controls accept diagnostic text during startup, then silently replace it with their empty controlled state.

**Severity:** minor for each root. Ordinary QA a838, 2026-10-17 UTC; extend m219 through t011/t012. Research main2d0766a062bd8d8790b0c1b81fc1aad0c961d56d; refreshed after af6a6310e, which changes only the webhook readiness root. The first five roots are now implemented, including the webhook gate in af6a6310e; these two log searches remain ungated.

## Six fresh replays

On owned Free web srv-db9emes50quc73dl5p2g, hold the real fetched /assets/index-D60ymwlM.js response, type with the browser keyboard, release unchanged, and sample20 times. Each response is HTTP200/1,952,882 bytes. [Durable evidence](../evidence/m219-a838-log-search-startup.json) includes exact scripts, all six complete receipts, source census and cleanup.

| Root | Fresh widths | Result |
| --- | --- | --- |
| /services/{id}/logs — Search logs | 1440,390,repeated390 | Enabled field without React props accepts trusted marker; later empties on the same node; ready marker retained. |
| /services/{id}/deploys/{deployId} — Search logs… | 1440,390,repeated390 | Same failure and ready control, independently captured. |

All20 samples per replay retain node identity. The deploy selector uses its real placeholder; it has no aria-label. Four mobile before/after screenshots were inspected: .playwright-mcp/qa-a838-{service,deploy}-search-{before,after}.png. Arriving select labels/header/deploy metadata also change in these frames, but this finding concerns the unchanged input nodes, not their surrounding layout.

**Target:** keep the two search inputs visible with the same geometry but natively uneditable until handlers attach. After readiness, exact typing survives normal updates and existing search semantics remain unchanged. No global Input or hydration-hook change.

## Two located causes and consumers

1. **Service logs:** features/logs/components/log-filter-bar.tsx:185–192 binds Input to filters.text and onChange without a readiness gate. LogViewer initializes viewFromUrl state at :78 and passes it to its one LogFilterBar mount at :298. ServiceLogsPage has one LogViewer mount, routes/services.$serviceId.logs.tsx:116, and one route-wrapper mount at :65. LogFilterBar has two direct Input sites; only this search site is targeted, and the second site at246 stays unchanged. Gate this field locally; preserve URL-owned filters, text debounce, range, Live behavior and m204 navigation adoption.
2. **Deploy logs:** features/deploys/components/deploy-log-panel.tsx:102 initializes local search to empty, and :241–246 binds the ungated Input. It has one production mount in DeployDetailPage:136, with two route mounts: services.$serviceId.deploys.$deployId.tsx:29 and static.$serviceId.deploys.$deployId.tsx:24. Add its own local gate while preserving debounce, literal matching/m216, type/instance filters, partial history, live-tail merging and per-deploy reset.

The App compute route covers web/private/worker/cron; Render-style web/pserv/worker/cron prefix aliases preserve suffix/query through render-alias.ts. Static deploy history uses the second DeployDetailPage route. There is no separate static LogViewer route in the route-file census; do not add one. Only web was live-driven. Postgres and Key Value use the distinct DatastoreLogViewer, with two route mounts, and are not claimed affected here. Aliases and non-web variants belong in the implementation audit.

The pinned React19.3.1 initInput/updateInput path was reopened for a837 and applies to these controlled native inputs too: hydration can preserve early DOM text, followed by a controlled update restoring component state. No specific later query or independently extracted deployed React version is claimed.

## Dedupe, scope and verification

Open/blocked/done searches, all84 current open/blocked milestone titles, DO_NOT_DO, recent40 product commits and targeted component history found no readiness gate. Existing m204 URL adoption and m216 matching occur after handlers; m192/m198 concern stream/history retention. These fixes remain intact. This extends the same original startup defect in m219, rather than reopening those unrelated guarantees or claiming their full acceptance matrices were rerun.

[Render logging docs](https://render.com/docs/logging) establish service search and individual deploy logs, not authenticated startup timing. No signed-in Render startup comparison was made. Preserve REST/GraphQL/MCP log fields, filters and errors.

**Unverified:** nonempty URL search, non-web types/aliases, read-error states, Chinese, reduced motion, DatastoreLogViewer and detailed ready filtered results. Ready controls prove exact retained input text only. Tasks cover relevant regression work without treating these as observed failures.

Startup probes submit no create/deploy action. The supporting Free web fixture was created separately, served200 and emitted ordinary markers, then was deleted. Parent/deploy return404, seven inventory ID sets match, own App/serving/build children are gone and public URL404. Exact-session logout succeeded: whoami401, cookies0, one blank page and own handles removed. Delay handlers are removed in finally.
