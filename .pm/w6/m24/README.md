# w6 · m24 — Env groups: workspace attribution — fix the cross-tenant read (+ Render object shape)

**Worker:** worker6 **Goal:** Env groups become workspace-owned resources: a caller sees and reveals only their own workspace's groups (closing a cross-tenant read hole), and the group object carries Render's `ownerId`/timestamps. **Status:** todo

## Tasks (in order)

| id   | title                                                       | est | depends_on                   |
| ---- | ----------------------------------------------------------- | --- | ---------------------------- |
| t001 | Reproduce the cross-tenant list + reveal (severity record)  | 45m | —                            |
| t002 | Attribute groups to a workspace (label/store + migration)   | 60m | t001                         |
| t003 | Scope list/get/reveal to the caller's workspace             | 45m | t002                         |
| t004 | Scope link/unlink (foreign-group-into-your-service hole)    | 45m | t002                         |
| t005 | `ownerId` + timestamps on the group object (Render shape)   | 40m | t002                         |
| t006 | Dashboard `/env-groups` follows the workspace switcher       | 40m | t003, t005                   |
| t007 | Render parity                                               | 30m | t003, t004, t005, t006       |
| t008 | Simplify                                                    | 30m | t007                         |
| t009 | Test coverage                                               | 45m | t007                         |
| t010 | Closeout                                                    | 15m | t009                         |

## Definition of done

On the mock cluster, a caller in workspace A cannot list, get, or reveal the values of workspace B's env groups (or link B's group into A's service) — each returns 403/absent, proven by a test mirroring `TestReadSideOwnerIDTargetingE2E`. Existing global (unattributed) groups are migrated to a workspace deterministically. The group object exposes `ownerId` and timestamps per Render's OpenAPI, and the dashboard `/env-groups` page shows the switcher-selected workspace's groups.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 4, 2026-07-14; code check: `envgroups.Service` gates every verb on the CALLER's own workspace (`service.go:101` `s.Authorize`) while groups live unattributed in one shared namespace (`ListEnvGroups` `service.go:100` filters nothing; reveal at `service.go:259` is equally caller-scoped) — the same cross-tenant read-bug class w6/m18 fixed in `ListAPIKeys`. w5/m26's own parity note recorded "tenant attribution/migration remains explicit follow-up work" (also flagged by `w7/m12/t001`).
- **Goal linkage:** w6's multi-tenant-workspace charter + w4/w7 security-hardening — one workspace reading another's env-group secret values is a credential-isolation hole, not a cosmetic ◐.
- **Expected outcome:** env groups are workspace-isolated like every other resource; the ADR018 env-groups ◐ (REST shape + attribution) closes.
- **Why now:** it's an exposure, cheapest to fix before real tenants (the w7/m1–m2 sequencing); w6 is down to one open milestone (m23) and owns exactly this concern. Render parity task included — REST/GraphQL/MCP + UI change.
