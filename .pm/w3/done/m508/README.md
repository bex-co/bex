# w3 · m508 — A project's Service Metrics view charts every service together, as Render documents

**Worker:** worker3 **Goal:** the project's Service Metrics view (the `METRICS` saved search w3/m505 creates) charts CPU, memory, instances, HTTP requests, response time and bandwidth for every service of the project or environment, one series per service, grouped as Render's bundle allows. **Status:** done — shipped in `2e5170b87`; verified on dev-3 and on production pin `3e53abbe3bc3` (§ Verification record), closed 2026-11-20 (§ Closeout)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Re-check Render's Service Metrics view and record what it shows — **DONE** | 30m | — |
| [t002](done/t002.md) | Metrics for a project scope with a group-by — **DONE** | 1h30m | t001, w3/m505/t003 |
| [t003](done/t003.md) | Dashboard: the Service Metrics view page — **DONE** | 2h | t002, w3/m505/t006 |
| [t004](done/t004.md) | Partial reads, resources without data, the fan-out bound and Hobby — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Service Metrics in New View and Edit view — **DONE** | 30m | t004, w3/m507/t003 |
| [t006](done/t006.md) | Verify on dev-3 and on production — **DONE** | 30m | t005 |
| [t007](done/t007.md) | ADR010 and ADR018 — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 30m | t008 |
| [t011](done/t011.md) | Closeout — **DONE** | 10m | t009, t010 |

## Definition of done

- The project sidebar lists the Service Metrics view; its page charts CPU, memory, instances, HTTP requests, response time and bandwidth with one series per service of the project (or the filtered environment).
- Plan gates and retention apply per service; unreadable services are absent.
- Members can create and edit metrics views (w3/m507).
- Verified on dev-3 and production at 1440 and 390.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first: Render's docs describe the Service Metrics view, and its bundle and default saved searches carry it.
- **Recommendation, not a user decision:** in the reference workspace Render hid the view on 2026-11-19 (no sidebar entry, no New View option, the URL redirects). The recommendation is to build it from Render's docs, which describe it as available, under parity first; t001 re-checks first. Holding it until Render shows it everywhere is the alternative, and that is the user's call.
- **Expected outcome:** a project's resource use can be compared across its services on one page.
- **Why now:** w3/m505 creates the `METRICS` default view, and the backend already fans out over several resources, so the remaining work is mostly the page.
- **Render parity included:** GraphQL and dashboard changes.

## Verification record

### dev-3 (2026-11-19)

A fresh dev-3 identity's Hobby workspace `tea-dbvmh9pjg4r6oemv01eg` with project `prj-dbvmhapjg4r6oemv01f0` ("observed"): environments production and staging, busybox web services `storefront` and `api` (production) and `admin` (staging), and a cron job `nightly-report` (staging, every minute). dev-N's shared Prometheus scraped no cAdvisor series, so every CPU, Memory and Total Instances chart on dev-N, a service's own included, was empty; it now keeps cAdvisor's two container series (`scripts/dev-env/prometheus/prometheus.yaml`). dev-N ships no request lines to its log store, so Total Requests and Response Times stay empty there (production reads them).

- GraphQL `projectMetrics`: CPU, Memory and Total Instances answered one series per resource labeled `resource` (three web services and the cron job); `resources` listed all four `CHARTED`. Total Requests listed `nightly-report` as `NOT_CHARTED`. An `ENVIRONMENT IS staging` document narrowed the read to `admin` and `nightly-report`. `groupBy: NONE` answered one unlabeled Memory series; `STATUS_CODE` on CPU was refused by name. Response Times answered `PLAN_FEATURE_REQUIRED` on Hobby.
- Dashboard, 1440: the sidebar listed Application Logs, Request Logs and Service Metrics; `/observability/metrics` without a view redirected to the project's Service Metrics view. At Last hour, CPU, Memory and Total Instances each drew four lines in the legend's four hues; the legend's `api` toggle hid its line on all four drawn charts and showed it back. Total Requests' Group by offered Resource, Status code and All requests, and its note read "Not charted for nightly-report". Response Times showed the Pro notice. At 390 the page fits with no horizontal scroll.
- New View → Service Metrics opened "New view" named "Service Metrics 2"; Environment = staging and Resource = admin saved and opened the view, its legend `admin` only. Edit view (rename "Staging without admin", Resource `is not one of`) left `nightly-report`; Duplicate opened "Staging without admin (copy)" on the metrics page, and deleting it landed on the project's first remaining view.
- Pending and ready (the saved-search read held back in the browser): at 1440 and 390 the header, Edit view, range picker, legend row, both card tops and every chart title have the same boxes in both states, up to Total Requests. Below it the ready page adds the data-dependent "Not charted" note (20 px at 1440, 36 px at 390), which no skeleton can know.
- Found and fixed on the way: Apollo normalized `ProjectMetricResource` by id, so the CPU read's `CHARTED` overwrote the Total Requests read's `NOT_CHARTED` for the cron job (`keyFields: false`, with a cache test).

