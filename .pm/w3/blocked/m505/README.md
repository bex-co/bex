# w3 · m505 — A project's Application Logs and Request Logs show every resource in one view, as on Render

**Worker:** worker3 **Goal:** a project gets Render's Observability group: Application Logs and Request Logs views, each a saved search, that read every service, cron job, Postgres and Key Value of the project (or one environment) in one time-ordered stream, each line naming its resource. **Status:** blocked — t001–t007 and t009–t012 done and shipped in `405547eb5`; t008 is verified on dev-3, and its production walk waits for a production pin carrying `405547eb5` (§ Gate)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | ADR010 + ADR032: project views are saved searches; scope resolution, filter document and caps — **DONE** | 45m | — |
| [t002](done/t002.md) | Saved-search store with three default views per project — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | GraphQL `savedSearches(projectId, type)` and `savedSearch(id)` with the project's view permission — **DONE** | 1h | t002 |
| [t004](done/t004.md) | GraphQL `logs` takes a project scope and a filter document — **DONE** | 2h | t003 |
| [t005](done/t005.md) | Live tail for a project scope — **DONE** | 1h | t004 |
| [t006](done/t006.md) | Dashboard: the project Observability group and its log view pages — **DONE** | 2h30m | t004 |
| [t007](done/t007.md) | Request Logs Pro gate and the partial, unavailable and empty states — **DONE** | 45m | t006 |
| [t008](t008.md) | Verify on dev-3 and on production | 45m | t005, t007 |
| [t009](done/t009.md) | ADR018 Projects row, ADR010 and the dashboard guide — **DONE** | 30m | t008 |
| [t010](done/t010.md) | Render parity — **DONE** | 20m | t009 |
| [t011](done/t011.md) | Simplify — **DONE** | 15m | t010 |
| [t012](done/t012.md) | Test coverage — **DONE** | 30m | t010 |
| [t013](t013.md) | Closeout | 10m | t011, t012 |

## Definition of done

