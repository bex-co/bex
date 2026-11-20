# w3 · m523 — A project log view reads `is not one of` on Level, Instance, Method, Status Code, Host and Path, as Render's Edit view offers

**Worker:** worker3 **Goal:** a project's Application Logs and Request Logs views take Render's `IS_NOT` on every line field Render's Edit view offers it on (Level, Instance, Method, Status Code, Host, Path). The log query reads the complement on the store, the pod-log and datastore paths and the live tail, the validator accepts the operator, and the Edit view modal offers `is not one of` there. A view such as "everything but info" reads instead of being refused. **Status:** done — shipped in `9f497080c`; verified on dev-3 (2026-11-19) and on production pin `12716d1d7097` at 1440 and 390 (2026-11-20)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's per-field operators and document shape in the capture and ADR010, and file the document-shape divergence — **DONE** | 30m | — |
| [t002](done/t002.md) | Backend: negated value sets on `LogQuery`, honored by the Loki selector and pipeline — **DONE** | 1h | — |
| [t003](done/t003.md) | Backend: the pod-log, datastore and live-tail paths honor the negated sets — **DONE** | 1h15m | t002 |
| [t004](done/t004.md) | Backend: the validator accepts `IS_NOT` on the six fields, the project view routes it, ADR010 and ADR018 drop the refusal — **DONE** | 30m | t001, t003 |
| [t005](done/t005.md) | Dashboard: the Edit view modal offers `is not one of` on the six fields — **DONE** | 45m | t004 |
| [t006](done/t006.md) | Verify on dev-3, and on production at 1440 and 390 — **DONE** | 45m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t008, t009 |

Total: **10 tasks, about 6h30m**.

## Definition of done

1. **The note's check.** A project view whose document holds `LEVEL IS_NOT [info]` reads every line of the project's services, cron jobs, Postgres and Key Value whose level is not info, a line with no level included. It does so on GraphQL `logs(project, filterDocument)` and on the `/v1/logs/subscribe?project=` tail, with a test per surface.
2. **Every negatable field.** `IS_NOT` on `INSTANCE`, `METHOD`, `STATUS_CODE` (a `4xx` class too), `HOST` and `PATH` reads the complement of what `IS` reads, on both surfaces. `*` wildcards and `/regex/` values negate the same way. A negated value that matches nothing excludes nothing. An `INSTANCE IS_NOT` that resolves in one resource and not in another narrows only the first.
3. **The rest stays as Render has it.** `ENVIRONMENT` and the time range take `IS` only. Text Search takes Render's `contains` only. `IS_NOT` on any of them is refused by name, as before.
4. **The editor.** The Edit view modal offers `is one of` / `is not one of` on Level, Instance, Method, Status Code, Host and Path, as it already does on Resource and Resource Type. A stored `IS_NOT` row reopens as `is not one of`.
5. **Recorded.** ADR010 § Project views no longer lists the refusal, and the ADR018 Projects row and the [capture](../../../../docs/render-artifacts/project-observability.md) name Render's operators per field.
6. **Live.** On dev-3, and on production once a pin carries the milestone, as the QA user in `bex-canary`, at 1440 and 390: a view edited to `Level is not one of info` reads no info line, in history and live.

## Verification record (t006)

### dev-3 (2026-11-19, from this tree)

`bash scripts/dev-env.sh 3 up` rebuilt bex-api from the tree. A throwaway identity's workspace `tea-dbvrc0hjg4r79qlurcg0` (Hobby) with project `prj-dbvrc1pjg4r79qlurcgg` ("observed", environment production): a busybox web service `storefront` writing a JSON `info` line, a JSON `warn` line, an unstructured line and a JSON line with an empty `level` every 6 s, a cron job `nightly-report` (`cron info` and `cron error` every minute), and a Postgres `orders-db`.

