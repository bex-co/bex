# Finding — moving a member out of a protected environment skips the environment's admin gate

Live functional `/qa-find-bugs w4`, pass 9d7581, 2026-10-19 UTC, `muse.env`, workspace `bex-canary` (Hobby, a single admin member). Production was pinned to `f3871ba9fd44`, and research is on main `e572e7977`. The probes are in [evidence.json](evidence.json). All fixtures were this run's and are deleted.

- **Severity:** major. An administrator-only boundary has an unguarded side door, and the dashboard gives no sign that a protection is being removed.
- **Repro (as performed, admin caller):**
  1. Create project `qa-20261019-9d7581-proj` with environments `production` and `staging`, and a Free web service in production.
  2. `PATCH /v1/environments/<production> {"protectedStatus":"protected","networkIsolationEnabled":true}` → 200.
  3. **Control:** `POST /v1/services/<id>/suspend` with no `confirm` → `400 PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED`, and the service stays `not_suspended`.
  4. Dashboard: project → staging → Manage resources. The Services tab lists the service as `qa-20261019-9d7581-web Running`, with no mention of production or its protection. Check it → Save → toast `Services for "staging" updated.`.
  5. The service is now in staging, production lists none, and `POST …/suspend` without `confirm` → **202** `suspended` (resumed 202).
  6. With the service back in protected production (suspend again 400), GraphQL `setProjectServices(id:<project>, serviceIds:[])` → ok. The service has no project or environment, and an unconfirmed suspend → **202**.
- **Expected:** removing a member from an ACL-bearing environment, by whatever verb, carries that environment's administrator gate. ADR084 (security round 22) finding 3 records the fix as preserving the boundary "on every call path". The dashboard should also say when a checked resource will leave another, protected environment.
- **Root cause (by code; the role half was not probed live):**
  - `lego/backend/internal/environments/service.go:954-1000` `SetServices` runs `authorizeACLBearingMutation(ctx, e)` (`:970`, defined `:1131-1136`, fresh `can_manage` when `aclBearing`, `:876-880`) on the **target** environment `e` only.
    - `store/environments.go:187-210` then points `environment_id` at `e` for every listed service, wherever it was before.
    - Only departures from `e` count as "leaving" (`:991-996`). A service pulled in from an ACL-bearing source loses that source's isolation label and inbound-IP layer (`applyAppEnvironmentLabels` and `applyAppAllowLists` with `e`'s values) and its protected-status guard, with no check on the source.
  - `environments/service.go:1048-1063` `setResourceMembers` (Postgres, Key Value and env groups through `:316, :352, :384`) has the same target-only check.
  - `projects/service.go:570-604` `SetServices` authorizes only `can_create` on the project (`:571`). The store NULLs `environment_id` for omitted members and for incoming members of another project's environment (`store/projects.go:196, 209-213`), and `ClearServiceEnvironmentLayer` strips the layers (`:587-597`), with no ACL check on the environment left.
  - `envgroups/service.go:653` `MoveEnvGroup` authorizes `can_create` on the group, with no check on the environment the group leaves.
  - Under ADR024, `developer` holds `can_create` and `admin` alone manages protected environments. So by code a developer can do through these verbs what `TestACLBearingMembershipTeardownRequiresCanManage` (`environments/acl_test.go:317-378`) refuses on the direct path.
- **Fix (target behavior):** a placement write that takes a resource **out of** an ACL-bearing environment requires a fresh workspace `can_manage` on that environment's workspace, before any store or projection change, on every writer.
  - That is the same rule `authorizeACLBearingMutation` applies to the environment being edited. It applies whether the write names the source, the destination, the project, or the resource itself.
  - A refusal changes nothing (store rows, labels and IP layers) and answers `403` / `ErrForbidden` with the existing wording. No existence detail beyond what the caller can already read leaks.
  - Adding a resource to an ACL-bearing environment stays `can_create`, as today.
  - The dashboard's Manage resources dialog shows each candidate's current environment, marks protected or isolated sources, and says before Save that checking one moves it out of `<environment>`.
- **Blast radius:** the writers that can end a resource's membership in an environment (t001 confirms the list, and the project datastore setters are unverified):
  1. env `SetServices` (incoming)
  2. env `setResourceMembers` for Postgres, Key Value and env groups (incoming)
  3. project `SetServices` (omitted and incoming)
  4. project `SetDatabases`/`SetKeyValues`, which only change the project label (`projects/service.go:613-660`); whether `SetProjectID` also clears the environment label is unverified
  5. `MoveEnvGroup`
  6. any service-level or datastore-level update that accepts an `environmentId`, not found by grep and listed for t001 to confirm
  - Every one is reachable from REST, GraphQL and MCP through one Core verb (ADR006). The dashboard reaches 1–3 through Manage resources and the project page's "Move selected resources to environment".
- **Adjacent classes:**
  - A target-side refusal keeps today's message.
  - A caller who cannot view the source environment must get the same `403` as for any foreign resource. The resource ids are already validated against the workspace (ADR032, datastore id admission).
  - Deleting an ACL-bearing environment or project keeps its existing fresh check (round 9 #5).
  - A bare (non-ACL) source keeps developer-level moves.
  - The protected confirmation phrase is not added to moves; protection is a role boundary for membership, as with ACL edits.
- **Unverified:**
  - The developer-role path live: bex-canary is Hobby, where the only member is an admin (ADR024), and an API key inherits its owner's role.
  - MCP adapters.
  - Datastore and env-group variants live (only services were moved).
  - Whether project datastore setters clear environment labels.
- **Dedupe:**
  - `docs/ADR084-security-review-round22.md` finding 3 and `docs/ADR063-security-review-round9.md` #5 fixed the direct paths.
  - `w4/done/m187` (project moves clear a foreign environment) introduced the clear without an ACL check.
  - `rg -i 'ACL-bearing|aclBearing|incoming move' .pm` found no open item.
  - No commit to `environments/service.go` or `projects/service.go` since `d4eb27ba2` (2026-10-17) touches authorization.
- **Render:** recalled, not re-checked this run: Render documents protected environments as admin-governed, including moving resources into or out of them. The parity task re-checks the docs.
