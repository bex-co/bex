# w4 · m651 — The create form's Advanced section sets Health Check Path and Pre-Deploy Command, as Render's does

**Worker:** worker4 **Goal:** a new service's first deploy already runs its pre-deploy command and waits for its health check, both set on the create page, instead of a second deploy after Settings. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Read Render's create form Advanced section | 30m | — |
| [t002](t002.md) | The dashboard's `CreateService` sends `healthCheckPath` and `preDeployCommand` | 40m | t001 |
| [t003](t003.md) | Advanced-section controls with Settings' validation, plan gates and 8 locales | 1h | t002 |
| [t004](t004.md) | Add any further Advanced field Render's form has, or record that it has none | 45m | t003 |
| [t005](t005.md) | dev-4: one create, one deploy, pre-deploy run and health-gated | 30m | t003, t004 |
| [t006](t006.md) | ADR018 records create-time parity | 20m | t005 |
| [t007](t007.md) | Render parity | 25m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 5h05m**.

## Definition of done

On the create page, a web service's Advanced section has Health Check Path, and a paid web service, private service or background worker has Pre-Deploy Command; on Free the pre-deploy field is disabled with the reason Settings gives. On dev-4, a service created with both reads them back on GraphQL `server { healthCheckPath preDeployCommand }`, its first deploy logs the pre-deploy run with `pre_deploy_started`/`pre_deploy_ended` and goes live only after its health check passes, and it has exactly one deploy. Every other Advanced field t001 found on Render's form is on bex's, or its absence there is recorded in ADR018.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w4`, 2026-11-25 (the aspect: creating a service from a repository, as Render's create docs describe), item 2. Two independent research agents (service types; creating a service from Git) found it separately; the Render quotes and the bex code were re-checked by hand the same day. Filed by `/pmx them all to w4` at `origin/main` `c196617a8b`.
- **Evidence (2026-11-25):** [render.com/docs/web-services](https://render.com/docs/web-services): "Under the *Advanced* section, you can set environment variables and secrets, add a persistent disk, set a health check path, and more." [render.com/docs/deploys](https://render.com/docs/deploys): the build, pre-deploy and start commands are specified "as part of creating your service in the Render Dashboard", and pre-deploy "is available for paid web services, private services, and background workers". bex: `CreateAdvanced` (`dashboard/src/features/services/pages/new-service-page.tsx:651-797`) has neither field, and the dashboard's `CreateService` mutation sends neither, though GraphQL `createService` accepts both (`lego/backend/internal/apps/graphql.go:1628`, `:1631`), as do REST and MCP. [w7/m41 t004](../../w7/done/m41/done/t004.md) deferred Build Filters at create for non-static types, and nothing filed it.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), on the first path every new user takes.
- **Expected outcome:** a new service is health-gated and runs its pre-deploy command from its first deploy, with no Settings round trip and no second deploy.
- **Why now:** the backend already accepts both fields, so this is dashboard wiring: the cheapest of the three items with the widest reach. w4/m652 builds on the same Advanced section, so this milestone goes first.
- **Render parity:** included: the dashboard changes, and REST, GraphQL and MCP must keep agreeing.
- **Recorded recommendation, not a user decision:** t004 adds Build Filters or Docker Build Context Directory to the create form only if t001 shows Render's form has them, because Render documents both on Settings only. This follows ADR008's parity rule (match what Render offers).
- **Coordination:** w8/m68 (blocked) owns whether command text is trimmed; t002 handles `preDeployCommand` as the builder handles build and start commands today.
