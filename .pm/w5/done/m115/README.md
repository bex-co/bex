# w5 · m115 — By-id verbs: one resolver, one 403/404 rule, and a guard that checks types instead of source text

**Worker:** worker5 **Goal:** Every by-id verb on REST, GraphQL and MCP resolves its workspace through `core.ScopeByID`, answers non-members per one documented ADR072 rule and audits the refusal; a type-level guard makes a new id kind impossible to wire otherwise. **Status:** done — 2026-10-05

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Write the by-id policy matrix into ADR072 and the backend guide — **DONE** | 30m | — |
| t002 | Move Blueprints and API keys onto ScopeByID — **DONE** | 45m | t001 |
| t003 | Make ScopeByID route only, with one core policy for choosing the workspace — **DONE** | 1h | t002 |
| t004 | Fix environment create and device unregister outside the default workspace — **DONE** | 45m | t003 |
| t005 | Make routing hooks required interface methods — **DONE** | 30m | t003 |
| t006 | Replace the source-text guard with an id-kind registry — **DONE** | 1h | t003 |
| t007 | Render parity — **DONE** | 20m | t004, t005, t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007, t008 |
| t010 | Closeout — **DONE** | 10m | t009 |

## Definition of done

- Every workspace-owned by-id family on REST, GraphQL and MCP, including Blueprints and API keys, answers per one documented ADR072 matrix (named exemptions only).
- A non-member refusal writes one audit row, and an ownerless API key revoke writes exactly one.
- Environment create in another workspace's project, and device unregister without ownerId, act in the right workspace.
- The guard fails when a workspace-owned id kind has no owner lookup or exemption, and a comment can't satisfy it.
- Backend tests (real DB) and lint pass, and w4/m172's DoD probes still pass.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05. The same fix class landed four times in one day: w4/m169 (`9a95c445c`), w4/194 (`0401d251e`), w4/m172 (`d1e4c9315`), and w4/199 (`f837cec46`), which restored ADR072 #8's typed-id 403 after m172 collapsed it and turned deploys red. Code-verified: Blueprints and API keys still answer non-members 404 while `ScopeByID` families answer 403, and the guard matches source text.
- **Goal linkage:** ADR006/ADR018 Render-compatible by-id API; ADR072 #8 typed-id semantics; ADR012 authz and audit.
- **Expected outcome:** By-id verbs behave the same everywhere, every refusal is audited, and a new id kind can't skip routing.
- **Why now:** Four same-class fixes in 24 hours, one of which broke deploys; the remaining stragglers are known.
- **Render parity included:** status codes of by-id verbs change on REST/GraphQL/MCP.
- **Gap vs w4/m172 (blocked on its live replay, t007):** m172 deliberately kept Blueprints (m169) and API keys (194) unchanged ("Controls, unchanged" in its README). This milestone is that deferred consolidation. Re-run m172's DoD probes after it ships.

## Evidence — 2026-10-05

**Matrix (t001).** ADR072 gains "By-id resolution matrix (amendment, w5/m115)", and `lego/backend/AGENTS.md`'s authz-seam line links it. It names three exemptions that answer an unreachable id like a missing one: GitHub installations (enumerable integers), sandboxes (per-workspace keys, no table) and caller-owned rows (push notifications, device subscriptions, claim selections, SSH keys). It states that the 403 discloses existence, and that this is accepted: typed ids are xids, so a neighbour of a known id is guessable. The API-key exemption t001 anticipated wasn't needed, because Hydra key ids get the typed-id 403 too.

