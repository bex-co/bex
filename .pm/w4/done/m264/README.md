# w4 · m264 — Projects and environments follow Render's lifecycle: a new project comes with its first environment, and deleting an environment or project deletes its services

**Worker:** worker4 **Goal:** creating a project in the dashboard asks for its first environment and creates it, as Render does. Deleting an environment or a project deletes the services inside it after a confirmation that names them, instead of silently leaving them running and unassigned. **Status:** done 2026-11-04 (shipped: first environment on create, service cascade on environment/project delete behind a typed-phrase confirmation, ADR032 updated; the project confirmation shows a service count, not names; live replay follows the release)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | New Project creates its first environment on the dashboard, GraphQL and MCP | 45m | — |
| [t002](t002.md) | Deleting an environment or project deletes its services, behind a confirmation that lists them | 70m | — |
| [t003](t003.md) | Record the lifecycle in ADR032 and ADR018 and retire the "explicit differences" | 15m | w4/m264/t001, w4/m264/t002 |
| [t004](t004.md) | Render parity | 15m | w4/m264/t003 |
| [t005](t005.md) | Simplify | 10m | w4/m264/t004 |
| [t006](t006.md) | Test coverage | 40m | w4/m264/t004 |
| [t007](t007.md) | Closeout | 15m | w4/m264/t005, w4/m264/t006 |

Seven tasks, 210m (3h30m). Two parity findings. This milestone schedules the work only.

## Findings

### 1. A new project has no environment

