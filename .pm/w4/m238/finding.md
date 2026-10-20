# A moved service keeps its old environment's group applied, and only the group page says so (as "Legacy")

- **Severity:** major. The move completes, but the product misleads: a Production-scoped environment group keeps feeding its values into a service that now sits in Staging. The service's own Environment page lists the link as ordinary. The group page calls the state "Legacy", though the move created it a minute earlier. The API refuses to create this exact link directly.
- **Source:** live `/qa-find-bugs w4` loop, run c7a1, 2026-10-20 UTC (2026-10-19 PDT). Production `https://dashboard.bex.co`, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), `muse.env` credentials via `scripts/qa-login.sh --serve`. Every fixture below was created and deleted this run; the session was revoked.

## Repro (all steps run this pass)

1. Project `qa-20261020-c7a1-proj` (`prj-dbbfu9r581vs73bjhh60`) with environment Production (`evm-dbbfuih9a20c7385r9k0`). Free web service `qa-20261020-c7a1-web` (`srv-dbbfv0r581vs73bjhh80`) from Public Git URL `https://github.com/bex-co/bex`, root `examples/hello-go`, runtime Go, in Production, with service env `MESSAGE=qa-c7a1-v2`.
2. Service → Environment → **Create group** `qa-20261020-c7a1-group` (`evg-dbbg5cj581vs73bjhhig`, scoped to Production), with `MESSAGE=qa-c7a1-from-group`, linked to the service. The service's own value wins as Render documents: the public URL served `qa-c7a1-v2` at 22:59:37 PDT.
3. **New Environment** `Staging` (`evm-dbbg6t99a20c7385ra70`). On the project page, tick the service → **Move selected resources to environment** → `Staging` → **Move**. No dialog and no warning; the move is a single write.
4. Session-authenticated REST from the page (`fetch(…, {credentials:'include'})`):

   ```json
   {
     "service": {
       "id": "srv-dbbfv0r581vs73bjhh80",
       "environmentId": "evm-dbbg6t99a20c7385ra70",
       "projectId": "prj-dbbfu9r581vs73bjhh60"
     },
     "group": {
       "id": "evg-dbbg5cj581vs73bjhhig",
       "environmentId": "evm-dbbfuih9a20c7385r9k0",
       "serviceLinks": [
         {
           "id": "srv-dbbfv0r581vs73bjhh80",
           "name": "qa-20261020-c7a1-web",
           "type": "web"
         }
       ],
       "envVarKeys": ["MESSAGE"]
     }
   }
   ```

   (`GET /v1/services/srv-dbbfv0r581vs73bjhh80` and `GET /v1/env-groups/evg-dbbg5cj581vs73bjhhig`, projected.)

5. Fresh `/services/srv-dbbfv0r581vs73bjhh80/env` (breadcrumb `qa-20261020-c7a1-proj › Staging › qa-20261020-c7a1-web`). Accessibility tree of the Linked Environment Groups card:

   ```yaml
   - heading "Linked (1)" [level=3]
   - list:
       - listitem:
           - link "qa-20261020-c7a1-group"
           - text: Linked MESSAGE
           - paragraph: Struck-through keys are overridden by this service's own environment variables.
           - button "Unlink"
   - heading "Available to link (0)" [level=3]
   - paragraph: Every workspace environment group is already linked.
   ```

   There is no scope notice. Fresh `/env-groups/evg-dbbg5cj581vs73bjhhig` → Linked Services row: `paragraph: Legacy link outside this group's Environment` + `button "Unlink qa-20261020-c7a1-web"`. Screenshots: `.playwright-mcp/qa-envgroup-c7a1-1.png` (group page) and `qa-envgroup-c7a1-2.png` (service page).

6. Proof the values really flow. Service Environment → Edit → **Delete MESSAGE** → **Save and deploy**. `curl -sS https://qa-20261020-c7a1-web.onbex.co/` then returned `qa-c7a1-from-group` at 23:03:16 PDT. So a Staging service runs on the Production group's value.
7. **Control: the API refuses this link.** From the same page:

   ```
   DELETE /v1/env-groups/evg-dbbg5cj581vs73bjhhig/services/srv-dbbfv0r581vs73bjhh80 -> 204
   POST   /v1/env-groups/evg-dbbg5cj581vs73bjhhig/services/srv-dbbfv0r581vs73bjhh80 -> 409
   {"code":"ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH","error":"linked services must have the same Environment scope as the environment group","id":"conflict","message":"linked services must have the same Environment scope as the environment group","params":{"serviceEnvironmentId":"evm-dbbg6t99a20c7385ra70","serviceId":"srv-dbbfv0r581vs73bjhh80","targetEnvironmentId":"evm-dbbfuih9a20c7385r9k0"}}
   ```

