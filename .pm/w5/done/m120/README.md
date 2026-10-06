# w5 · m120 — Env-group links store service ids and are removed when the service is deleted

**Worker:** worker5 **Goal:** Deleting a service unlinks it from every env group, and a later service with the same name is never treated as linked. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce a same-name service after delete — **DONE** | 30m | — |
| t002 | Service delete unlinks its env groups — **DONE** | 45m | t001 |
| t003 | Store service ids in link sets and migrate existing names — **DONE** | 1h | t002 |
| t004 | Delete the deleted-service unlink fallback — **DONE** | 20m | t003 |
| t005 | Render parity — **DONE** | 15m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- After deleting "web" and creating a new "web", the group lists no linked service, and Unlink doesn't restart the new service.
- Deleting a linked service removes it from every group, leaving no dangling entries.
- Existing name-based link entries are migrated to ids; dangling ones are dropped and logged.
- `unlinkDeletedService` and its cluster-wide List are gone.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `ee1341133` (w4/183). Scenario: a Blueprint links group G to "web"; "web" is deleted and an unrelated "web" is created. G now shows the new "web" as linked, and Unlink restarts it through `rollLinked`. w4/183's fix covers only the not-found case.
- **Goal linkage:** ADR006/ADR018 env group parity; ADR008 reliability.
- **Expected outcome:** Env-group links can't leak to unrelated services.
- **Why now:** w4/183 patched one symptom on 2026-10-04, and the same-name restart is a cross-service side effect.
- **Render parity included:** env-group link lists change on every surface.

## Evidence — 2026-10-06

**Reproduced (t001).** `TestEnvGroup_ANewServiceWithALinkedNameIsNotLinked`: a Blueprint (`LinkEnvGroup`) links a group to "web" by name; "web" is deleted and an unrelated "web" is created. At HEAD the group listed `[web]`, and unlinking "web" restarted the new service (`restartedAt` set). Both pass now.

**Delete unlinks (t002).**

- `apps.Delete` calls the new `EnvGroupLinks` seam first, before the clone-secret delete, the secrets purge, the store row and the CR. The seam is `envgroups.WorkspacePurger.UnlinkApp`, wired in `api/server.go`. It removes the service from every group its spec mounts, under its id or any name it was linked by.
- The choice t002 asked to record: a failure fails the delete, with nothing irreversible done yet, and the retry completes it.
- Another workspace's group, and a group already deleted, are skipped. A group that lists a service without mounting it, after an interrupted unlink, is pruned by its next patch.
- Tests:
  - `TestDeleteUnlinksEnvGroupsBeforeRemovingTheService` (apps): a failed unlink keeps the row and the CR, and the retry unlinks, then deletes.
  - `TestWorkspacePurger_UnlinkAppLeavesNoGroupListingIt`, for groups that stored the id or the name.
  - `TestWorkspacePurger_UnlinkAppSkipsAnotherWorkspacesGroup`.
  - `TestEnvGroups_DeletingAServiceUnlinksIt`, through the wired server: a REST link by name stores the id, and a REST delete of the service empties `serviceLinks`.

**Ids, not names (t003).**

- Every link write stores `core.AppPublicID` (exported from core for this): `LinkService`, Blueprint `LinkEnvGroup`, create's `serviceIds` and Blueprint initial groups. A service linked by name and then by id is linked once. Unlinking by name removes the id entry. A link survives a rename (`TestEnvGroup_ALinkSurvivesARename`); a link stored as a display name used to go stale once the service was renamed again.
- A group patch, an unlink and a group delete act only on a service whose spec mounts the group, and a patch prunes any other entry. No path restarts an unrelated service, whether the group has been migrated or not.
- The migration runs once per group, gated by a new `linksByID` marker. The marker is set at create, by the migration, and when a link is added to an empty set.
  - It runs after authorization (`fetchGroup`), in `ApplyEnvGroup`'s found branch, and on env-group list reads after filtering, with one tenant-scoped App list per workspace.
  - A name a mounting service answers to, its display name included, becomes that service's id. A mounting service no stored name reaches keeps its link. Any other name is dropped and logged.
  - Ids stay. A deleted service's id is pruned by the next patch or dropped by unlinking it, and an id may reserve an App still being created.
  - The marker is set only once every name was resolved, so a name an old replica stores during the rollout is migrated by a later read.
  - `TestEnvGroup_MigratesLegacyLinksToServiceIDs` covers names, display names current and changed, a name whose service never mounted the group, a deleted name, a deleted id, and the marker.
