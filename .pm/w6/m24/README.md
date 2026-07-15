# w6 · m24 — Env groups: workspace attribution — fix the cross-tenant read (+ Render object shape)

**Worker:** worker6 **Goal:** Env groups become workspace-owned resources: a caller sees and reveals only their own workspace's groups (closing a cross-tenant read hole), and the group object carries Render's `ownerId`/timestamps. **Status:** in progress — t001–t010 done 2026-07-15, t011 (added round 7, environments linkage) outstanding

## Tasks (in order)

| id   | title                                                                              | est | depends_on                   | status     |
| ---- | ----------------------------------------------------------------------------------- | --- | ----------------------------- | ---------- |
| t001 | Reproduce the cross-tenant list + reveal (severity record)                        | 45m | —                             | — **DONE** |
| t002 | Attribute groups to a workspace (label/store + migration)                         | 60m | t001                          | — **DONE** |
| t003 | Scope list/get/reveal to the caller's workspace                                   | 45m | t002                          | — **DONE** |
| t004 | Scope link/unlink (foreign-group-into-your-service hole)                          | 45m | t002                          | — **DONE** |
| t005 | `ownerId` + timestamps on the group object (Render shape)                         | 40m | t002                          | — **DONE** |
| t006 | Dashboard `/env-groups` follows the workspace switcher                            | 40m | t003, t005                    | — **DONE** |
| t011 | Environments linkage: `envGroupIds` + `set_environment_env_groups` (added round 7) | 45m | t002                          | todo       |
| t007 | Render parity                                                                      | 30m | t003, t004, t005, t006, t011  | — **DONE** (predates t011's addition — see below) |
| t008 | Simplify                                                                           | 30m | t007                          | — **DONE** |
| t009 | Test coverage                                                                      | 45m | t007                          | — **DONE** |
| t010 | Closeout                                                                           | 15m | t009                          | — **DONE** |

## Definition of done

On the mock cluster, a caller in workspace A cannot list, get, or reveal the values of workspace B's env groups (or link B's group into A's service) — each returns 403/absent, proven by a test mirroring `TestReadSideOwnerIDTargetingE2E`. Existing global (unattributed) groups are migrated to a workspace deterministically. The group object exposes `ownerId` and timestamps per Render's OpenAPI, and the dashboard `/env-groups` page shows the switcher-selected workspace's groups.

**Met by t001–t010, verified live 2026-07-15** against a real Postgres + OpenFGA (`TestEnvGroupReadSideOwnerIDTargetingE2E`, `lego/backend/internal/api/envgroups_ownerid_e2e_test.go`) plus a real-browser click-through of the dashboard switcher wiring (local dev stub). t011 was added to this milestone (round 7) after t001–t010's implementation session was already underway with the original 10-task scope; it's genuinely new work (an Environments↔env-groups membership surface), not a residual of the cross-tenant fix, so the milestone stays open for it rather than being closed prematurely.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 4, 2026-07-14; code check: `envgroups.Service` gates every verb on the CALLER's own workspace (`service.go:101` `s.Authorize`) while groups live unattributed in one shared namespace (`ListEnvGroups` `service.go:100` filters nothing; reveal at `service.go:259` is equally caller-scoped) — the same cross-tenant read-bug class w6/m18 fixed in `ListAPIKeys`. w5/m26's own parity note recorded "tenant attribution/migration remains explicit follow-up work" (also flagged by `w7/m12/t001`).
- **Goal linkage:** w6's multi-tenant-workspace charter + w4/w7 security-hardening — one workspace reading another's env-group secret values is a credential-isolation hole, not a cosmetic ◐.
- **Expected outcome:** env groups are workspace-isolated like every other resource; the ADR018 env-groups ◐ (REST shape + attribution) closes.
- **Why now:** it's an exposure, cheapest to fix before real tenants (the w7/m1–m2 sequencing); w6 is down to one open milestone (m23) and owns exactly this concern. Render parity task included — REST/GraphQL/MCP + UI change.

## Progress note (2026-07-15)

t001–t010 implemented, tested, and closed out in this pass:

- Env groups' KV-stored meta gained `workspace`/`createdAt`/`updatedAt` fields, stamped from `core.WithWorkspace`/`Tenant` at create.
- Every verb now goes through a new `authorizeGroup`/`fetchGroup` pair — bare `Authorize` + `core.Base.AuthorizeLabeled` against the group's OWN workspace (the `AuthorizeApp`-seam idiom applied to a non-CRD/KV resource).
- `LinkService` refuses a group whose workspace doesn't match the target service's, closing the write-side "inject a foreign group's secrets into your own service" hole.
- A pre-attribution group is migrated in place to `core.DefaultTenant` the first time it's read once the control-plane store is live — never silently stranded.
- `ownerId`/`createdAt`/`updatedAt` ship identically on REST/GraphQL/MCP; the dashboard's `/env-groups` list + create follow the switcher, mirroring m18's `useApiKeys`/`useCreateApiKey`.
- `/simplify`'s 4-agent pass found and fixed a real gap (`LinkService`/`UnlinkService` weren't bumping `updatedAt`) and flagged two out-of-scope follow-ups: `gqlStr`'s 7th verbatim copy across feature packages is overdue to graduate into `internal/gqlutil`; the lazy on-read migration would benefit from an explicit audit trail.
- ADR018's env-groups row REST cell flips ◐→✅; `w7/m12/t001`'s stale "no tenant attribution" exclusion reasoning flagged in place (moved to `.pm/w7/done/m12/t001.md` by a concurrent session closing that milestone).
- **Found + fixed a second real cross-tenant bug at `/ship` merge time**, in code neither this session nor its reviewer had seen: a concurrently-landed `w1/m35` blueprint-apply seam (`envgroups.Service.GroupNames`/`ApplyEnvGroup`/`LinkEnvGroup`/`findGroupByName`, wired as `apps.EnvGroupApplier`) matched a bex.yml's `envVarGroups:` entries by NAME with zero workspace scoping — a workspace A deploy naming a group `shared` would find, reuse, and silently overwrite workspace B's same-named group's secret values, and `GroupNames`' pre-flight leaked other workspaces' group names into A's validation. Rebasing this milestone's own fix onto that landed code surfaced the conflict; all four functions now scope through `boundWorkspace` (the same helper `ListEnvGroups` uses), and a dedicated regression test (`TestEnvGroup_BlueprintSeamScopesToActingWorkspace`) locks it in. Not part of the original t001–t010 scope — found only because the two features' merge forced a side-by-side read of both.

**t011 remains open** — a distinct, substantial feature (Environments↔env-groups membership: `envGroupIds` on the environment object + a `set_environment_env_groups` verb across REST/GraphQL/MCP + `environmentId` on env-group create) added to this milestone after the above was already underway. Left for a follow-up session; not implemented here to avoid scope creep into unreviewed new work.
