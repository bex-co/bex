# w22 · m603 — A sandbox group moves between project environments through Render's environment-resources route, as CLI v2.38.0 and SDK v1.10.0 do

**Worker:** worker22 **Goal:** the calls Render's own clients added on 2026-11-20 to put a sandbox group in a project environment, move it to another one or take it out work unchanged against bex: `POST /v1/environments/{id}/resources` and `DELETE /v1/environments/{id}/resources?resourceIds=` accept an `sbg-` id, a protected environment needs an Admin on either side of the move, and REST, GraphQL, MCP and the dashboard agree. **Status:** blocked (t001–t009 done; t010 waits for the production walk)

## Tasks (in order)

| id   | title                                                                                                                                                 | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Freeze the CLI v2.38.0 and SDK v1.10.0 wire contract for a group's environment as fixtures — **DONE** | 40m | —                |
| t002 | The environment-resources routes take a sandbox group: add moves it, remove clears it, an unknown or foreign `sbg-` is 404, all-or-nothing with the rest — **DONE** | 75m | t001             |
| t003 | Moving a group into or out of a protected environment needs an Admin: 403 `PROTECTED_ENVIRONMENT_ADMIN_REQUIRED` — **DONE** | 45m | t002             |
| t004 | GraphQL and MCP reach the same core verb — **DONE** | 60m | t002             |
| t005 | Dashboard: the group page shows its environment and moves or removes it, after reading what Render's dashboard offers — **DONE** | 90m | t004             |
| t006 | Documents: `sandbox-groups.md`, ADR104 D7, ADR042 § Sandbox groups, ADR018 — **DONE** | 25m | t003, t005       |
| t007 | Render parity — **DONE** | 20m | t003, t004, t005 |
| t008 | Simplify — **DONE** | 20m | t007             |
| t009 | Test coverage — **DONE** | 30m | t007             |
| t010 | Closeout                                                                                                                                              | 10m | t006, t008, t009 |

## Definition of done