### Production (2026-11-20)

Pin `3e53abbe3bc3`, as the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`; Response Times charted, as Pro and higher do). The fixture project `qa-20261120-m508` (`prj-dbvuh0529hjs73bcfqp0`) had environments production (`evm-dbvuh0529hjs73bcfqpg`) and staging (`evm-dbvuh0529hjs73bcfqq0`). Five resources were created over REST: `traefik/whoami` web services `storefront` (production, `srv-dbvuh2l29hjs73bcfqrg`), `api` (production, `srv-dbvuh51bgjqs73fpskrg`) and `admin` (staging, `srv-dbvuh5529hjs73bcfqtg`); a static site `docs` (production, `srv-dbvuh7pbgjqs73fpskvg`, `octocat/Spoon-Knife`); and a busybox cron job `nightly` (staging, `srv-dbvuh59bgjqs73fpsktg`, every minute), each named `qa-20261120-m508-<name>`. A `curl` loop sent traffic from 06:32 UTC: storefront's `/` and `/cart`, api's 64 KiB `/data`, admin's `/`, and docs' `/` plus a missing path that answers 404. The cron job ran for about a second, too short for a scrape, so it had no line ("No data in this range"). Its command was lengthened to 40 s and deployed, as on Render, where a settings change applies on the next deploy, and from 06:40 it charted.

| check | production | result |
| --- | --- | --- |
| GraphQL `projectMetrics`, last hour | CPU, Memory and Total Instances: one series per web service and the cron job, labeled `resource`. `docs` reads `NOT_CHARTED` (a static site has no container of its own). Total Requests and Response Times (p95): the three web services and `docs`, with `nightly` `NOT_CHARTED`. Outbound Bandwidth: all five | holds |
| Group-by | `HTTP_REQUESTS` with `STATUS_CODE` answered `statusCode` 200, 404 and 503 (503 from the first probes, before the deploys went live). `NONE` answered one unlabeled series. `STATUS_CODE` on CPU was refused: `groupBy STATUS_CODE applies only to http_requests` | holds |
| `ENVIRONMENT` document | staging: `resources` `admin` and `nightly`. production: `api`, `docs` and `storefront` | holds |
| Sidebar, 1440 | Overview; Observability with Application Logs, Request Logs, Service Metrics and + New View; Manage → Settings. Service Metrics opened `/observability/metrics?savedSearchId=svs-86kfdn370ebncgcdfqka`, titled "qa-20261120-m508 / Service Metrics", at Last 12 hours | holds |
| Charts, 1440, Last hour | the legend listed all five in five hues. CPU, Memory and Total Instances each drew one line per web service, plus `nightly` once it charted; each had the note "Not charted for qa-20261120-m508-docs". Total Requests and Response Times drew 4 lines and noted `nightly`. Outbound Bandwidth drew 5. The browser console logged no errors | holds |
| Total Requests Group by, 1440 and 390 | the options read Resource, Status code and All requests. Status code drew 200, 404 and 503 with their own legend; All requests drew one summed line | holds |
| Legend, 1440 and 390 | the `api` toggle at 1440 and the `storefront` toggle at 390 each removed that resource's line from all six charts, and the second click brought it back | holds |
| Edit view `ENVIRONMENT`, 1440 | Add filter offered Environment `is` with values production and staging. staging left the legend `admin` and `nightly` and one line on each chart (two on Outbound Bandwidth). production left `api`, `docs` and `storefront`, and CPU's note named `docs`. Removing the row restored all five | holds |
| New View, 1440 | the TYPE menu read Application Logs, Request Logs and Service Metrics. Service Metrics opened "New view" named "Service Metrics 2". Renamed "qa-m508 storefront and api" with Resource `is one of` those two, Done opened `svs-dbvuoo1bgjqs73fpsl1g`. It was listed in the sidebar, and its legend held those two only | holds |
| 390 | no horizontal scroll (scroll width 390); the charts are 316 px wide | holds |
| Pending and ready, 1440 (the `SavedSearches` read held back 8 s, read 2.5 s after the sidebar click) | the eyebrow (72–88), Edit view (92–124), the range picker (148–180), the legend row (204–232), the first card's top (256) and the CPU title (333) are the same in both states. Below CPU each "Not charted" note adds 20 px to the ready page: Memory +20, Total Instances +40, the second card and Total Requests +60, Response Times +80 and Outbound Bandwidth +100, as on dev-3 | holds; the notes depend on the data |
| Pending and ready, 390 | the eyebrow (64–80), Edit view (128–160) and the range picker (184–216) are the same. Five 28-character fixture names wrap the ready legend to four rows (240–376, against the skeleton's 240–268), which moves both cards down 108 px. Inside each card the title and the first chart keep their offsets, and each "Not charted" note adds 36 px, as on dev-3 | holds; the legend's height and the notes depend on the data |

**Cleanup.** The five services were deleted (`DELETE` 204 each), then the project (`DELETE /v1/projects/…` 204). By-id `GET` answered 404 for the five services, the project and both environments. `savedSearch` answered `NOT_FOUND` for the default Service Metrics view and the new one. The app cluster kept no App, Deployment, CronJob or Pod of them. The QA session was logged out (`qa-login.sh --logout`, after which its cookie answered 401). The session jar, the fetched kubeconfig and the screenshots were deleted.

## Gate (2026-11-19)

**Clears by itself; no decision needed.** t006's production half needs a production deploy carrying `2e5170b87`. Check: the newest `chore(deploy): pin platform images to <sha>` on `main` passes `git merge-base --is-ancestor 2e5170b87 <sha>`, and production's `bex-api` runs that digest.

**Next (t006 step 1, then t011):** sign in as the QA user (`bash scripts/qa-login.sh`) and, in `bex-canary`, use or create a project of the QA user's own with at least three services that take traffic (and, if there is one, a worker or cron job and a static site). Open its Service Metrics view from the sidebar at 1440 and 390: one line per service on CPU, Memory, Total Instances, Total Requests and Outbound Bandwidth (Response Times too when the workspace is Pro, else the plan notice), Total Requests grouped by status code and by none, the legend hiding a service on every chart, and an `ENVIRONMENT` filter (Edit view) narrowing the lines. Compare pending and ready at both widths. Record the evidence here, delete any fixture with by-id 404 evidence, log out, and close the milestone.

## Unblock (2026-11-20)

**Source:** verified evidence (`/pm unblock`, `/loopx w3` worker). **Affected:** w3/m508 t006, t011. **Finding:** the gate cleared. The newest pin commit on `main` is `a47204c7e chore(deploy): pin platform images to 3e53abbe3bc3`, and `git merge-base --is-ancestor 2e5170b87 3e53abbe3bc3` succeeds. It pins `ghcr.io/bex-co/bex-operator@sha256:3cfea3acbc67…` and `ghcr.io/bex-co/bex-dashboard@sha256:b72fe48b24c8…`. Read with `kubectl` on the control plane at 06:24 UTC: both `bex-api` pods (since 04:28 UTC) and `bex-controller-manager` (since 04:25 UTC) run the operator digest, and the `dashboard` pod (since 04:29 UTC) runs the dashboard digest. **Remaining gates:** none. **Next:** t006's production walk as § Gate says, then t011.

## Closeout (t011, 2026-11-20)

- **The sidebar lists the Service Metrics view, and its page charts CPU, memory, instances, HTTP requests, response time and bandwidth with one series per service of the project or the filtered environment:** dev-3 and production at 1440 and 390 (§ Verification record). A static site has no CPU, Memory or Total Instances series and is named "Not charted" there. A cron job has no request series and is named the same way.
- **Plan gates and retention apply per service; unreadable services are absent:** Response Times charted on production's Scale workspace, and dev-3's Hobby workspace got the Pro notice. Each resource is read through the per-resource Metrics verb, so its own retention applies (`TestProjectMetricsLatencyFollowsThePlan`, `TestProjectMetricsChartEveryResource`). The QA user is production's only credential, so the absent unreadable resource rests on `TestProjectMetricsChartEveryResource` (a Postgres the caller cannot read is never named) and `TestProjectMetricsRefusals` (an unreadable project is refused).
- **Members can create and edit metrics views:** New View → Service Metrics and Edit view's `ENVIRONMENT` and Resource rows, on dev-3 and on production.
- **Verified on dev-3 and production at 1440 and 390:** both records above. At both widths pending and ready match down to the first data-dependent note, and at 390 the legend's wrap is data-dependent too.

## Render parity (t008)

Compared 2026-11-19 against [render.com/docs/projects](https://render.com/docs/projects) § Observability, [render.com/docs/service-metrics](https://render.com/docs/service-metrics) and the capture ([project-observability.md § Service Metrics](../../../../docs/render-artifacts/project-observability.md#service-metrics-re-checked-2026-11-19-w3m508)). Render's page was hidden in its reference workspace, so nothing below is matched against a live page.

| Surface | Render | bex | Verdict |
| --- | --- | --- | --- |
| Route | `/project/{prj}/observability/metrics` (bundle) | the same, `?savedSearchId=…&r=…` | route matched; the page itself unverified |
| Sidebar, New View | docs: a default view under Observability; the reference workspace hid it | listed and offered | follows the docs (ADR018) |
| Charts | docs: CPU, memory, HTTP requests, response times, bandwidth per service | CPU, Memory, Total Instances, Total Requests, Response Times, Outbound Bandwidth | unverified chart set |
| Scope | views filter by Environment, Resource, Resource Type | the same document; a static site counts as a Service; Postgres and Key Value chart CPU and memory | unverified for metrics views |
| Group-by | bundle enum `NONE`, `RESOURCE`, `STATUS_CODE` | the same values on GraphQL; the page offers them on Total Requests | values matched; placement unverified |
| Response times plan | Pro and higher | the Pro refusal on Hobby, read once | matched |
| Retention | per plan | per resource through the Metrics verb | matched |
| GraphQL read | private, not captured | `projectMetrics`, a Bex extension | recorded in ADR018 |
| REST, MCP | no project metrics operation | unchanged, explicit resources | matched |

Across bex's surfaces: `projectMetrics` reads each resource through the verb GraphQL `metrics`, REST and MCP read, so units, retention, creation floors and the latency gate agree. One drift found: a status-code series is `statusCode` on REST, MCP and `projectMetrics` but `code` on GraphQL `metrics`; filed as [w3/080](../../080.md).

## Test coverage (t010)

| Behavior | Test |
| --- | --- |
| A three-service project returns three CPU series labeled by resource, instances added up | `TestProjectMetricsChartEveryResource` (`lego/backend/internal/metrics/project_test.go`) |
| The view's document narrows the scope; a log field in it is refused | `TestProjectMetricsFollowTheViewDocument`; `TestScopeSelectsWhatAViewReads` (static sites as Services) |
| Group-by NONE, STATUS_CODE and their refusals | `TestProjectMetricsGroupBy` |
| Hobby gets the response-time refusal before any read | `TestProjectMetricsLatencyFollowsThePlan` |
| A failed resource is FAILED and adds nothing; all failed answers the error | `TestProjectMetricsPartialAndFailedReads` |
| The 20-resource bound, coded; no store; an unreadable project | `TestProjectMetricsRefusals` |
| GraphQL shape | `TestGraphQLProjectMetrics`, the schema golden |
| Lines, legend toggle, group-by, notes, too large, empty, Hobby, loading | `project-metrics-view.test.tsx`, `project-metrics.test.ts` |
| The page: scope, 12-hour default, not found, skeleton | `project-metrics-view-page.test.tsx` |
| Sidebar lists and offers Service Metrics | `project-sidebar.test.tsx` |
| A metrics view's Edit view fields and static sites | `edit-view-dialog.test.tsx` |
| Each chart keeps its own resource status in the cache | `cache.test.ts` |

## Simplify (t009)

`/simplify` folded the two view pages' shared frame (loading, not found, failed read, header and range) into `ProjectViewPage`, and the three per-type view links (sidebar, not-found states) into `ProjectViewLink`; the GraphQL `parameters` parsing is one helper for `metrics` and `projectMetrics`.