- **Deviation: one row is split.** A named workspace that does not hold the id answers 403 from Apps, Postgres, Key Value and API keys (w6/m14's cross-workspace gate) and 404 from the store-backed families, whose lookups search only that workspace. A single answer would mean changing w6/m14's gate and its e2e, or giving every store-backed verb an unscoped existence read. The matrix states the split, and w5/086 picks one answer.

**Blueprints and API keys (t002).** `blueprintScope` and `revokeScope` route through `ScopeByID`, and the `isMember` 404 collapse is gone.

- A non-member's typed Blueprint id gets 403; a missing one gets 404 (`blueprint_owner_scope_test.go`).
- An ownerless revoke writes one audit row. A non-member or viewer revoke gets 403, recorded once (`apikeys_test.go`).

**ScopeByID routes only (t003).** The verb's own `Authorize` refuses and writes the one audit row (`TestScopeByIDLeavesTheRefusalToTheVerb`). `ResourceOwner` returns every owning workspace, and one core `pickOwner` chooses among them. It takes the caller's default if listed, else the only one they belong to. Two or more of theirs is a 409 `OWNER_AMBIGUOUS`, whose message names `ownerId` and MCP's `workspaceId`. None of theirs falls back to the first, where the verb refuses.

- Membership goes through `requireMember`, so an unreachable membership store fails closed.
- The per-package pickers are gone: events' `preferredWorkspace` and `EventWorkspaceRouter`, notifications' `errNotificationNotFound`, GitHub's `errInstallationNotVisible` and `forbiddenAs`, and apps' `isMember`.
- `ScopeByVisibleID` carries the exemptions (GitHub installations and claim selections, sandboxes).

**Environment create and device unregister (t004).**

- Environment `Create` and `CreateWithACL` route by the project's workspace and authorize once there. Previously the gate ran on the caller's default, refusing an admin of the project's workspace who was a viewer in their default and auditing the refusal in the wrong place (`TestCreateEnvironmentAuthorizesInTheProjectsWorkspace`). A create that arms an ACL gates `can_manage` up front.
- A named workspace that does not hold an environment now reads 404. Before, the verb acted in the environment's own workspace and authorized twice (`TestEnvironmentVerbsStayInTheActingWorkspace`).
- Device unregister without `ownerId` routes to the device's own workspace through `DevicePushSubscriptionWorkspaces`, keyed by the caller's subject (`TestUnregisterDeviceRoutesToTheDevicesOwnWorkspace` and a Postgres test). Migration 0143 adds the `(subject, device_id)` partial index for that read.

**Required hooks (t005).** `EventStore` requires `ServiceEventWorkspaces`, with a compile-time `PGStore` assertion. The notifications store requires `PushNotificationWorkspaces` and `DevicePushSubscriptionWorkspaces`, and the sandbox `KeyProvider` embeds `PurgeKeyLookup`. A sweep finds no optional routing assertion left; the four remaining optional assertions are deploy narration, the tenant advisory lock, and app-creation rollback and completion.

**Guard (t006), with a deviation.** `byid_resolution_guard_test.go` parses each family's routing helper and requires the resolver call inside it. A comment, a string or another family's call doesn't count (`TestPackageCallsIgnoresMentions`). `TestEveryIDKindIsClassifiedForByIDRouting` makes each of the 33 id kinds name the family that resolves it, or say why none does.

- Mutation checks all fail the guard: removing webhooks' routing call, leaving `dsk` unclassified, and stripping `scopeDevice`'s call while the inbox still calls `ScopeByID`.
- Owner reads stay as per-family closures, not the wiring-time registry in core that t006's steps describe. Core can't import the families, and the test-side classification fails the same way when a kind is added.
- The behavioral half is per-family tests: a member reaches their other workspace's resource with no `ownerId`.

**Render parity (t007).** Render's by-id endpoints take no owner, and bex now routes by the id, so no `ownerId` is required anywhere Render requires none. REST, GraphQL and MCP share the verbs, so the changed answers are the same on each: the Blueprint and API-key non-member 403, environment create in another workspace's project, and the 409 (REST maps it; GraphQL and MCP pass the message through). The typed-id 403 is ADR072 #8's documented divergence. The dashboard needs no change, since it only offers the caller's own workspaces.

**`/simplify` (t008), first pass over the milestone diff, applied:**

- owner-read closures as one-liners, because the store's routing reads now return none for an unknown id;
- `pickOwner` through `requireMember`, the one fail-closed membership contract, stopping at the second member workspace;
- the coded 409 with a message correct on MCP;
- `actingProject` fails closed when no acting workspace resolves;
- sandboxes on `ScopeByVisibleID`, with a one-allocation default-first probe order;
- reuse of the package's `recordingSink`, and stale docs (webhooks `Get`, the AGENTS.md create rule, `ScopeByVisibleID`, the ADR's "unguessable");
- the guard's per-helper check, and migration 0143's index.

Skipped:

- a string-keyed core `RequireInActingWorkspace`: the strict version broke fixtures that wire no resolver, so environments check only when an acting workspace resolves;
- a shared parser helper for two test guards (low value);
- reusing the project `scopeProject` fetched (one extra single-row read on create).

Reverted: the first pass offered "answer 404, or list it as an exception" for API keys' named mismatch. 404 broke `TestReadSideOwnerIDTargetingE2E`, which locks w6/m14's 403, so API keys keep 403 and the matrix lists the split.

**`/simplify`, second pass over the post-review delta.** Two of the reviews independently found that every environment write recorded two "allowed" rows. `requireEnvironment` re-authorized the relation the verb's gate had just checked, in the same workspace. That predates m115, but routing made it provably redundant. The api audit sweep never saw it because it builds environments without a store. Fixed:

- One acting-workspace check, `inActingWorkspace`, now serves `requireEnvironment` and `actingProject`. A row outside the acting workspace reads 404. The row's workspace is authorized again only when no resolver is wired, and an unresolvable acting workspace fails closed.
- Each environment verb has one gate. `SetACL` gates `can_manage`, and `CreateWithACL` and `Update` gate `can_manage` when the request arms an ACL. This also fixes two older bugs the review found:
  - `CreateWithACL` with an ACL answered 403, audited in the project's workspace, where the matrix says 404, which made it an existence probe;
  - a developer's rename-plus-ACL `Update` saved the rename before refusing.
- `TestEnvironmentWritesRecordOneDecision` and the extended `TestEnvironmentVerbsStayInTheActingWorkspace` fail against the old code (two rows; a saved rename; 403 instead of 404). Raising `SetACL`'s gate is load-bearing: dropping the duplicate check without it let a developer set ACLs (mutation-checked).
- Sandbox routing skips the probe when the caller's only workspace is their default, since the verb's own path looks there. Moving the keyless probe above the membership listing was tried and reverted: a fixture without a listing then routed, which is a behavior change, not a simplification.
- `coretest` passes `ctx` through, and four more identical membership fakes (in the secrets, Postgres, apps and webhooks tests) now use it.
- Stale test docs in `events/byid_test.go` and the migration's comment are fixed.

Skipped:

- one shared helper for inbox and device routing: the guard checks each family's own helper, so a shared one would need a second-level check;
- listing the candidates in `OWNER_AMBIGUOUS`: the early exit stops at two;
- a batched membership read for non-members: N is one to three in practice.

Filed: w5/087 (sandbox verbs rewrite the tenant-key row on every call). The `List` named-workspace case joins w5/086.

**Tests (t009).**

- `core/coretest` (`Members`, `Workspaces`) replaces the six fakes w4/m172 added and three identical `memberships` copies (apps, Postgres and Key Value list tests). Core's own `multiWorkspace` stays, because package `core` can't import `coretest`.
- `github/m172_byid_scope_test.go` is renamed `byid_workspace_routing_test.go`.
- Hollow assertions fixed. The sandbox fixture gains member workspace `tea-c` with no key, and the test fails if routing mints one; routing through `WorkspaceKey` fails it. The inbox test seeds an unread item in the default inbox and asserts it stays unread.
- The Postgres and Key Value `actions_owner_test.go` copies are merged into `TestActionCapabilitiesGraphQLWorkspaceSelection`'s table: `serverActions`, `deployActions`, `databaseActions` and `keyValueActions`, seven cases each. Wiring `databaseActions` with `IDVerb` fails it, and so does dropping Key Value's acting-workspace check.

**Gates.**

- Backend suite on fresh Postgres 17, OpenFGA and OpenBao (`scripts/backend-test-deps.sh`). The first run caught the API-key named-mismatch regression above. After the revert, and again after the second `/simplify` pass, the full suite is green (68 packages).
- `make lint`: 0 issues in all four modules, after each pass.

**w4/m172.** Its DoD probes are a live replay in bex-canary after deploy, which is its own blocked t007, and m115's Source schedules the re-run after this ships. In code, m172's tests pass in the suite above. With m115, members still resolve Blueprints and API keys by id (its "controls" bullet), and non-members get the typed-id 403 its adjacent-classes bullet requires. m172's board disagrees with itself on a mismatched `ownerId`: its DoD says 403, like a non-member, while its README line says 404. Store-backed families answer 404, unchanged by m115. That question is w5/086.