- **The validator.** `updateSavedSearch` stored the Application Logs view (`svs-kkgr343vfhavejtse4b9`) as `LEVEL IS_NOT [info]`; `TEXT IS_NOT` answered `filter field "TEXT" does not support operator IS_NOT`.
- **The tail** (`/v1/logs/subscribe?project=…&type=app`, SSE, 15 s each). `LEVEL IS_NOT info`: storefront's warning (20), unstructured (`unknown`, 20) and no-level (20) lines, no info line, and none of the Postgres's `LOG` (info) lines. `LEVEL IS info`: storefront's 22 info lines and 7 Postgres `LOG` lines only. `INSTANCE IS_NOT <storefront's instance>`: only the Postgres's lines; `INSTANCE IS` it: only storefront's.
- **History** (GraphQL `logs(project, type: "app", filterDocument)`, last 10 min). dev-3's log shipper stores every App line with `level="unknown"` (`scripts/dev-env/values/log-shipper.values.yaml` does not parse levels), and ships no datastore lines, so the store has no info line: `LEVEL IS info` read none and `LEVEL IS_NOT info` read all 96 storefront lines, its complement; `LEVEL IS_NOT unknown` read none against `LEVEL IS unknown`'s 96. `INSTANCE IS_NOT <storefront's instance>` read only `nightly-report`'s 6 lines, `INSTANCE IS` it only storefront's 100. The real-level history read rests on `TestNegatedFiltersReadTheComplementOnThePinnedLoki` and the production walk.
- **The dashboard** (a local dashboard against dev-3, signed in as that identity). At 1440, Edit view reopened the stored row as `Level` `is not one of` `info`, the operator enabled. Rebuilt from the + button: a new Level row read `is one of`, its operator offered `is one of` and `is not one of`, and Done saved `IS_NOT`. Edited to `Instance is not one of <storefront's instance>` (typed), the page read no storefront line, `nightly-report` from history and `orders-db` live. At 390 the same view read no storefront line, and Edit view showed `is not one of`.
- **Found and fixed at 390:** a long typed value (the instance id) widened the modal's rows 51 px past its right edge, clipping the Name field, the + button and the notice. The modal's grid column is now `minmax(0, 1fr)` (`edit-view-dialog.tsx`), so the chip truncates: re-measured, the modal's content is 356 px in a 356 px box at 390 and 672 px wide at 1440, as before.
- **Not walked on dev-3:** Request Logs. dev-3's shipper ships no request lines and the workspace is Hobby; the request fields rest on `TestProjectRequestViewNegatesEveryRequestField`, the pinned-Loki test and the production walk.
- **Cleanup:** both services, the Postgres and the project answer 404 by id, the identity was deleted through Kratos' admin API, and the browser session was logged out.

### Production (2026-11-20, pin `12716d1d7097`)

Pin `4e37ec03f` (`12716d1d7097`) carries `9f497080c`, and production's `bex-api` and `dashboard` ran its digests (§ Unblocked). As the QA user in `bex-canary`, the fixture project `qa-20261120-m523` (`prj-dc00n1cd1ruc73b8cedg`, environment production `evm-dc00n1kd1ruc73b8cee0`) held three resources created over REST:

- a busybox web service `qa-20261120-m523-web` (`srv-dc00n5kd1ruc73b8ceg0`, starter). Every 6 s it wrote a JSON `info`, `warn` and `error` line, a JSON line with no level and an unstructured line. `httpd` on `$PORT` served `/` (200), and `/healthz` and `/missing-m523` answered 404;
- a cron job `qa-20261120-m523-cron` (`srv-dc00n5sd1ruc73b8cei0`, every minute, a JSON `info` and `error` line);
- a Postgres 17 `qa-20261120-m523-pg` (`dpg-dc00n5p8bccs73evqjng`, `basic_256mb`).

`curl` requests to `/`, `/healthz`, `/missing-m523` and `/other-m523` gave Request Logs its lines. The project's Application Logs (`svs-89ccua5uhtsplebo2f6i`) and Request Logs (`svs-5c8bnonbsh87tnq7p5mj`) views were `{"filters": [], "version": 1}` before the walk.