- On a `dev-N` stack, unmodified Render SDK v1.10.0 (TypeScript, and Python where it runs) calls `setGroupEnvironment(sbg, A)`, then `(sbg, B)`, then `(sbg, null)`, and `getGroup` reads `environmentId` A, then B, then `null`. The same three steps with `curl` on REST give the same reads, and `POST` answers 201 as for every other kind.
- `render sandbox-groups environment set <group> <environment>` and `environment remove <group>` from CLI v2.38.0's source (the pin bump is w22/m604) succeed against the same stack, with the environment given by id and by name.
- A non-Admin moving a group into or out of a protected environment gets 403 `PROTECTED_ENVIRONMENT_ADMIN_REQUIRED`; an Admin succeeds. An unknown `sbg-` id, or one from another workspace, answers 404. A mixed request such as `[srv-<ok>, sbg-<unknown>]` changes no member.
- GraphQL and MCP move and clear a group with the same answers. The dashboard moves a group as Render's dashboard does (amended 2026-11-23 from the bundle read in t005): the project page lists each environment's groups, bulk Move moves them, and Manage resources puts them in or takes them out, while the group's own page shows no environment, as Render's does not. Pending and ready states match at desktop and narrow-mobile widths.
- `docs/render-artifacts/sandbox-groups.md` no longer says a group cannot move or that an official client never sends `environmentId`.
- The backend suite is green against the real database. Once the production pin carries the milestone's commit, the move is walked in `bex-canary` with the QA user on a group the run creates and deletes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w22` (2026-11-23), item 1, filed by `/pmx them all to w22`; verified at `origin/main` `cb469e7cfd`.
  - **Render SDK v1.10.0** (Python and TypeScript, 2026-11-20 03:21 UTC; release notes read 2026-11-23): "`setGroupEnvironment()` puts a sandbox group in a project environment or moves it to another one. Pass `null` as the environment ID to take the group out of its environment." The TypeScript source (`typescript/src/sandboxes/index.ts` at `typescript/v1.10.0`, lines 1266–1316) sends `POST /environments/{environmentId}/resources` with `{resourceIds: [sandboxGroupId]}`; to clear, it reads the group, then sends `DELETE /environments/{environmentId}/resources?resourceIds=<sandboxGroupId>` for the environment the group reads. `createGroup({environmentId})` sends `environmentId` in the `POST /sandbox-groups` body, and "Creating a group in a protected environment requires the Admin role in the workspace."
  - **Render CLI v2.38.0** (`09b7802aa379c92a087884412d88b2b3f8a75a9a`, 2026-11-20 03:27 UTC): `sandbox-groups environment set <sandboxGroupId> <environment>` and `environment remove <sandboxGroupId>` (`cmd/sandboxgroupsenvironmentset.go`, `…remove.go`). Its help says "Moving a group into or out of a protected environment requires an admin." `pkg/sandboxgroup/repo.go` `AddToEnvironment` "relies on the API to move the group out of any environment it was in"; `ClearEnvironment` reads the group and removes it from the environment it names. `create --environment` resolves an id or a name and sends `EnvironmentId` in the create body.
  - **bex today:** `classifyResources` (`lego/backend/internal/environments/resources.go:59-79`) answers 400 "is not a service, Postgres, Key Value or environment group id" for an `sbg-` id, so both clients' move calls fail. The group's placement is set only at create time (w22/m412: migration 0270 `sandbox_groups.environment_id … ON DELETE SET NULL`, `placeGroup` at `lego/backend/internal/sandbox/groups.go:320`). `docs/render-artifacts/sandbox-groups.md:34` says moving "not offered (the update takes only `concurrencyLimit`)", and `:29` says an official client never sends `environmentId` on create, which SDK v1.10.0 and CLI v2.38.0 now do.
  - Render's public docs (render.com/docs/sandboxes-creating, read 2026-11-23) still mention only the dashboard placing a group at create time, and the public OpenAPI's `environmentResourcesPOSTInput.resourceIds` item lists "env group, service, Postgres, or Redis". The clients are the contract here, as ADR104 D9 makes the SDK.
- **Goal linkage:** ADR008 § Render parity first; ADR104 D7 (groups) and D9 ("the SDK is part of the contract"); ADR018's sandbox groups row.
- **Expected outcome:** a user who upgrades to Render's current SDK or CLI and moves a group between environments sees it work on bex, as on Render, instead of a 400.
- **Why now:** w22/m604 (CLI v2.38.0) and w22/m445 (retargeted to SDK v1.10.0) both need this server path for their done criteria, and the scheduled schema-drift job is red on the CLI pin today. The same handler was just changed by w4/m596 (201 on add), so its context is fresh.
- **Recorded, not decided here:** whether Render refuses deleting an environment that still holds a group (its 409 `ENVIRONMENT_NOT_EMPTY` for other members) cannot be checked: Render's environment object does not list groups. bex keeps m412's behavior (the group stays and reads `environmentId: null`) and records it as unknown, not enforced. Whether Render's environment object should list groups is likewise unknown, so the REST environment object keeps Render's shape.
- **Render parity included:** the change touches REST, GraphQL, MCP and the dashboard, so t007 compares every surface with the clients' contract.

## Progress

**Triage (2026-11-23, worker22):** work. At `a52d4422ae`, `classifyResources` (`lego/backend/internal/environments/resources.go`) still answered 400 for an `sbg-` id, so both clients' moves failed; nothing external gates the server, GraphQL, MCP or dashboard work. Only the `bex-canary` walk waits for the production pin.

**t001: the clients' contract** is `lego/backend/internal/sandbox/testdata/group-environment-contract.json`, read on 2026-11-23 from SDK `typescript/v1.10.0` and `python/v1.10.0` (both `0529a77a19fd`) and CLI `v2.38.0` (`09b7802aa379`). Python sends the same requests as TypeScript, except that it leaves `region` out of a create that names none. Its generated POST client parses a 201 body as Render's `Environment`.

