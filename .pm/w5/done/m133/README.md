# w5 · m133 — A Blueprint apply updates only the services its plan names, inside the deploy's workspace

**Worker:** worker5 **Goal:** a Blueprint deploy, create or sync updates exactly the services its plan names: in the deploy's own workspace, matched by the name the manifest declares. It looks them up without listing the workspace's Apps once per declared service. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The apply resolves each declared service by its manifest name in the deploy's workspace, from the request's snapshot — **DONE** | 1h | — |
| t002 | The Blueprint path's other service lookups resolve the same way — **DONE** | 45m | t001 |
| t003 | The plan resolves a service still only in the shared namespace as the apply does — **DONE** | 45m | t001 |
| t004 | Render parity — **DONE** | 30m | t002, t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- A stack deploy that names no workspace (MCP `deploy` passes no ownerId; REST `POST /v1/blueprints/deploy` makes it optional) by a member of several workspaces creates and updates services only in the workspace it deploys into. A same-named service in another workspace is untouched. Before this change, the reviewer's probe patched `tea-b`'s `web` while creating `api` in `tea-a`.
- A manifest's `web` updates the service created as `web`, which is the object the plan names, not a service merely displayed as "web". Before this change the plan said "update `tea-a-web`" while the apply patched `tea-a-api`, displayed as "web".
- For a service still only in the shared namespace (mid ADR043 D8), the plan's action and resource id name the object the apply updates. Today the plan says "create" while the apply updates `default/web`. Twins still resolve to the workspace's own copy, and two copies in one namespace are still refused (w6/m125).
- A sync declaring several existing and new services lists the workspace's Apps once before its first write. Each declared service used to cost two or three lists, some cluster-wide, plus a membership query per same-named App elsewhere.
- The backend suite on real Postgres and `make lint` pass.

## Source + Goal linkage

- **Source:** found by w5/132's efficiency and quality reviews (2026-10-07). Both behaviour bugs were reproduced with probe tests against `main` 018254503.
  - `applyCreateWithFields` (`lego/backend/internal/apps/deploy.go`) finds each declared service with `s.GetApp(ctx, core.RelCanCreate, req.Name)`.
  - `core.Base.GetApp` (`lego/backend/internal/core/base.go`) resolves by app id cluster-wide, then by displayed name (`appByDisplayedName`). When the request names no workspace, it resolves across every workspace the caller belongs to.
  - The plan (`newBlueprintActionResolver`) and the ownership check key by the manifest name, `core.AppPublicName`, inside the workspace, so the apply can update a different object than the plan named.
- **Goal linkage:** ADR043's tenant boundary, under which a workspace's deploy touches only that workspace. Also the w6/m125 and round-21 finding 7 rule that the plan an approver reviews describes the object the apply touches.
- **Expected outcome:** a deploy from chat (MCP) or the API by a multi-workspace member can no longer rewrite another workspace's same-named service. An approved plan's resource ids match what changes, and large Blueprints apply without one workspace-wide list per service.
- **Why now:** the cross-workspace write is reachable in production through MCP `deploy`, which never passes an ownerId. That makes it a data-integrity defect, so the drain takes it ahead of the remaining notes.
- **Render parity included:** this changes which service a deploy updates on REST and MCP, and possibly GraphQL. Render's Blueprint sync scopes to the Blueprint's owner workspace.

## Result (2026-10-07)

- **One resolution for the plan and the apply** (t001, t003).
  - `workspaceSnapshot.services` keys the deploy's own workspace's Apps by manifest name (`core.AppPublicName`).
  - Of a name's copies, the one in the workspace's own namespace answers. A service still only in the shared namespace answers from there. Two copies in the answering namespace are refused (w6/m125), whatever the list order.
  - A request with no workspace counts only Apps no workspace owns.
  - The plan's resolver, the ownership check and the apply all read it. The apply gets the matched App fresh before patching, so the shared listing is never written through.
  - `listWorkspaceApps` and `foldWorkspaceApps` are gone. The resources view reads the same resolution.
- **The other lookups on the path** (t002).
  - `fromService` host targets, the host-collision preview and reference validation resolve through the same map. Validation reports a workspace-wide duplicate once, as the workspace's conflict.
  - Env-group links, seeded values and maintenance mode now address the App the apply wrote, through `stackServiceRef`: its public id, or its object name when it has no service-name label.
  - The deferred second pass re-applies to the App the first pass wrote, without re-listing.
  - The three apply wrappers are one function, `applyStackService`.
- **Render parity** (t004). Every surface reaches this path through the shared `apps.Service` flows:
  - REST: create, sync, and deploy, whose `ownerId` is optional.
  - GraphQL: create and sync. The dashboard reaches it through GraphQL.
  - MCP: `deploy` (no ownerId), and `create_blueprint` and `sync_blueprint` (the session's named workspace).

  Because `core.WithWorkspace(ctx, "")` is a no-op, MCP `deploy` acts in the session's named workspace when there is one, else in the identity's default, the same choice create and sync make. Render scopes a Blueprint to its owner workspace and keys its services by name, and bex now does too.
- **Simplify** (t005).
  - **Reuse:** one resolution instead of the ownership fold, the shared twin predicate, the merged apply wrappers, and the shared test helpers.
  - **Quality** found five should-fixes, all applied: an unscoped request reaching another tenant's shared-namespace App, a list-order-dependent duplicate refusal, a workspace conflict reported against each service with domains, the list-count goal untested, and the ownership check's second fold. It also fixed the nits.
  - **Efficiency measured** App lists 38 → 10 on a deploy of 3 existing and 2 new services with links, seeds, references and domains, and 39 → 11 on a sync. Validate went 7 → 2. Ownerless deploys went from 15 member-workspace store lookups to 0.
- **Test coverage** (t006). `blueprint_service_resolution_test.go` has 13 tests:
  - the ownerless cross-workspace deploy;
  - an unscoped deploy beside another tenant's App;
  - a manifest name versus a displayed name;
  - a shared-only service planned and applied as one;
  - the own copy answering whatever the list order;
  - the workspace duplicate refused once;
  - one App list per apply;
  - the shared listing never patched;
  - links and seeds reaching the written App, for an update and a create;
  - maintenance mode;
  - `fromService` resolution;
  - the host preview;
  - a reference matching only a displayed name refused.

  Twenty-four mutants across two rounds each fail a test. The apps suite and `make lint` pass. ADR049 records the rule.
- **Follow-ups:**
  - w5/142 now also covers `validateMaintenanceMode` building the host index for an unchanged service.
  - w5/143: a new service's env groups are linked twice.
  - w5/144: an App without a service-name label still reaches the env and maintenance verbs by name.