## Root cause

- **Backend: the service-side placement writers never look at group links.** `environments.Service.SetServices` (`lego/backend/internal/environments/service.go:957-1035`) moves rows, labels, isolation and the inbound-IP layer. It contains no group or link reference (`grep -n -iE "group|link"` over that range is empty). `projects.Service.SetServices` (`lego/backend/internal/projects/service.go:589`) has zero env-group references in the whole file. The Blueprint re-apply placement path is `apps.Service.applyEnvironmentChange` → `SetAppEnvironment` (`lego/backend/internal/apps/deploy.go:3676-3701`).
- **The guards exist only on the group side.** Link uses `validateGroupServiceEnvironment` (`lego/backend/internal/envgroups/service.go:1278-1290`, `ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH`). Group move uses `validateLinkedServiceEnvironments` (`:1292-1320`, `ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES`). The group-move remedy text is "move the linked services to the target Environment or unlink them before moving this group". So a service moved ahead of its group is a sanctioned intermediate state, not corruption. `w4/m187` t001 likewise preserved group links on project moves on purpose.
- **Dashboard: the state is invisible where the user acts.**
  - The service panel applies `serviceMatchesGroupScope` only to the Available-to-link candidates (`dashboard/src/features/services/components/env-groups-panel.tsx:226-238`, the file's only call). Linked rows are never checked.
  - The group page checks linked rows (`dashboard/src/features/env-groups/components/linked-services-card.tsx:141-153`). Its copy `envGroups.incompatibleLink` = "Legacy link outside this group's Environment" (`dashboard/src/features/env-groups/locales/en.ts:447-449`) dates the state as pre-scoping legacy.
  - None of the move UIs reads group links: inline Move `dashboard/src/features/environments/components/environment-card.tsx:254`, Manage resources Save `manage-resources-dialog.tsx:261` (group-aware only for group membership), and Move to project / Remove from project `dashboard/src/features/projects/hooks/use-move-to-project.ts` (`removeFromProject` `:208`) via `move-to-project-menu.tsx`, mounted in `services/components/service-row-actions.tsx:252`.

## Target behavior

- **Moves stay allowed; the API contract is unchanged.** Refusing service moves would deadlock against `ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES`: neither the group nor its services could move without an unlink. That unlink drops the configuration from the service on its next deploy. The intermediate state is already readable on REST, GraphQL and MCP (`service.environmentId` vs `group.environmentId` + `serviceLinks`).
- **Before the move:** a dashboard move that would leave one or more linked groups scoped to a different environment asks for confirmation first. That covers a move into another environment, into another project, or out of every project. The dialog names each group with its Project / Environment. It states that the group's variables and files keep applying to the service until the group is unlinked or moved. Cancel writes nothing. A service with no such link moves exactly as today, with no dialog.
- **After the move:** on the service Environment page, every linked group whose scope does not match the service shows a notice next to its Unlink. The notice names both environments, for example "Scoped to qa-proj / Production — this service is in Staging". It uses the same exact-equality `serviceMatchesGroupScope` predicate, including workspace/null scope, as the group page. It appears only after the scope index resolves: pending is neutral and a failure keeps the existing error and retry. The group page uses matching wording without "Legacy".
- **Adjacent classes:** a workspace-scoped group (`environmentId` null) linked to a service that moves into an environment is the same mismatch under the exact rule, so it is flagged too. A move that fails (forbidden, protected-environment refusal, `ENVIRONMENT_MEMBERSHIP_PARTIAL`, timeout) keeps its current error handling; the confirmation never stands in for one.

## Blast radius

- Dashboard move writers: 3 components (`environment-card.tsx`, `manage-resources-dialog.tsx`, `move-to-project-menu.tsx` via `use-move-to-project.ts`). Datastore row actions also mount `MoveToProjectMenu` (`databases/components/database-row-actions.tsx:179`, `keyvalue/components/key-value-row-actions.tsx:103`), but Postgres and Key Value never link groups, so they must keep moving without a dialog.
- `serviceMatchesGroupScope` has 3 production calls today (linked-services-card ×2, env-groups-panel ×1). The service-panel notice adds a 4th. The existing callers need regression tests that they are unchanged.
- Backend: none. Keep both 409 guards byte-identical.

## Unverified this run

- Manage resources Save, Move to project and Remove from project with a linked group: the code was read, but these were not exercised live.
- The Blueprint re-apply environment change (no UI) and protected or network-isolated environments.
- Static, cron, worker and private service types; 390px; Chinese copy.
- Render's own behavior when a service with a scoped group changes environment. Public docs ([scoped configuration](https://render.com/docs/projects#scoped-configuration)) say only that a scoped group is usable by services in that environment. The move itself was not captured.