| check | production | result |
| --- | --- | --- |
| GraphQL `logs(project, type: "app", filterDocument)`, last 30 min | the empty document read 69 lines. `LEVEL IS info` read 15 (web 13, cron 2). `LEVEL IS_NOT info` read the other 54: web warning 13, error 13 and `unknown` 26 (the level-less JSON and the unstructured lines), and cron error 2. No info line | holds |
| GraphQL `logs(project, type: "request", filterDocument)` | the empty document read 9 (`GET 200 /`, `404 /healthz`, `404 /missing-m523`, 3 each). `STATUS_CODE IS 2xx` read the 3 `200 /`, `IS_NOT 2xx` the 6 404s, and with `PATH IS_NOT /healthz` added the 3 `/missing-m523`. `PATH IS_NOT /healthz` alone read `/` and `/missing-m523` | holds |
| Tail `/v1/logs/subscribe?project=…&type=app&filterDocument=…`, SSE, 25 s each | `LEVEL IS_NOT info`: web warning 22, error 22 and `unknown` 44, cron error 3, and the Postgres's two `FATAL` lines; no info line. `LEVEL IS info`: web 26, cron 3 and the Postgres's 7 `LOG` lines only | holds; the Postgres's lines follow the negation (dev-3 shipped none) |
| Tail `type=request`, 60 s, six requests sent during it | the empty document streamed all 6, `STATUS_CODE IS_NOT 2xx` the 4 404s, and with `PATH IS_NOT /healthz` the 2 to `/missing-m523` | holds |
| Application Logs at 1440, edited in Edit view | the + button added an Environment row with its operator fixed at `is`. Field → Level read `is one of`, the operator offered `is one of` and `is not one of`, and Done stored `LEVEL IS_NOT [info]`. Before the edit the page read web and cron info lines and 10 Postgres `LOG` lines. After it, the page read web warning, error, level-less and unstructured lines, cron errors and the Postgres's two `FATAL` lines, with no info line. Live, 75 s: 50 new lines, none info. Edit view reopened as `Level` `is not one of` `info` | holds |
| Request Logs at 1440, edited | Status code `is not one of` `2xx` and Request path `is not one of` `/healthz`, both typed, stored `STATUS_CODE IS_NOT [2xx]` and `PATH IS_NOT [/healthz]`. History went from 15 lines (5 each) to the 5 to `/missing-m523`. Live, with 8 requests sent, showed only the 4 to `/missing-m523` and `/other-m523`. Reopened, both rows read `is not one of`, in a 672 px modal with no overflow | holds |
| Both views at 390, set back to empty over GraphQL and edited again in the modal | the same rows saved the same documents, in a 358 px modal whose content is 356 px, on a 390 px page. Application Logs: history 467 lines and Live (75 s) 57 new lines, with no info line in either. Request Logs: history the 9 lines to `/missing-m523` and `/other-m523`. Live, with 8 requests sent, showed the 4 to those two paths. Both reopened as `is not one of` | holds |
| Pending and ready, both views at 1440 and 390 | method: the `SavedSearches` read was delayed 7 s and the view opened client-side from Overview (at 390 through Toggle Sidebar). 1440: header 72–124, actions 92–124, toolbar 148–184, note line 200–216 and panel 232–868 in both states. 390: header 64–160, actions 128–160, toolbar 184–272 (search 184–220, range 234–266, Live and the buttons 228–272), note line 288–320 and panel 336–812 in both states. The ready Application Logs panel draws its "Only part of this time range is loaded" notice inside its top, because the read was a partial page; the panel's bounds do not move | holds |

