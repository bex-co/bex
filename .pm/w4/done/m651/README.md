# w4 · m651 — The create form's Advanced section sets Health Check Path and Pre-Deploy Command, as Render's does

**Worker:** worker4 **Goal:** a new service's first deploy already runs its pre-deploy command and waits for its health check, both set on the create page, instead of a second deploy after Settings. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Read Render's create form Advanced section — **DONE** | 30m | — |
| [t002](done/t002.md) | The dashboard's `CreateService` sends `healthCheckPath` and `preDeployCommand` — **DONE** | 40m | t001 |
| [t003](done/t003.md) | Advanced-section controls with Settings' validation, plan gates and 8 locales — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Add any further Advanced field Render's form has, or record that it has none — **DONE** | 45m | t003 |
| [t005](done/t005.md) | dev-4: one create, one deploy, pre-deploy run and health-gated — **DONE** | 30m | t003, t004 |
| [t006](done/t006.md) | ADR018 records create-time parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 25m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007, t008 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t009 |

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

## Record — 2026-11-26

**Triage: work.** `CreateAdvanced` (`new-service-page.tsx:651-797`) had neither field and `mutation CreateService` sent neither, while GraphQL `createService` took both (`graphql.go:1628`, `:1631`) with Render's type and Free gates already enforced (`service.go:3854-3857`, `settings_check.go:71-80`). Render's form has both (t001).

**dev-4 (t005).** bex-api rebuilt from this tree (`dev-env.sh 4 up`), the dashboard from this tree on `:50040`, a disposable verified Kratos identity, and headless Chrome on the create page. Removed afterwards: the service, then the account (`DELETE /v1/users`, the identity answers 404).

- **Free first.** A web service opens on Free: Pre-Deploy Command is disabled, with "Pre-deploy commands are available on paid plans." under it, at 1440 and at 390 px. Choosing Starter enables it.
- **One create.** `https://github.com/bex-co/bex/tree/main/examples/hello-go` (Root Directory `examples/hello-go`, branch `main`), runtime Docker, Starter, Health Check Path `/healthz`, Pre-Deploy Command `echo M651_PREDEPLOY_MARKER`. The page sent one `CreateService` with `healthCheckPath: "/healthz"` and `preDeployCommand: "echo M651_PREDEPLOY_MARKER"`, and landed on the first deploy's page. Service `bexm651-walk` (disposable).
- **Read back.** GraphQL `server { healthCheckPath preDeployCommand }` answered `/healthz` and `echo M651_PREDEPLOY_MARKER`, plan `starter`. `GET /v1/services/{id}/deploys` listed exactly one deploy, `live`.
- **Pre-deploy ran first.** The deploy went `build_in_progress` → `pre_deploy_in_progress` → `update_in_progress` → `live`. Events: `deploy_started` 08:04:11, `build_started`, `build_ended` 08:06:15, `pre_deploy_started` 08:06:20, `pre_deploy_ended` 08:06:30 (`preDeployStatus: succeeded`), `deploy_ended` 08:06:56. The deploy log reads `==> Starting pre-deploy: echo M651_PREDEPLOY_MARKER` (08:06:20), `==> Pre-deploy complete!` (08:06:30), then `==> Deploying...`. The command's own output, `M651_PREDEPLOY_MARKER`, is the pre-deploy pod's log line at 08:06:27 and is in dev-4's Loki. dev-4's local log shipper labels pre-deploy Job pods `type=app`, where production's ships them as `type=build` (`deploy/gitops/base/log-shipper.yaml`), so dev-4's deploy-log read shows the step's narration but not the line itself (build output is absent there for the same reason). That is the dev harness, not this change.
- **Health-gated.** The instance's readiness probe is `health-probe check --path /healthz` (period 5 s). Its first `GET /healthz` came at 08:06:41.05, the pod turned Ready at 08:06:41, and the deploy went live at 08:06:56.94, after it.

**Decisions.** On Free the Pre-Deploy Command shows empty, as Render's gated input does, and the reason is text under the field rather than Render's tooltip. A command typed under a paid plan comes back when one is chosen again, and is never sent on Free. Its help text and the path's are Settings' bex wording, as the item asked, not Render's create copy.
