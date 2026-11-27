# w4 · m697 — A Blueprint sync keeps the routes and headers its file doesn't name, as Render's does

**Worker:** worker4 **Goal:** a Blueprint sync adds and modifies the static-site redirect/rewrite rules and header rules its file declares and keeps every other rule, as Render documents, instead of replacing the whole list and deleting rules added in the dashboard or over the API. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A declared `routes` or `headers` list merges into the live rules instead of replacing them | 1h30m | — |
| [t002](t002.md) | Blueprint plans report only the rules a file actually adds or changes | 45m | t001 |
| [t003](t003.md) | Tests, a dev-4 sync of a static site with a rule the file doesn't name, and the ADR049/ADR018 record | 45m | t002 |
| [t004](t004.md) | Render parity | 25m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

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
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) and [ADR049](../../../docs/ADR049-render-yaml-parity.md).
- **Expected outcome:** a team can manage some static-site rules in a Blueprint and others in the dashboard without the sync deleting them.
- **Why now:** today the loss is silent: the sync reports the deletion as part of the plan, and nothing warns that a dashboard-added redirect will disappear.
- **Render parity:** included: Blueprint plan and sync are served on REST, GraphQL, MCP and the dashboard.