**Cleanup.** Both views were set back to `{"filters": [], "version": 1}` before the deletes. The web service, the cron job and the Postgres were deleted (`DELETE` 204 each), then the project (`DELETE /v1/projects/…` 204). By-id `GET` answered 404 for all three, the project and the environment, and `savedSearch` answered `NOT_FOUND` for the project's three views. The canary namespace kept no App, Deployment, pod, CNPG cluster or PVC of them; the web pod ran out its 90 s grace, since busybox's `sh` ignores SIGTERM. The QA session was logged out (`qa-login.sh --logout`; the page's cookie then answered 401), and the session jar, the screenshots and the fetched kubeconfig were deleted.

## Closeout (t010, 2026-11-20)

1. **The note's check:** `TestProjectViewReadsEveryLineButInfo` (GraphQL and the tail, Key Value included). On production, `LEVEL IS_NOT info` read the web service's, the cron job's and the Postgres's non-info lines, level-less lines included, in history and on the tail (§ Production).
2. **Every negatable field:** the t009 tests. On production, `STATUS_CODE IS_NOT 2xx` (a class) and `PATH IS_NOT /healthz` read the complement of `IS` on GraphQL and the tail; `INSTANCE IS_NOT` on dev-3.
3. **The rest stays as Render has it:** the refusal tests; `TEXT IS_NOT` refused by name on dev-3; the production editor fixes Environment's operator at `is`.
4. **The editor:** `view-filters.test.ts` and `edit-view-dialog.test.tsx`; on dev-3 and on production at 1440 and 390 the six-field rows offer `is not one of`, save `IS_NOT` and reopen as `is not one of`.
5. **Recorded:** ADR010 § Project views lists the operators per field and no line-field refusal; ADR018's Projects row and the capture name Render's operators per field.
6. **Live:** dev-3 (2026-11-19) and production pin `12716d1d7097` at 1440 and 390 (§ Production).

## Render parity (t007, 2026-11-19)

Compared against Render's dashboard bundle (§ Render's Edit view below, and the [capture](../../../../docs/render-artifacts/project-observability.md#operators-per-field-read-from-the-bundle-2026-11-19-w3m523)). Render's public API has no saved-search or project-log operation and no operator on its log filters.

| Surface | Render | bex | Verdict |
| --- | --- | --- | --- |
| GraphQL `logs(project, filterDocument)` | its `logs(query: {project, filterDocument})` reads the view's document | `IS_NOT` on Level, Instance, Method, Status Code, Host and Path reads the complement of `IS`, a line without the label included; `IS_NOT` on Environment, Text Search or Time Range is refused by name | matches |
| `/v1/logs/subscribe?project=` (a Bex extension) | the view's live tail follows the same document | the same negation, per producer (pods, datastore pods, the request poll) | matches |
| `createSavedSearch` / `updateSavedSearch` | the modal saves `IS_NOT` on the eight negatable fields | stored after the shared validator; refused on the other three | matches |
| Edit view modal: operators | `is one of` / `is not one of` on Resource, Resource Type and the six line fields; `is` on Environment and Time Range; `contains` on Text Search | the same, but Text Search's operator reads `is` | drift: Text Search's `contains` label and `TEXT_SEARCH`/`CONTAINS` spelling, filed by t001 as [w3/083](../083.md) |
| Edit view modal: values | Level, Instance, Method, Status Code, Host and Path list the project's values (`logValues(label, logQuery: {project})`) | Instance, Status Code, Host and Path typed; Level and Method fixed lists | drift: Instance is [w3/079](../079.md); the other five filed as [w3/084](../084.md) |
| Saved document shape | `timeRange.relative` and `display` beside `filters` | a `TIME_RANGE` row, no `display` | drift: [w3/083](../083.md) |
| REST `GET /v1/logs`, MCP `list_logs` | no operator on log filters | unchanged | matches |

## Simplify (t008, 2026-11-19)

`/simplify` over the milestone's diff. Applied: the tests' per-field switch became one `withFilter` helper, and a corpus index lookup became a resource-to-pod table. `translateInstanceFilter` lost a negated-set branch that a mutation run showed changes nothing (an unresolved negated set keeps `noSuchInstance`, which excludes nothing). Kept: `negated` as one unexported label set on `LogQuery` rather than six parallel slices, so routing, validation and counting stay the selecting form's and only matching inverts.

## Test coverage (t009, 2026-11-19)

`lego/backend/internal/logs/negated_filters_test.go`, with the fake Loki (`loki_streams_test.go`) now evaluating every LogQL matcher (`=`, `!=`, `=~`, `!~`, a missing label read as "") and the `json` stage's path and host filters:

- DoD 1: `TestProjectViewReadsEveryLineButInfo` (GraphQL and the project tail, over a web service on two instances, a cron job, a Postgres and a Key Value, with info, warning, `unknown` and level-less lines).
- DoD 2: `TestProjectViewNegatesAnInstanceInItsOwnResourceOnly` (history and tail; an instance that resolves nowhere excludes nothing), `TestProjectRequestViewNegatesEveryRequestField` (Method, Status Code with a class, Host wildcard, Path literal and regex, a value that matches nothing, Level, on GraphQL and the tail), `TestNegatedFiltersReadTheComplementOnThePinnedLoki` (the pinned Loki 3.6.7: each field's `IS` and `IS_NOT` partition the whole read, a stream without the label and an access record without the field kept), `TestPodLogPathsInvertNegatedFilters` and `TestPreDeployReadInvertsANegatedInstance` (pod-log, datastore and pre-deploy reads), `TestNegatedRequestFiltersRouteAsTheirSelectingForm`, `TestNegatedFiltersRenderTheComplementMatcher`.
- DoD 3: the refusals in `TestParseRefusesByName`, `TestProjectReadRefusals` and `TestViewWritesRefuseByName`.
- DoD 4: `view-filters.test.ts` and `edit-view-dialog.test.tsx` (a stored `IS_NOT` row reopens as `is not one of`, the operator offers both, Done saves `IS_NOT`).
- Mutation runs, each restored after: the selector rendering `=`/`=~` for a negated set failed five tests (the pinned-Loki one included); `keep` ignoring a negated level failed three; `keepPod` ignoring a negated instance failed two; the pre-deploy read emptying on an unresolved negated instance failed its test; `viewFieldOperators` offering `IS_NOT` on Resource and Resource Type alone failed both dashboard tests.

## Render's Edit view, read from its dashboard bundle (2026-11-19)

The note asked for Render's operator set per field before shipping. The triage read it from Render's public dashboard bundle. No sign-in is needed: `https://dashboard.render.com/` loads `assets/index-3HFiS5T1.js`, and following its `./*.js` imports to a fixpoint fetches about 457 chunks. The hashes change with every Render deploy, so find the chunks by name.

- **`ProjectLogs-DdJvXDu8.js`** builds the view's fields. It sets `R=[IS, IS_NOT]` and gives `operators:R` to Resource, Resource Type, Level, Instance (both panes), Method, Status Code, Host and Path (Request Logs only). Text Search takes `operators:[CONTAINS]` with a free-text value. Environment takes no `operators`, which defaults to `[IS]` with one value. Time Range takes one preset.
- **`SavedViewFiltersModal-C9MBDKkv.js`** draws the rows. A many-value field labels its operators `is one of` / `is not one of`, and a one-value field `is` / `is not` / `contains`. It saves `{version: 1, filters: [...], timeRange: {relative}, display: {...}}`. The time range is not a filter row: it is stored as `timeRange.relative`, and `display` holds the metrics view's `aggregationMethod`, `requestGroupBy`, `groupBy` and `rootTasksOnly`.
- **`git-DtnyV415.js`** (the generated GraphQL enums) has the saved-search filter field enum `ENVIRONMENT, HOST, INSTANCE, LEVEL, METHOD, PATH, RESOURCE, RESOURCE_TYPE, RUN_ID, STATUS, STATUS_CODE, TASK, TASK_TYPE, TEXT_SEARCH, VERSION`. Its operator enum is `CONTAINS, IS, IS_NOT`. The log query's own filter operators are `INCLUDES, EXCLUDES`.

So the line fields Render negates are six, not the note's seven: Text Search is `contains` only. The bundle also shows that bex's document diverges from Render's in three places: `TEXT` where Render has `TEXT_SEARCH` with `CONTAINS`, a `TIME_RANGE` filter row where Render has top-level `timeRange.relative`, and no `display` block. Those change what a stored document looks like, which is a migration of stored views, not a negation. t001 files them as their own w3 item.

## Gate (2026-11-19)

**Clears by itself; no decision needed.** t006's production half needs a production deploy carrying `9f497080c` (`feat(logs): a project log view reads … (w3/m523 t001-t005, t007-t009)`): bex-api for the reads and the tail, the dashboard for the editor. Check: the newest `chore(deploy): pin platform images to <sha>` on `main` passes `git merge-base --is-ancestor 9f497080c <sha>`, and production's `bex-api` and `dashboard` Deployments run the digests that pin names.

**Next (t006, then t010):** sign in as the QA user (`bash scripts/qa-login.sh`). In `bex-canary`, use or create a project with a web service that logs at more than one level (info, warn, an unstructured line) and serves a 404, a cron job and a Postgres. At 1440 and 390: edit its Application Logs view to `Level is not one of info` and check that history and Live tail show no info line (warnings, errors and level-less lines stay); edit its Request Logs view to `Status Code is not one of 2xx` and `Path is not one of /healthz` and check that only the other requests show, in history and live; reopen Edit view and check both rows read `is not one of`. Compare pending and ready at both widths. Set the views back to their filters before the walk, record the evidence here, delete any fixture with by-id `GET` 404 evidence, log out, and close the milestone (t010).

## Unblocked (2026-11-20)

- **Affected:** w3/m523 t006 (production walk), then t010.
- **Source:** verified evidence; the gate needed no decision. `/loopx w3` dispatched the item from `blocked.ready`.
- **Finding:** the newest pin on `main`, `4e37ec03f` (`chore(deploy): pin platform images to 12716d1d7097`, 08:47Z), carries `9f497080c`: `git merge-base --is-ancestor 9f497080c 12716d1d7097` succeeds. Production's `bex-system/bex-api` runs `bex-operator@sha256:0910030a…5709677` and `dashboard/dashboard` runs `bex-dashboard@sha256:852fe33f…faa8a50`, the digests that pin names, both rolled out.
- **Remaining gates:** none.
- **Next:** t006's production walk as § Gate writes it, then t010.

## Source + Goal linkage

- **Source:** inbox note [w3/076](../../done/076.md), filed from w3/m505 on 2026-11-19 (`IS_NOT` on a line field refused, its Render parity row). Promoted by `/loopx w3` on 2026-11-19.
- **Triage evidence (2026-11-19, `a767dc133`):**
  - **The refusal.** `Filter.validate` refuses `IS_NOT` on every field but `RESOURCE` and `RESOURCE_TYPE` (`lego/backend/internal/savedsearch/document.go:180`). `newProjectView` copies a line field's values into `LogQuery` without the operator (`lego/backend/internal/logs/project.go:110`).
  - **No negation in the log query.** `LogQuery`'s structured filters are value sets that only select (`lego/backend/internal/logs/service.go:309`). They are read in six places, and each needs the negated form: the Loki stream selector (`loki.go:499`–`511`: level, instance, method, status code with its classes), the `json` stage for path and host (`loki.go:589`), the text prefilter (`textprefilter.go:71`), the pod-line level matcher and `keep` (`service.go:1258`, `:874`), the pod selection `keepPod` with `translateInstanceFilter` (`service.go:734`, `:2431`), and the live-tail routing `liveSubscribable` (`service.go:536`).
  - **The editor.** `viewFieldOperators` offers `IS_NOT` only on Resource and Resource Type (`dashboard/src/features/logs/lib/view-filters.ts:141`).
  - Render's operators per field: § Render's Edit view above.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)), project observability ([ADR010 § Project views](../../../../docs/ADR010-observability.md#project-views-one-stream-for-a-whole-project-w3m505-2026-11-19)). It finishes the filter set of [w3/m505](../../done/m505/README.md) and the editor of [w3/m507](../../done/m507/README.md).
- **Expected outcome:** a team keeps an "everything but info" or "not the health checks" (`PATH is not one of /healthz`) view and reads it like any other. A view written on Render with these operators reads on bex.
- **Why now:** m507 shipped the editor, so a user can now see the six fields Render offers `is not one of` on and cannot pick it. The stores and the tail each read the same `LogQuery`, so the six paths change together.
- **Render parity included:** the change reaches GraphQL `logs(project, filterDocument)`, the project tail and the dashboard. REST and MCP keep their filters, which have no operator, as Render's public API does.
