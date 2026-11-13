# Render sandbox groups: delete and default set

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

render.com/docs/sandboxes-creating (read 2026-11-12) mentions the dashboard only for creating a group ("+ New > Sandbox Group") and placing it in a project environment. Listing groups, deleting them, setting the default and images are CLI and API operations.

## What bex answers where Render is silent

- `sandbox_group_suspended` is never produced: bex has no suspended sandbox groups.
- A repeated `DELETE`, or one for a group whose delete is still finishing, answers `404 SANDBOX_GROUP_NOT_FOUND`: the group leaves every read the moment its delete starts.
- `set-default` on the group that is already the default answers `200` with it.