**t002: the routes.** `POST` and `DELETE /v1/environments/{id}/resources` take an `sbg-` id. An add moves the group from any environment it was in, and a removal leaves it in none. The groups go first in a request, so a refusal on their account changes no other member. A failure after them returns each group to where it was, and a group that cannot be returned is named in `ENVIRONMENT_MEMBERSHIP_PARTIAL`. The write is a compare-and-set on the group's row (`store.SetSandboxGroupEnvironment`), so a group another request moved meanwhile is refused with `RESOURCE_PLACEMENT_CHANGED`, as a datastore is.
- An unknown `sbg-` id, or another workspace's, answers 404 `SANDBOX_GROUP_NOT_FOUND`, "sandbox group not found", before anything changes. This holds on a removal too, because the CLI's fake answers it there. The other kinds answer a foreign id with 403 on an add and ignore it on a removal; groups differ because Render's clients read the 404.
- Removing a known group from an environment it is not in changes nothing and answers 204, as for every other kind.
- The environment object lists no groups, and deleting an environment still keeps its groups with `environmentId: null`.

**t003: protected environments.** The existing checks cover a group with no new code. The target environment is checked by `authorizeMembershipChange`, and the environment a group leaves by the exit guard. Both answer 403 `PROTECTED_ENVIRONMENT_ADMIN_REQUIRED`. An environment that is isolated or has an IP allowlist, but is not protected, still needs `can_manage` with the plain 403, as for every other kind.

**t004: GraphQL and MCP.** The shape chosen is a group-side verb, as the SDK's `setGroupEnvironment` is one: GraphQL `setSandboxGroupEnvironment(id, environmentId, ownerId)` and MCP `set_sandbox_group_environment {id, environmentId?}`. A null or absent `environmentId` takes the group out of its environment. Both call `sandbox.Service.SetGroupEnvironment`, which sends the add, or the removal from the environment the group reads, to `environments.Service.ChangeResources`, the verb REST serves (`PlaceGroup`). A scoped key needs `projects.move` for both, as for REST's route, and both spend the `group-write` rate class. `TestGroupBehaviorsAgreeAcrossRESTGraphQLAndMCP` runs eleven move scenarios on all three surfaces.

**t005: what Render's dashboard offers** (bundle read 2026-11-23 22:47 UTC, entry `assets/index-9Nb9X1Dz.js`, 459 chunks, no sign-in):
- **The group's own page** never shows its environment and has no move control. Its breadcrumbs carry no project or environment, and Settings has only Name, a read-only Concurrency limit, and Delete.
- **At create,** the "New Sandbox Group" form has an optional Project field with an Environment select ("Add this sandbox group to a project once it's created").
- **The project page** lists an environment's groups in the same table as its services (`projectResources` asks each environment for `sandboxGroups`). Selected groups move with the bulk **Move** control, into another environment or out to the workspace, through the generic `changeEnvironmentForResources(resourceIDs, environmentID)` mutation. The row "Options" menu is not offered on a group.
- **Elsewhere:** the environment's "Move Services" dialog lists the workspace's groups as candidates, and the home "Ungrouped Services" table lists groups that are in no environment.
- **New finding:** the bundle refuses deleting an environment or project that holds the workspace's default group: "This ${t} contains the default sandbox group. Make another group the default first."

**t009: test coverage.** The gaps the task named are covered:
- one request moving groups out of two environments into a third (`TestChangeResourcesMovesSandboxGroupsFromSeveralEnvironments`);
- a clear of a group in no environment (the `clearWhenInNone` sequence, and a removal from an environment the group is not in);
- concurrent moves of one group, against the real store (`TestChangeResourcesConcurrentMovesOfOneSandboxGroupPG`): exactly the requests that land succeed, the rest are refused with `RESOURCE_PLACEMENT_CHANGED`, and the group ends in one environment;
- the rollback when a later kind fails, and a group that cannot be returned;
- another workspace's group;
- the store's compare-and-set (`TestSetSandboxGroupEnvironmentPG`).