- A create's `pendingServiceIds` are ids too; they were CR names.

**Fallback removed (t004).** `unlinkDeletedService` and its cluster-wide App list are gone. Unlinking a deleted service drops its link only when the identifier is a well-formed service id in the link set (`dropDeletedLink`). Ids are never reused, and `AuthorizeApp` answers an id it finds but the caller may not touch with a refusal, not not-found. A deleted service's name leaves through the migration or the next patch. The w4/183 tests are rewritten on ids.

**Render parity (t005).**

- REST, GraphQL and MCP share `EnvGroupView.ServiceLinks`, ids on all three now.
- The dashboard already keys links by service id (the linked-services card and the service's env-groups panel). A Blueprint-linked group now shows as linked in the service's panel, where a name entry never matched.
- Render's `serviceLinks` are `envGroupLink` objects with `id`, `name` and `type` ([retrieve env group](https://api-docs.render.com/reference/retrieve-env-group)); bex's are id strings. This drift predates the milestone and is filed as w5/092.
- Docs: ADR006 (links paragraph) and ADR018 (env-group row).

**`/simplify` (t006)**, three reviews. Applied:

- The quality review found the first cut did not fully close the bug:
  - A Blueprint sync still restarted the unrelated service: `ApplyEnvGroup` patched a group found by name without migrating it, and the patch fan-out resolved "web" by name. The group is now migrated there, and the fan-out acts only on mounting services (`TestEnvGroup_PatchSkipsAServiceThatDoesNotMountTheGroup`, `TestEnvGroup_PatchPrunesAListedServiceThatDoesNotMountTheGroup`). A group delete detaches only mounting services too (`TestEnvGroup_DeleteDetachesOnlyServicesThatMountTheGroup`).
  - The migration dropped a link stored as a display name. Display names are aliases now, and a mounting service the names no longer reach keeps its link.
  - Two races: a concurrent read could drop a reservation for an App being created, or bake in a name stored after its App list. Ids are never dropped now, and the marker waits until every name is resolved.
  - A group deleted while a list migrated it turned the whole list into a 404. It now leaves the list (`TestEnvGroup_ListSkipsAGroupDeletedDuringItsMigration`); any other migration failure keeps the stored links for that read.
  - The unlink ran after the irreversible secrets purge; it now runs first.
- Efficiency: the migration listed every App in the cluster, through bex-api's uncached client, once per legacy group. It did so on list reads, before authorization, and for Environment views that never read links. It now lists the group's workspace by label, once per request, after authorization and filtering, and only for env-group reads and verbs.
- No-op writes: a `mutateMetaCAS` mutate can answer `errMetaUnchanged`. `dropLinks` uses it, so a delete retry no longer bumps a group's `updatedAt`.
- Reuse: `removeString` is variadic, and one `dropLinks` replaces five copies of remove-then-CAS (unlink, stale-id drop, delete unlink, patch prune, initial-link rollback). The api test reuses `decodeJSON`.
- Comments: the seam's satisfier, `dropDeletedLink`, `detach`'s split exits, and the marker spelled `"1"` like the package's other stored flags. `mountedGroups` checks the env-group id kind.

Skipped:

- One fake for both delete seams in the apps tests: they are two seams with different assertions.
- A shared Secret-name-to-group-id parser for apps' two copies and `mountedGroups`: apps cannot import envgroups, and each copy is a few lines.
- Hand-applied Apps (no id) stay keyed by their object name, which can be reused. API creates always mint an id.

**Tests (t007).** All the tests named above are new except the w4/183 pair, which is rewritten. The initial-links reservation test now expects the id. Each fix fails its test when reverted (12 mutations):

- storing a name instead of the id fails 4 tests, including the wired-server one;
- skipping the unlink in delete fails 2;
- leaving the seam unwired fails the wired-server test;
- adopting by name without the mount check fails 2;
- the fan-out, `detach` and unlink mount checks each fail theirs;
- dropping the tenant guard, keeping a deleted group in the list, dropping dangling ids, not adding unnamed mounting services and never setting the marker each fail theirs.

**Gates.**

- The backend suite on fresh Postgres, OpenFGA and OpenBao is green (68 packages); `/ship`'s gate runs it again before the push.
- `make lint`: 0 issues in all four modules, including the whole-program dead-code pass.
