# w4 · m697 — A Blueprint sync keeps the routes and headers its file doesn't name, as Render's does

**Worker:** worker4 **Goal:** a Blueprint sync adds and modifies the static-site redirect/rewrite rules and header rules its file declares and keeps every other rule, as Render documents, instead of replacing the whole list and deleting rules added in the dashboard or over the API. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A declared `routes` or `headers` list merges into the live rules instead of replacing them — **DONE** | 1h30m | — |
| [t002](done/t002.md) | Blueprint plans report only the rules a file actually adds or changes — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Tests, a dev-4 sync of a static site with a rule the file doesn't name, and the ADR049/ADR018 record — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 25m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

Total: **7 tasks, about 4h20m**.

## Definition of done

- Syncing a Blueprint whose static site declares `routes` and/or `headers` adds the declared rules that are new, updates the ones it names, and keeps every live rule it does not name, in their current order. A declared `routes: []` or `headers: []` deletes nothing.
- Re-planning a file that declares a subset of the live rules, unchanged, reports no changes on REST, GraphQL, MCP and the dashboard's Blueprint pages.
- On dev-4, a redirect added through the dashboard to a Blueprint-managed static site survives a sync of a file that declares other rules.
- ADR049 and the ADR018 Blueprint row state the rule with Render's quote.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: static sites' redirects, rewrites and headers), item 1. Found by a research agent, re-checked by hand at `origin/main` `a4ce267dfd` before filing; no open, blocked or done item owns it. Related but separate: w31/008 (env vars a file drops stay `managedBy: blueprint`), w3/m678 (the sync page's UI). w4/m694 and w4/m695 also edit `blueprint_plan.go`; run this after them.
- **Render:** [render.com/docs/blueprint-spec](https://render.com/docs/blueprint-spec) § `headers`: "You can modify existing header rules and add new ones. Render _preserves_ any existing header rules that are not included in the Blueprint file." § `routes`: "You can modify existing routing rules and add new ones. Render _preserves_ any existing routing rules that are not included in the Blueprint file."
- **bex today:** `lego/backend/internal/apps/blueprint_plan.go:319-320` replaces the whole list whenever the field is declared (`dst.Routes = canonicalSlice(want.Routes)`, `dst.Headers = canonicalSlice(want.Headers)`). `SetRoutes` and `SetHeaders` (`lego/backend/internal/apps/service.go`) don't lock a Blueprint-managed static site's rules, so rules added in the dashboard or over the API are deleted on the next sync. Env vars already keep what a file doesn't declare (ADR049:147-150).
- **Recommendation, not a user decision:** Render does not say how a file's rule is matched to an existing one or where a new rule goes. Match routes by `type` plus `source` and headers by `path` plus `name`, and append new rules after the existing ones.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) and [ADR049](../../../../docs/ADR049-render-yaml-parity.md).
- **Expected outcome:** a team can manage some static-site rules in a Blueprint and others in the dashboard without the sync deleting them.
- **Why now:** today the loss is silent: the sync reports the deletion as part of the plan, and nothing warns that a dashboard-added redirect will disappear.
- **Render parity:** included: Blueprint plan and sync are served on REST, GraphQL, MCP and the dashboard.

## Record

- **t001.** `mergeBlueprintRules` (`lego/backend/internal/apps/blueprint_plan.go`) backs both appliers. A declared rule replaces the live rule with its key where that rule stands, or is appended; every other live rule keeps its place, and `routes: []` / `headers: []` change nothing. **Deviation from the recommendation:** a route matches by `source` alone, not `type` plus `source`. The static server answers with the first route whose source matches (`matchRoutes`, `staticserver.go`), so with `type` in the key, a file that turned its redirect into a rewrite appended a rewrite the old redirect shadows forever, while the plan reported a change. A header rule matches by `path` plus its canonical name (`applyHeaders` sets by `http.Header.Set`). The n-th declared rule with a key takes the n-th live one, so a file that repeats a key re-syncs as no change. A merge that would pass the 100-rule edge budget is refused at `services[i].routes` / `services[i].headers` before anything is written; before, the CRD's `MaxItems` would have refused the write mid-sync.
- **t002.** The plan already probes `ApplyBlueprintServiceSpec`, so it compares the merged lists with no change of its own. `TestBlueprintStaticRulesPlanAndSyncAcrossSurfaces` validates through REST multipart, GraphQL and MCP against a site whose rules were added with `SetRoutes`/`AddHeader`. A file naming one of three live routes, unchanged, plans `noop`, and so do empty lists. A new route plans `update` of exactly `[routes]`, and a changed header plans exactly `[headers]`. The sync then leaves `[/a′, /b, /c/*, /d]` and `[X-Frame-Options′, Cache-Control]`. `changedFields` stays field-level, as for `envVars`, so "lists exactly that rule" is the path `routes` and no other.
- **t003, dev-4.** bex-api was rebuilt from this tree (`dev-env.sh 4 up`). A disposable verified Kratos identity got its workspace (disposable). `POST /v1/blueprints/deploy` created the static site `m697-site-5c8a70` (disposable) from `octocat/Spoon-Knife` with one redirect, `/one → /index.html`. The dashboard's own GraphQL mutations (`setStaticRoutes`, `setStaticHeaders`, the ones the Redirects/Rewrites and Headers editors send) added `/two → /styles.css` and the header `/* X-M697-Dashboard: kept`. Requests went through a port-forward of `bex-system/bex-static-server` with the site's `Host`. Validating the original file read `noop` on REST and GraphQL. The second file changed `/one`'s destination to `/README.md`, added `/three → /`, and declared `/index.html X-M697-File: synced`. It validated as one `update` of `[headers, routes]` on both. After its sync, `GET …/routes` read `/one → /README.md`, `/two → /styles.css`, `/three → /` at priorities 0–2. `GET …/headers` read both header rules. The static server answered `/one` 301 to `/README.md`, `/two` 301 to `/styles.css` and `/three` 301 to `/`. `/` carried `X-M697-Dashboard` and `/index.html` carried both headers. The same file re-validated as `noop`, and a file with `routes: []` and `headers: []` validated as `noop`. The service was deleted (`DELETE` 204, then 404), then the account (`DELETE /v1/users`, 202). The Kratos identity read 404 afterwards and both tenant namespaces were gone. ADR049 D5 and the ADR018 Blueprint row (`docs/ADR018/deployment-sources-iac.md`) state the rule with Render's quote.
- **t004.** Render parity holds. REST, GraphQL and MCP validate, plan and sync share one applier, and the cross-surface test pins all three. The dashboard's Blueprint pages render GraphQL's `changedFields` unchanged, so they show the same `noop`/`[routes]` plans. Render's spec (re-read 2026-11-27) states the preserve rule for both lists and says nothing on matching or order, so the matching key and the append position are bex's choices, recorded in ADR049 D5. No drift found.
- **t005.** `/simplify`: the merge computed each live rule's key twice; it now computes it once. Two findings were left as they are. `mergeBlueprintEnv` keeps its own merge, because its duplicate-name rule (last wins, one slot) and its nameless-variable drop differ. The budget check merges once more before the applier does, but the lists are at most 100 rules, so that costs nothing.
- **t006.** Each guarded line was reverted in turn and failed its test. The old replacing appliers failed the route and header keep tests, every case of the cross-surface plan test on all three surfaces, and the Blueprint cases of the two persistence tests. `type` in the route key failed the type-change case. A case-sensitive header name failed the case case. Always taking the first slot failed `TestMergeBlueprintRulesRepeatedKey`. Dropping the budget failed `TestBlueprintRulesRespectTheEdgeBudget` and `TestBlueprintRuleBudgetRefusalIsLocated`. `TestStaticRouteDestinationPersistenceAcrossAdapters` and `TestStaticHeaderWildcardPersistenceAcrossAdapters` now expect the site's own rule kept ahead of the file's.