Mutation spot-check (2026-11-23). Each mutation was applied alone and reverted; each one failed the tests listed:

| Mutation | Failed by |
| --- | --- |
| Drop the group kind from `classifyResources` | 20 tests, the environments unit and real-Postgres tests among them |
| Skip the source environment's exit check | `TestMovingASandboxGroupIntoOrOutOfAProtectedEnvironmentNeedsAnAdmin`, `TestRenderClientsReadTheGroupMoveRefusals` |
| Skip the group kind's rollback | `TestChangeResourcesFailureReturnsASandboxGroupToWhereItWas`, `TestChangeResourcesNamesASandboxGroupItCouldNotReturn` |
| Answer a foreign or unknown group 400 instead of 404 | the environments refusal test, the REST contract test, and the cross-surface "moving an unknown group" scenario |
| Apply groups after services, not first | the protected test. A first run of this mutation survived, because the test read only the end state; it now counts service writes, so a change that was undone also fails it |
| GraphQL and MCP clear skip the removal | two cross-surface scenarios |

**t005: bex's dashboard**, built in the form Render's uses (DoD line 4 amended to it, by the parity rule: Render's group page shows no environment, so bex's does not either).
- **The project page.** Each environment's card lists its sandbox groups among its resources: type "Sandbox Group", status "Ready", the Default badge, the group's region, no runtime, no row menu. They are counted under "All", with no type pill of their own, so the card keeps the five pills its skeleton previews.
- **Moving and removing.** Selected groups move with bulk Move, one `setSandboxGroupEnvironment` per group. A group that moved leaves the selection; a refused one stays with its reason. Manage resources has a Sandbox Groups tab that puts a group in (checked) or takes it out (unchecked), writing only the groups that changed.
- **A workspace whose bex-api has no sandboxes** reads no groups and renders as before.
- **Checked by a subagent run** on the local stub with injected groups, at 1200 px and 390 px: the rows, the dialog, a save, and a bulk Move.
  - The pending and ready states match, now that the conditional sixth pill is gone. That pill had moved the ready state about 36 px at 390 px.
  - At 390 px the Default and type badges had wrapped badly in the name cell; that was fixed.
- **Not built (w22/031):** the Overview's ungrouped list of groups in no environment, and a bulk-Move item that takes a selection out of its environment.
- `yarn lint` passes (one existing warning in `use-live-logs.ts`), and `yarn test` passes 845 files and 10,585 tests.

**t006: documents.** `docs/render-artifacts/sandbox-groups.md` (§ The project environment, Render's dashboard, bex's dashboard), ADR104 D7, ADR042 § Sandbox groups and ADR018's sandbox groups row now describe the move. None of them still says a group cannot move or that an official client never sends `environmentId`.

**t007: Render parity.** The eleven move scenarios of `TestGroupBehaviorsAgreeAcrossRESTGraphQLAndMCP` agree across REST, GraphQL and MCP, and the client sequences replay over REST. The dashboard moves a group where Render's does. On 2026-11-23, render.com/docs/sandboxes-creating still said only "In the Dashboard, you can optionally place a group in a project environment", and the live OpenAPI's `resourceIds` description was unchanged. Drift is filed as w22/030 (Render's dashboard refuses deleting an environment or project that holds the default group) and w22/031 (above). ADR018 lists both.

**t008: simplify.**
- Applied: the not-found test compares messages directly. The dashboard's conditional "Sandbox Groups" filter pill and its URL kind are removed, because the ready state no longer matched its skeleton.
- Kept: the 404 check reads the workspace's groups once more before the move, which one query costs. The GraphQL and MCP verbs also compute and discard the environment view `ChangeResources` builds for REST. Neither is on a hot path.