- **Repro (live, 2026-11-04 10:30 UTC, pin `fb6098dd2e37`, `bex-canary`):** on `/`, choose Create new project. The dialog has one field, Name (aria: `dialog "New Project"` → `textbox "Name"`, `button "Create Project"`). Create `qa-20261104-proj`. The page opens on _"No environments yet. Create one (e.g. staging or production) to group this project's resources."_ `GET /v1/projects/prj-dblgikqobfes738b3ejg` → `environmentIds: []` until an environment is created separately.
- **Render:** [render.com/docs/projects](https://render.com/docs/projects), fetched 2026-11-04: "Provide a name for your new project, along with a name for its first environment." Render's create-project API requires `environments` (`render-public-api-1.json`, `POST /projects` required `['name','ownerId','environments']`).
- **Root cause:**
  - `dashboard/src/features/projects/components/new-project-dialog.tsx:31-35` ("so this dialog just collects a name") and `:61` `create(name.trim())`.
  - GraphQL `createProject` takes only `name`/`ownerId` (`lego/backend/internal/projects/graphql.go:70-78`) and calls `s.Create`.
  - MCP `create_project` calls `s.Create` (`projects/mcp.go:94-99`).
  - REST already has the transactional `CreateWithEnvironments` (`projects/service.go:414`, `rest.go:166-177`, w4/041), so only wiring is missing. REST still accepts an omitted or empty array as a compatibility shim (ADR018 line 35).
- **Downstream cost:** w1/086 exists because a project can sit with no environment and the page opens on an empty one.

### 2. Deleting an environment or project keeps its services running, unassigned

- **Repro (same run):**
  - Project `qa-20261104-proj` → environment Production → More actions → Delete. The dialog reads: _"Its services, databases, and key-value stores stay in the project and keep running — they just lose this environment label."_ (Cancelled.)
  - `/project/<id>/settings` → Delete Project reads: _"Its environments and their configuration are deleted. Its services, databases, and key value stores become unassigned, not deleted."_ This was confirmed on an empty project; `GET` on the project and the environment both returned 404.
- **Render:** [render.com/docs/projects](https://render.com/docs/projects): "Deleting a project deletes _all_ of its associated environments and services." "Deleting an environment deletes all of its associated services." Render's REST API instead refuses non-empty deletes with 409, which bex REST already matches (w8/188).
- **Root cause:** ADR032 records the dashboard, GraphQL (`deleteEnvironment`, `deleteProject`) and MCP preserve-and-detach teardown (`docs/ADR032-environments.md:27`, `:35`). w8/188 (done) left it with "the bex dashboard may keep its own confirmed flow", and w8/188 itself notes ADR032 called these "explicit differences … with no user sign-off". There is no ADR018 entry signing the divergence off, so under ADR008 § Render parity first it does not stand.
- **Blast radius:**
  - Dashboard environment delete (`environments-panel` More actions).
  - Dashboard project delete (`dashboard/src/routes/project.$projectId.settings.tsx`, `features/projects/hooks/use-delete-project.ts`).
  - GraphQL `deleteEnvironment` and `deleteProject` (`projects/graphql.go:81-92`).
  - MCP delete tools.
  - REST stays at 409 for non-empty.
  - The envgroups `DetachEnvironment` path (ADR032:41) and the datastore conditional clears (w4/m214) are the current teardown, and t002 must decide what happens to them.
- **Adjacent classes and the decision t002 records:**
  - Render's sentence names "services". Whether Render's dashboard also deletes the environment's Postgres and Key Value instances and its environment groups is **unverified**.
  - The parity-sufficient choice: delete services as Render documents, and keep datastores and groups detached (today's behavior) until verified. File any wider cascade as a follow-up.
  - A protected environment keeps its Admin gate (ADR032:73).
  - A delete that partially fails must leave the environment row for retry, as today's cleanup does.

### Unverified (both findings)

- Render dashboard's exact confirmation wording and whether it lists the resources.
- The datastore and env-group fate above.
- The GraphQL and MCP create paths were read from source, not called live.

### Dedupe

- `grep -ril "first environment\|lose this environment label" .pm` returns no open item. w1/086, w4/266 and w4/231 are about the project page with or without environments, not about create or delete semantics.
- w8/188 (done) fixed only REST and explicitly left the dashboard flow.
- `git log -S"this dialog just collects a name"` → `1d00b77d2` (w1/m31) and `39ad1b170` (w5/m25), so nothing is pending deploy.
- No `.pm/DO_NOT_DO.md` entry applies.

## Definition of done

Live on production after the release pins this build, with owned `qa-` fixtures in `bex-canary`:

- **Create.** `/` → Create new project shows Name and an environment name field defaulting to `Production`. Creating `qa-…-proj` lands on the project with that environment selected, and `GET /v1/projects/<id>` → `environmentIds` has exactly one ID whose `GET /v1/environments/<id>` name is `Production`. GraphQL `createProject(name, ownerId, environments: [{name}])` and MCP `create_project` with an environment name do the same. A failed environment insert leaves no project, as REST already guarantees.
- **Environment delete.** With a Free image service `qa-…-svc` in that environment, More actions → Delete names `qa-…-svc` as deleted. After confirming, `GET /v1/services/<id>` → 404 and the environment → 404. Cancel sends no mutation.
- **Project delete.** Same shape from `/project/<id>/settings`: the dialog lists the services across every environment. After confirming, the project, its environments and those services all → 404.
- **Datastores and groups.** Whatever t002 decides is exactly what the dialog text says, and is verified on an owned Free Postgres in the environment.
- **REST unchanged.** `DELETE /v1/environments/<id>` on a non-empty environment still answers 409 `ENVIRONMENT_NOT_EMPTY`.
- **Checks pass.** Backend `go test ./...` (real-DB gate), dashboard `yarn lint` and `yarn test`.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 2, 2026-11-04 10:30–10:50 UTC (`muse.env`, workspace `bex-canary`, pin `fb6098dd2e37`, main `0fcb917c7`). Fixtures `prj-dblgikqobfes738b3ejg`, `evm-dblgj8bkom4c73e8d0ng`, `srv-dblgjqrkom4c73e8d140`, `dpg-dblgotsncp7s738f0u1g`, all deleted in-run.
- **Goal linkage:** [ADR032](../../../docs/ADR032-environments.md) (projects and environments), [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) and [ADR018](../../../docs/ADR018-render-parity.md). An unsigned divergence must be amended.
- **Expected outcome:** a Render user's mental model holds. Every project has an environment to put things in, and deleting a grouping removes what it grouped instead of leaving orphaned running services that keep billing.
- **Why now:** both behaviors shape every project journey (the w1/086 workaround exists because of the first), and the delete semantics are a customer-visible surprise that also leaves billable services behind.
- **Render parity included:** dashboard, GraphQL and MCP change. t004 compares Render's documented lifecycle and records anything left (datastores and groups) in ADR018.
