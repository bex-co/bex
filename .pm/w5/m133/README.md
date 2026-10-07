# w5 · m133 — A Blueprint apply updates only the services its plan names, inside the deploy's workspace

**Worker:** worker5 **Goal:** a Blueprint deploy, create or sync updates exactly the services its plan names: in the deploy's own workspace, matched by the name the manifest declares. It looks them up without listing the workspace's Apps once per declared service. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The apply resolves each declared service by its manifest name in the deploy's workspace, from the request's snapshot | 1h | — |
| t002 | The Blueprint path's other service lookups resolve the same way | 45m | t001 |
| t003 | The plan resolves a service still only in the shared namespace as the apply does | 45m | t001 |
| t004 | Render parity | 30m | t002, t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 30m | t004 |
| t007 | Closeout | 10m | t006 |

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
