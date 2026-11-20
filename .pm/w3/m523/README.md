# w3 · m523 — A project log view reads `is not one of` on Level, Instance, Method, Status Code, Host and Path, as Render's Edit view offers

**Worker:** worker3 **Goal:** a project's Application Logs and Request Logs views take Render's `IS_NOT` on every line field Render's Edit view offers it on (Level, Instance, Method, Status Code, Host, Path). The log query reads the complement on the store, the pod-log and datastore paths and the live tail, the validator accepts the operator, and the Edit view modal offers `is not one of` there. A view such as "everything but info" reads instead of being refused. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's per-field operators and document shape in the capture and ADR010, and file the document-shape divergence | 30m | — |
| [t002](t002.md) | Backend: negated value sets on `LogQuery`, honored by the Loki selector and pipeline | 1h | — |
| [t003](t003.md) | Backend: the pod-log, datastore and live-tail paths honor the negated sets | 1h15m | t002 |
| [t004](t004.md) | Backend: the validator accepts `IS_NOT` on the six fields, the project view routes it, ADR010 and ADR018 drop the refusal | 30m | t001, t003 |
| [t005](t005.md) | Dashboard: the Edit view modal offers `is not one of` on the six fields | 45m | t004 |
| [t006](t006.md) | Verify on dev-3, and on production at 1440 and 390 | 45m | t005 |
| [t007](t007.md) | Render parity | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 6h30m**.

## Definition of done

1. **The note's check.** A project view whose document holds `LEVEL IS_NOT [info]` reads every line of the project's services, cron jobs, Postgres and Key Value whose level is not info, a line with no level included. It does so on GraphQL `logs(project, filterDocument)` and on the `/v1/logs/subscribe?project=` tail, with a test per surface.
2. **Every negatable field.** `IS_NOT` on `INSTANCE`, `METHOD`, `STATUS_CODE` (a `4xx` class too), `HOST` and `PATH` reads the complement of what `IS` reads, on both surfaces. `*` wildcards and `/regex/` values negate the same way. A negated value that matches nothing excludes nothing. An `INSTANCE IS_NOT` that resolves in one resource and not in another narrows only the first.
3. **The rest stays as Render has it.** `ENVIRONMENT` and the time range take `IS` only. Text Search takes Render's `contains` only. `IS_NOT` on any of them is refused by name, as before.
4. **The editor.** The Edit view modal offers `is one of` / `is not one of` on Level, Instance, Method, Status Code, Host and Path, as it already does on Resource and Resource Type. A stored `IS_NOT` row reopens as `is not one of`.
5. **Recorded.** ADR010 § Project views no longer lists the refusal, and the ADR018 Projects row and the [capture](../../../docs/render-artifacts/project-observability.md) name Render's operators per field.
6. **Live.** On dev-3, and on production once a pin carries the milestone, as the QA user in `bex-canary`, at 1440 and 390: a view edited to `Level is not one of info` reads no info line, in history and live.

## Render's Edit view, read from its dashboard bundle (2026-11-19)

The note asked for Render's operator set per field before shipping. The triage read it from Render's public dashboard bundle. No sign-in is needed: `https://dashboard.render.com/` loads `assets/index-3HFiS5T1.js`, and following its `./*.js` imports to a fixpoint fetches about 457 chunks. The hashes change with every Render deploy, so find the chunks by name.

- **`ProjectLogs-DdJvXDu8.js`** builds the view's fields. It sets `R=[IS, IS_NOT]` and gives `operators:R` to Resource, Resource Type, Level, Instance (both panes), Method, Status Code, Host and Path (Request Logs only). Text Search takes `operators:[CONTAINS]` with a free-text value. Environment takes no `operators`, which defaults to `[IS]` with one value. Time Range takes one preset.
- **`SavedViewFiltersModal-C9MBDKkv.js`** draws the rows. A many-value field labels its operators `is one of` / `is not one of`, and a one-value field `is` / `is not` / `contains`. It saves `{version: 1, filters: [...], timeRange: {relative}, display: {...}}`. The time range is not a filter row: it is stored as `timeRange.relative`, and `display` holds the metrics view's `aggregationMethod`, `requestGroupBy`, `groupBy` and `rootTasksOnly`.
- **`git-DtnyV415.js`** (the generated GraphQL enums) has the saved-search filter field enum `ENVIRONMENT, HOST, INSTANCE, LEVEL, METHOD, PATH, RESOURCE, RESOURCE_TYPE, RUN_ID, STATUS, STATUS_CODE, TASK, TASK_TYPE, TEXT_SEARCH, VERSION`. Its operator enum is `CONTAINS, IS, IS_NOT`. The log query's own filter operators are `INCLUDES, EXCLUDES`.

So the line fields Render negates are six, not the note's seven: Text Search is `contains` only. The bundle also shows that bex's document diverges from Render's in three places: `TEXT` where Render has `TEXT_SEARCH` with `CONTAINS`, a `TIME_RANGE` filter row where Render has top-level `timeRange.relative`, and no `display` block. Those change what a stored document looks like, which is a migration of stored views, not a negation. t001 files them as their own w3 item.

## Source + Goal linkage

- **Source:** inbox note [w3/076](../done/076.md), filed from w3/m505 on 2026-11-19 (`IS_NOT` on a line field refused, its Render parity row). Promoted by `/loopx w3` on 2026-11-19.
- **Triage evidence (2026-11-19, `a767dc133`):**
  - **The refusal.** `Filter.validate` refuses `IS_NOT` on every field but `RESOURCE` and `RESOURCE_TYPE` (`lego/backend/internal/savedsearch/document.go:180`). `newProjectView` copies a line field's values into `LogQuery` without the operator (`lego/backend/internal/logs/project.go:110`).
  - **No negation in the log query.** `LogQuery`'s structured filters are value sets that only select (`lego/backend/internal/logs/service.go:309`). They are read in six places, and each needs the negated form: the Loki stream selector (`loki.go:499`–`511`: level, instance, method, status code with its classes), the `json` stage for path and host (`loki.go:589`), the text prefilter (`textprefilter.go:71`), the pod-line level matcher and `keep` (`service.go:1258`, `:874`), the pod selection `keepPod` with `translateInstanceFilter` (`service.go:734`, `:2431`), and the live-tail routing `liveSubscribable` (`service.go:536`).
  - **The editor.** `viewFieldOperators` offers `IS_NOT` only on Resource and Resource Type (`dashboard/src/features/logs/lib/view-filters.ts:141`).
  - Render's operators per field: § Render's Edit view above.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)), project observability ([ADR010 § Project views](../../../docs/ADR010-observability.md#project-views-one-stream-for-a-whole-project-w3m505-2026-11-19)). It finishes the filter set of [w3/m505](../done/m505/README.md) and the editor of [w3/m507](../done/m507/README.md).
- **Expected outcome:** a team keeps an "everything but info" or "not the health checks" (`PATH is not one of /healthz`) view and reads it like any other. A view written on Render with these operators reads on bex.
- **Why now:** m507 shipped the editor, so a user can now see the six fields Render offers `is not one of` on and cannot pick it. The stores and the tail each read the same `LogQuery`, so the six paths change together.
- **Render parity included:** the change reaches GraphQL `logs(project, filterDocument)`, the project tail and the dashboard. REST and MCP keep their filters, which have no operator, as Render's public API does.