- Every project has Render's three default saved searches (Application Logs, Request Logs, Service Metrics), readable through GraphQL `savedSearches`/`savedSearch`.
- On dev-3 and on production, `/project/<prj>/observability/logs/application?savedSearchId=<svs>` shows the project's web services', cron jobs', Postgres' and Key Value's lines interleaved in time, each with its resource name and instance, with an environment filter narrowing it, live tail included.
- Request Logs shows the project's web services' request lines; a Hobby workspace gets the Pro refusal.
- A resource the caller cannot read contributes no line, label or name.
- Pending and ready states match at 1440 and 390.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first. [render.com/docs/projects](https://render.com/docs/projects) § Observability: "You can view logs, metrics, and task runs for all services in a project or environment"; the default views are Application Logs and Request Logs ("Available if you have a Pro workspace or higher"). bex has per-service logs only: the project sidebar has Overview and Settings, and GraphQL `logs` takes one resource.
- **Recommendation, not a user decision:** the capture shows Render's own model, so the tasks follow it: every view is a saved search (`svs-`), defaults included. Leaving out the Region picker (bex hosts one region) is recorded in ADR018 as an omission, not a behavior difference.
- **Expected outcome:** a project's whole stack can be read in one stream, as on Render, instead of one service page at a time.
- **Why now:** Render ships this in every workspace and bex's ledger did not record the gap. The saved-search store is the base w3/m507 (view management) and w3/m508 (Service Metrics) build on. Sequenced beside w5/m503 (slow `/v1/logs` reads): t004 reuses the time-boxed paging so a project query cannot outlast it.
- **Render parity included:** new GraphQL reads and dashboard pages; REST and MCP are unchanged because Render has no public operation for them.

## Verification record

### dev-3 (2026-11-19, backend)

A fresh dev-3 identity's workspace `tea-dbvbid1jg4r3h8hu6e7g` (Hobby) with project `prj-dbvbjbpjg4r4tmnnp3k0` ("observed"): environments production (`evm-dbvbjbpjg4r4tmnnp3kg`) and staging (`evm-dbvbjbpjg4r4tmnnp3l0`), a busybox web service `storefront` (production, a line every 7 s), a cron job `nightly-report` (staging, every minute), a Postgres `orders-db` (production) and a Key Value `session-cache` (staging).

- `savedSearches(projectId)` answered Render's three defaults with `{"filters": [], "version": 1}`; `savedSearch(id: "svs-nope")` answered `NOT_FOUND`.
- `logs(project, type: "app")` answered 50 lines, `storefront` and `nightly-report` interleaved in time, each with its `resource`, and `resources` listed all four with names, types and environments. An `ENVIRONMENT IS staging` document narrowed `resources` to `nightly-report` and `session-cache`.
- The tail (`/v1/logs/subscribe?project=…&type=app`, SSE, 25 s) streamed `storefront` (35), `nightly-report` (9) and `orders-db` (2 CNPG checkpoint lines) with their `resource` labels; with the staging document only `nightly-report`. dev-3's log shipper ships only `type=app` App streams to Loki, so the history read had no datastore lines to return there (a direct `logs(resource: "dpg-…")` read returned none either); the tail reads the datastore's pods and showed them.
- `logs(project, type: "request")` and the request tail answered `PLAN_FEATURE_REQUIRED` on Hobby.
- The strict REST gate first refused `project` on `subscribe-logs` ("unsupported query parameter"); `project` and `filterDocument` are now declared Bex extensions there, and `resource` is optional on that operation only.

- The migration shipped as `0308_saved_searches` (renumbered while shipping, above the 0305–0307 that landed on main meanwhile). dev-3's control-plane database had run it as 0305, so its saved-search objects were dropped and its version set back to 304; `dev-env.sh 3 up` then migrated it to 308 cleanly. The dev-3 fixture project was deleted (every by-id `GET` answers 404).

### dev-3 (2026-11-19, dashboard)

Signed in as that identity on a local dashboard against dev-3:

- `/project/prj-dbvbjbpjg4r4tmnnp3k0/observability/logs/application` replaced its URL with `?savedSearchId=svs-vv6c11roi266vom0do1v` (the project's Application Logs view); the title read "observed / Application Logs" and the header "Observability view" + "Application Logs". The sidebar showed Overview, the Observability group (Application Logs active, Request Logs) and Manage → Settings, with the back link to the project.
- `storefront` and `nightly-report` lines interleaved, each with its own dot color and a `[name]` link to its service page; live lines arrived with their label within 16 s. `r=4h` read as "Last 4 hours", and picking Last 24 hours wrote `r=24h`.
- Request Logs (Hobby) showed "HTTP request logs require the Pro plan" with the billing link, and no tail ran. A Request Logs URL carrying an Application Logs id read "View not found" with a link to the project's Request Logs view.
- The service page's Logs tab still has no resource label.
- Pending (the saved-search read slowed) and ready, side by side at 1440 and 390: header, toolbar, search box, range, Live switch, note line and the panel's bottom edge match; the ready page differs only by the data-dependent "Only part of this time range is loaded" notice, as on the service Logs tab.
- No Postgres or Key Value line showed in the page there: dev-3's history has none (above), so their rendering is covered by `log-line-list-resources.test.tsx`; the production walk covers them.

## Render parity (t010)

Compared 2026-11-19 against the capture ([project-observability.md](../../../../docs/render-artifacts/project-observability.md)), [render.com/docs/projects](https://render.com/docs/projects) § Observability and [render.com/docs/logging](https://render.com/docs/logging). Render has no public REST or MCP operation for saved searches or project logs (the pinned `render-public-api-1.json`), so bex's REST and MCP stay as they are: `GET /v1/logs` and `list_logs` keep taking explicit `resource` values, and `GET /v1/logs?project=` is refused by the strict gate.

| Surface | Render | bex | Verdict |
| --- | --- | --- | --- |
| `savedSearches(projectId, type)` | private GraphQL, fields `id name type filters` | same name, arguments and fields; `SavedSearchType` without `WORKFLOW_RUNS` | matched (Workflows is an ADR018 non-goal) |
| `savedSearch(id)` | same fields | same fields; unknown id `NOT_FOUND` | matched; another workspace's view answers the project read's refusal (`FORBIDDEN`, or `NOT_FOUND` under a named workspace), as `project(id)` does, which Render does not show: unverified |
| Default views | Application Logs, Request Logs, Service Metrics, `{"filters": [], "version": 1}` | the same three, every project, created with it and backfilled | matched |
| Filter document | `{filters, version: 1}`, operators `IS`/`IS_NOT` | same shape; fields named from the editor's labels | shape matched; Render's field enum spellings unverified (ADR018 note); `IS_NOT` on line fields refused, filed as w3/076 |
| Project log query | `logs(query: {project, filterDocument, …})`, server-resolved, 50 a page, `hasMore` + cursors | `logs(project, filterDocument, type, …)`, server-resolved, 50 a page, same envelope | matched in semantics; bex's GraphQL schema is its own, not Render's private one |
| Line labels | `SERVICE`, `DEPLOY`, `INSTANCE`, `LEVEL`, `LOG_TYPE` | `resource`, `instance`, `level`, `type` per line, plus the page's `resources` with names | matched for what the page shows; Render's `DEPLOY` label on app lines is not shown in its view and has no bex counterpart: recorded, not filed |
| Scope | the project's services, cron jobs, Postgres, Key Value; environment via the `Environment` filter | the same; Request Logs reads web services | matched |
| Request Logs plan | Pro and higher (docs) | `PLAN_FEATURE_REQUIRED` on Hobby on GraphQL and the tail; the page shows the upgrade path | matched |
| Live tail | Live tail preset in the view | `/v1/logs/subscribe?project=` (SSE), re-resolved every 30 s | matched in behavior; SSE transport is bex's documented choice (ADR010) |
| Region picker | the workspace's regions | omitted | omission recorded in ADR018 |
| Routes | `/project/{prj}/observability/logs/application\|request?savedSearchId=…&r=1h` | the same | matched |
| Sidebar | Overview, OBSERVABILITY (views, + New View), MANAGE Settings | the same, without + New View | New View, Edit view, Duplicate, Delete are w3/m507 |
| Toolbar and lines | search, time range, Maximize, Options, Region; date separators, level gutter | search, time range, live tail; colored dot + `[resource]`, instance, message; request method and status chips | Maximize, Options, date separators and the level gutter are w3/m509 (every explorer) |
| Resource cap | none seen | 20 resources per query, refused by name above it | bex bound, recorded in ADR018 |

## Test coverage (t012)

| Definition of done | Tests |
| --- | --- |
| Three default saved searches, readable through GraphQL | `store/saved_searches_pg_test.go` (created with the project, Render's names, types and empty document, ids equal `id.Derive`, unique names, no `WORKFLOW_RUNS`, deleted with the project; the migration backfills existing projects and the trigger covers new ones), `projects/savedsearches_test.go` (view permission, `NOT_FOUND`, the GraphQL shape, `WORKFLOW_RUNS` refused by the schema) |
| Every resource interleaved with its name and instance, environment filter, live tail | `logs/project_test.go` (`TestProjectApplicationLogsInterleaveEveryResource`, `TestProjectViewFiltersNarrowTheScope`, `TestProjectTailFollowsTheScopeAsItChanges`, `TestProjectTailOutlivesAResourceThatIsGone`, `TestPageReadsItsResourcesConcurrently`), `savedsearch/document_test.go` (scope and validation), `api/render_openapi_logs_limit_test.go` (the gate admits the project tail); dashboard `use-project-log-scope.test.ts`, `project-log-viewer.test.tsx`, `log-line-list-resources.test.tsx`, `project-log-view-page.test.tsx`, `project-log-search.test.ts`, `project-log-views.test.ts`, `project-sidebar.test.tsx` |
| Request Logs reads web services; Hobby is refused | `TestProjectRequestLogsFollowThePlan`; dashboard `project-log-viewer.test.tsx` (the plan refusal, no tail) |
| An unreadable resource contributes no line, label or name | `TestProjectViewNamesNothingTheCallerCannotRead`, `TestProjectViewDropsAResourceThatIsGone`, `TestProjectReadRefusals`, `TestProjectTailRefusesBeforeStreaming` |
| Pending and ready match at 1440 and 390 | `route-skeleton-manifest.test.ts` (both routes registered with their skeleton), the side-by-side check above |

Each test fails without the change (the feature is new); `TestPageReadsItsResourcesConcurrently` was run against a one-at-a-time read and failed.

## Simplify (t011)

`/simplify` ran three reviews (reuse, quality, efficiency) over the diff. Applied: a project read authorizes its resources eight at a time, and a tail's re-resolution skips the resources it already follows (each follower re-checks its own access); a follower that ends has its context released; the tail ends quietly when its own context does; the resource-only and project tails share one live-producer check (`checkLiveSubscribable`) and one serialized writer (`serializedWriters`); per-resource reads use `errgroup` with a limit; the store reuses `appDisplayLabel` and scans through one helper; the scope rule uses the `v1alpha1` service type constants; the dashboard reuses `blueprintResourceHref` for a resource's page, `STRUCTURED_LABEL_KEYS` and `COMMIT_DELAY_MS`, returns the page's `resources` as read, and keys a view's scope on the view alone. Not taken: a shared state ladder for the service and project explorers and one route options factory for the two view routes (a larger refactor of `log-viewer.tsx` than this milestone's scope), one subscription slot per resource for a project tail (it would refuse any project above the five-per-caller default; ADR010 records the one-slot bound), and label-filtered datastore listing (the workspace-wide listing is the projects feature's existing read).

## Gate (2026-11-19)

**Clears by itself; no decision needed.** t008's production half needs a production deploy carrying `405547eb5`. `deploy.yml` deploys a push only when the production pin is at least 6 hours old, and the newest pin (`19faf35f3 chore(deploy): pin platform images to 4b2cebd0bd03`) landed at 09:13Z, so the next deploy runs after about 15:13Z. Check: the newest `chore(deploy): pin platform images to <sha>` on `main` passes `git merge-base --is-ancestor 405547eb5 <sha>`, and production's `bex-api` runs that digest.

**Next (t008 step 2, then t013):** sign in as the QA user (`bash scripts/qa-login.sh`), and in `bex-canary` use or create a project with a web service, a cron job and a Postgres (one in a second environment). Check `savedSearches(projectId)` returns the three defaults; walk `/project/<prj>/observability/logs/application?savedSearchId=<svs>` and `/request` at 1440 and 390, pending and ready, with and without an `ENVIRONMENT` document (GraphQL `logs(project, filterDocument)`), and the live tail; confirm Postgres lines appear (production Loki ships them, dev-3's did not). Record the evidence here, delete any fixture with by-id `GET` 404 evidence, log out, and close the milestone.
