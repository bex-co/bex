# w2 · m359 — Delete a sandbox group and change the workspace default, as on Render

**Worker:** worker2 **Goal:** `render sandbox-groups delete` and `render sandbox-groups default set` work against bex with Render's semantics. Deleting a group terminates its sandboxes and deletes what they stored. The workspace's default group cannot be deleted until another group is made the default. **Status:** done

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | Contract: wire shapes of `DELETE /v1/sandbox-groups/{id}` and `POST …/set-default`, and the 409 codes, from the pinned CLI v2.36 — **DONE** | 30m | — |
| t002 | Set default: one default per workspace, swapped under a lock; REST, GraphQL and MCP — **DONE** | 45m | t001 |
| t003 | Delete: refuse the default group; terminate the group's sandboxes; delete its suspended rootfs images and imported images; stop their storage meters; idempotent — **DONE** | 75m | t002 |
| t004 | Dashboard: offer Delete and Make default wherever Render's dashboard does; bex shows groups only in a create dialog today — **DONE** | 45m | t003 |
| t005 | Production walk: delete a group with a sandbox in it, swap the default, and see the default refused — **DONE** | 30m | t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t007 |
| t009 | Closeout — **DONE** | 10m | t008 |

## Definition of done

- On production, with the unmodified pinned Render CLI v2.36:
  - `render sandbox-groups delete <id>` without `--confirm` changes nothing. With `--confirm` the group leaves `sandbox-groups list`, and its running and suspended sandboxes end and leave `sandboxes list`.
  - `render sandbox-groups delete <default-id> --confirm` is refused with 409 `default_sandbox_group_not_deletable`, and the CLI prints its own "make another group the default first" sentence.
  - `render sandbox-groups default set <id>` makes that group the default. The previous default is no longer marked default, and a create without `--group` lands in the new default.
- After a delete, the group's suspended rootfs images and imported images (w2/m358) are gone from the registry, and their storage meters have stopped.
- `SANDBOX_GROUP_OPERATION_UNSUPPORTED` is gone from the code.
- If Render's dashboard offers delete and make-default, bex's does too, to members allowed to manage groups, and the delete confirmation names what it removes. If Render's dashboard has no such actions, ADR104 records that these are CLI and API operations on both products.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-12, item 3, filed by `/pmx them all for w2`.
- **Render** (CLI v2.36, `render-oss/cli`):
  - `cmd/sandboxgroupsdelete.go`: "Delete a sandbox group. This action is irreversible. Deleting a group terminates its sandboxes and deletes its snapshots and credentials shortly after. Suspended groups cannot be deleted. The workspace's default group cannot be deleted; make another group the default with "render sandbox-groups default set <sandboxGroupId>" first. Without --confirm, show the group that would be deleted without making changes."
  - `cmd/sandboxgroupsdefaultset.go`: "Make a sandbox group the default for its workspace."
  - `pkg/sandboxgroup/repo.go`: delete maps 409 `default_sandbox_group_not_deletable` and 409 `sandbox_group_suspended`. Set-default maps 409 `sandbox_group_suspended` and expects the group back (200). Neither route is in Render's public OpenAPI.
- **bex today:**
  - `lego/backend/internal/sandbox/notoffered.go` refuses both routes with `SANDBOX_GROUP_OPERATION_UNSUPPORTED`. w3/m303 owns only that refusal.
  - w22/m311 shipped groups with one lazily created default per workspace (`lego/backend/internal/store/sandbox_groups.go`, `is_default`).
  - bex has no suspended groups, so `sandbox_group_suspended` has no producer. t001 records that.
- **Goal linkage:** ADR008 Render parity first and pillar 5, ADR104 (groups, D7).
- **Expected outcome:** a workspace can tidy its groups. Render allows 10 per workspace, so without delete a workspace fills up and can never create another.
- **Why now:** Render Sandboxes entered public beta on 2026-11-11, and the CLI bex pins ships both commands. This milestone comes after w2/m358 so a deleted group also deletes its imported images. If m358 has not landed, t003 deletes rootfs images only and records that.
- **Render parity included:** REST, GraphQL, MCP, the CLI and the dashboard. bex's dashboard has only `dashboard/src/features/sandbox-groups/components/new-sandbox-group-dialog.tsx` (opened from the New menu) and no group list.
