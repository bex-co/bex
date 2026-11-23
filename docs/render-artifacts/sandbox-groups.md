# Render sandbox groups: delete, default set and the project environment

Captured 2026-11-12 for w2/m359 from the pinned Render CLI v2.36 (`render-oss/cli` at `4fd34231b637`). Neither operation is in Render's public OpenAPI, so the CLI's generated client and commands are the contract. The group create, list, retrieve and update operations are in the OpenAPI and were served by w22/m311. The design bex chose is [ADR104 § D7](../ADR104-sandbox-render-parity.md#d7--groups-plans-limits-and-regions).

## Operations

| CLI | Route | Success | Refusals |
| --- | --- | --- | --- |
| `render sandbox-groups delete <sbg-id> [--confirm]` | `DELETE /v1/sandbox-groups/{sandboxGroupId}` | `204`, no body (`ParseDeleteSandboxGroupResponse`) | `409 default_sandbox_group_not_deletable`, `409 sandbox_group_suspended` (`pkg/sandboxgroup/repo.go`) |
| `render sandbox-groups default set <sbg-id>` | `POST /v1/sandbox-groups/{sandboxGroupId}/set-default`, no body | `200` with the group (`ParseSetDefaultSandboxGroupResponse`) | `409 sandbox_group_suspended` |

## Behavior the CLI states

- `cmd/sandboxgroupsdelete.go`: "Delete a sandbox group. This action is irreversible. Deleting a group terminates its sandboxes and deletes its snapshots and credentials shortly after. Suspended groups cannot be deleted. The workspace's default group cannot be deleted; make another group the default with "render sandbox-groups default set <sandboxGroupId>" first. Without --confirm, show the group that would be deleted without making changes."
- Before calling `DELETE`, the command reads the group (`GET /v1/sandbox-groups/{id}`) and refuses a default group itself with `ErrDefaultGroupNotDeletable`. Without `--confirm` it stops after that read.
- `cmd/sandboxgroupsdefaultset.go`: "Make a sandbox group the default for its workspace."
- `pkg/sandboxgroup/repo.go` maps the 409 codes to: "the workspace's default sandbox group cannot be deleted; …", "the sandbox group is suspended and cannot be deleted", and "the sandbox group is suspended and cannot be made the default".

## The dashboard

render.com/docs/sandboxes-creating (read 2026-11-12 and again 2026-11-14) mentions the dashboard only for creating a group ("+ New > Sandbox Group") and, as its words are, "In the Dashboard, you can optionally place a group in a project environment". Listing groups, deleting them, setting the default and images are CLI and API operations.

## The project environment

The pinned OpenAPI's `create-sandbox-group` body has `ownerId`, `name` and `region` only. Its response, and the retrieve and list shapes, carry a nullable `environmentId` ("The ID of the environment this sandbox group belongs to, if any"). Render's own clients go further. They are the contract here, as [ADR104 § D9](../ADR104-sandbox-render-parity.md#d9--the-sdk-is-part-of-the-contract) makes the SDK:

- **Create:** SDK v1.10.0's `createGroup({environmentId})` and CLI v2.38.0's `sandbox-groups create --environment <id or name>` send `environmentId` in the create body.
- **Move:** both clients put a group in an environment, or move it to another one, with Render's environment-resources routes. The SDK's verb is `setGroupEnvironment(group, environment)`; the CLI's is `sandbox-groups environment set <group> <environment>`, which "relies on the API to move the group out of any environment it was in". Both send `POST /environments/{id}/resources` with `{resourceIds: [group]}`.
- **Clear:** `setGroupEnvironment(group, null)` and `sandbox-groups environment remove <group>` read the group and, when it is in an environment, send `DELETE /environments/{that id}/resources?resourceIds=<group>`.
- **Read back:** both clients read the group again afterwards.

The requests and the statuses each client accepts are recorded in `lego/backend/internal/sandbox/testdata/group-environment-contract.json` (w22/m603). Render's public docs (render.com/docs/sandboxes-creating, read 2026-11-23) still name only the dashboard's placement at create, and the public OpenAPI's `resourceIds` description lists "env group, service, Postgres, or Redis".

Placement is membership only: who can see or use the group does not change (w22/m412, [ADR104 § D7](../ADR104-sandbox-render-parity.md#d7--groups-plans-limits-and-regions)).

|  | bex |
| --- | --- |
| REST create body | the pinned fields, plus an optional `environmentId` (a string or `null`), which SDK v1.10.0 and CLI v2.38.0 send |
| Response | `environmentId` is the environment's public `evm-` id, `null` for a group in none |
| `POST /v1/environments/{id}/resources` with an `sbg-` id | `201` with the environment: the group is in it, moved out of any other |
| `DELETE /v1/environments/{id}/resources?resourceIds=<sbg->` | `204`: the group is in no environment; a group that is not in this one is left where it is, as for every other kind |
| An `sbg-` id that names no group of the workspace, on either route | `404 SANDBOX_GROUP_NOT_FOUND`, "sandbox group not found" (the CLI's own reading), the same for another workspace's group; nothing in the request changes |
| A create in another workspace's environment, or one that does not exist | `404 NOT_FOUND`, one answer for both |
| A protected environment on either side of a create or a move, for a non-Admin | `403 PROTECTED_ENVIRONMENT_ADMIN_REQUIRED` ("Moving a group into or out of a protected environment requires an admin", CLI v2.38.0) |
| The environment object | lists no groups, as Render's does not |
| The environment is deleted | the group stays and reads `environmentId: null` (unknown on Render, not enforced; see below) |

Placement and moves are the only group verbs gated by environment protection. Deleting, editing or reading a group in a protected environment is not restricted: Render documents none of it (unknown, not enforced).

GraphQL `setSandboxGroupEnvironment` and MCP `set_sandbox_group_environment` take the SDK's shape (an environment, or null to clear) and go through the same verb as REST.

### Render's dashboard (bundle read 2026-11-23)

- **The group's own page** shows no environment and offers no move.
- **The project page** lists an environment's groups among its resources. Selected groups move with the bulk **Move** control, into another environment or out to the workspace, through the generic `changeEnvironmentForResources` mutation.
- **Other lists:** the environment's "Move Services" dialog lists the workspace's groups as candidates, and the home page's ungrouped table lists groups that are in no environment.
- **Deleting an environment or project that holds the default group** is refused there: "This … contains the default sandbox group. Make another group the default first." Whether Render's API refuses it too is not known (w22/030).

### bex's dashboard (w22/m603)

- **The project page.** Each environment's card lists its sandbox groups among its resources: type "Sandbox Group", status "Ready", the Default badge, and the group's region. They are counted under "All" but have no type filter of their own, so the card's pills stay the ones its skeleton previews, and a group row has no row menu.
- **Moving and removing.** Selected groups move with the card's bulk **Move**. The environment's **Manage resources** dialog has a Sandbox Groups tab, where a group is checked to put it in the environment and unchecked to take it out. Both write `setSandboxGroupEnvironment`.
- **The group's own page** is unchanged, as Render's.
- **Not yet as on Render (w22/031):** the Overview's ungrouped list does not list groups that are in no environment, and bulk Move has no workspace item that takes a selection out of its environment.

## What bex answers where Render is silent

- `sandbox_group_suspended` is never produced: bex has no suspended sandbox groups.
- A repeated `DELETE`, or one for a group whose delete is still finishing, answers `404 SANDBOX_GROUP_NOT_FOUND`: the group leaves every read the moment its delete starts.
- `set-default` on the group that is already the default answers `200` with it.
