# w4 · m6.5 — Env vars in the dashboard: Render-style Environment tab wired to the m6 API

**Worker:** worker4 **Goal:** give humans the counterpart of m6's env-vars API — a Render-style **Environment** tab on the service detail page where a tenant lists a service's env vars (keys first), reveals a value on demand ("Show secret"), and adds / updates / deletes variables, all backed by m6's dashboard-shaped GraphQL (`service(id){ envVarKeys{ id key } envVar(key){ value } }`, `setEnvVars` / `setEnvVar` / `deleteEnvVar`). Proven end-to-end against a real app (beancount-cms): set its env vars from the dashboard and confirm the running app picks them up after the rollout. **Status:** done — Environment tab shipped (`dashboard/src/features/services`, nav + route + table/reveal/edit/delete), `dashboard` lint + 323 tests green; the dashboard's exact GraphQL operations verified end-to-end against a live `beancount-cms` by `scripts/dashboard-env-verify.sh` (set → OpenBao → materialized → pods roll → app serves the value → delete).

## Tasks (in order)

| id   | title                                                                                           | est | depends_on   | status     |
| ---- | ----------------------------------------------------------------------------------------------- | --- | ------------ | ---------- |
| t001 | GraphQL operations + codegen for env vars (queries + mutations) in the dashboard Apollo layer   | 30m | — (w4/m6)    | — **DONE** |
| t002 | Environment route + service-nav item (Overview / Environment / Logs / Metrics)                  | 30m | —            | — **DONE** |
| t003 | Env-var table: keys list + per-key "Show secret" value reveal (Render-style)                    | 45m | t001, t002   | — **DONE** |
| t004 | Edit mode: add / update / delete a variable, wired to setEnvVar / deleteEnvVar (+ rollout note) | 45m | t003         | — **DONE** |
| t005 | E2E with beancount-cms: set its env vars from the dashboard, confirm the running app uses them  | 40m | t004         | — **DONE** |
| t006 | Simplify — run `/simplify` over the dashboard code this milestone changed                       | 20m | t005         | — **DONE** |
| t007 | Test coverage — meaningful tests for the env-vars dashboard behavior                            | 30m | t005         | — **DONE** |

**Note (t001):** `dashboard/.env` isn't present here, so `yarn codegen` (which introspects a live bex-api with a session token) couldn't run standalone; the env-var operations were added to `definitions.ts` by hand from the known m6 schema and then **verified against the real backend schema** by `scripts/dashboard-env-verify.sh` (the queries parse + resolve live). A future `yarn codegen` run will regenerate `definitions.ts` authoritatively with no behavior change.

## Definition of done

On the local stack (bex-api with `BEX_OPENBAO_URL` set + the dashboard dev server): the service detail page has an **Environment** tab that lists a service's env-var **keys** (values masked), reveals a value on demand via `envVar(key)`, and lets a user add / update / delete variables through `setEnvVar` / `deleteEnvVar` (bulk replace via `setEnvVars`), with the rollout surfaced to the user. A tuple-less key sees the API's 403 surfaced as an error state; with the store unconfigured the tab shows the 503 "unavailable" state rather than crashing. Verified end-to-end with **beancount-cms**: a value set from the dashboard lands in OpenBao, materializes into `beancount-cms-env`, rolls the pods, and the running app serves/uses it. Dashboard unit tests cover the list/reveal/mutation hooks and the loading/empty/error states; `dashboard`'s test + lint pass.

## Source + Goal linkage

- **Source:** user request 2026-07-07 (`/pm add m6.5 …`), with Render's env page as the UX reference (`dashboard.render.com/web/srv-…/env`); consumes w4/m6's env-vars API + GraphQL (dashboard-shaped nesting captured live from Render).
- **Goal linkage:** pillar 1 (Render-compatible surface — the dashboard mirrors Render's Environment page) and pillar 4 (deploy-from-chat needs a human credentials surface too, not only the API/agent path). It's the human counterpart of m6, exactly as w4/m8 is the human counterpart of the API-keys work.
- **Expected outcome:** a human manages a service's environment variables from the bex dashboard just like Render's env page, and a real app (beancount-cms) picks up the values — closing the loop from API (m6) to UI.
- **Why now:** m6 just shipped the env-vars API and the GraphQL was deliberately shaped to Render's dashboard nesting (`service(id){ envVarKeys }`) — building the UI now, while that shape is fresh and verified, is the cheapest it will ever be; and the dashboard already has the service-detail IA (w5/m5: Overview / Logs / Metrics tabs) to hang an Environment tab on.
