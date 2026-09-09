# Environment Groups live parity — 2026-08-17

This note records the authenticated Render reference walk and the post-fix Bex acceptance for w2/m73. It intentionally contains no environment value, secret-file content, session cookie, token, kubeconfig, or reusable credential. Random values were compared only in memory and were discarded with the synthetic resources.

## Environments

- **Render reference:** authenticated `dashboard.render.com/env-groups` walk on 2026-08-17, earlier in the same implementation session. The populated list, create/edit/move/import/export/clone/search/delete surfaces supplied the product reference for this milestone.
- **Bex acceptance:** production dashboard build from base `dc30786fafce` plus the m73 working tree, exercised at 19:45 PDT against the local CAPD application cluster. The browser used a real Ory registration/session and the real bex-api, PostgreSQL control plane, Kubernetes projections, and an ephemeral OpenBao KV-v2 instance. Local authorization used the repository's explicit insecure-development checker, so authorization enforcement itself is covered by permanent Core/adapter tests rather than claimed as production evidence.
- **Transport note:** the production dashboard CSP correctly rejects cross-origin plain-HTTP services. The isolated Playwright context bypassed CSP only so the unchanged production bundle could reach localhost port forwards; application authentication and server-side secret custody were not bypassed.

## Behavior matrix

| workflow | Render reference | Bex post-fix result | non-secret request/evidence |
| --- | --- | --- | --- |
| Empty and populated list | Search plus Name, Environment, Env Vars, Secret Files, and Updated columns; distinct empty/no-match states | Pass. The table, scope labels, counts, timestamp, search/reset, and both empty states rendered correctly | `EnvGroups(ownerId)` plus one batched `EnvGroupScopeIndex(ownerId)` query; no per-project Environment reads |
| Create and dotenv import | Create accepts staged variables and `.env` import | Pass. Imported key and secret-file name appeared masked after create | `CreateEnvGroup`; the ordinary response was recursively checked to contain no `value` field |
| Generated value | Generate stores a non-empty masked value | Pass. Existing-group staged Generate saved, remained masked, and a fresh single-key reveal proved it was non-empty | `PatchEnvGroupEnvironment(generateValue: true)` followed by explicit `EnvGroupVarValue`; no ordinary response carried plaintext |
| Duplicate name | Exact workspace duplicate is rejected | Pass. The second create returned `ENV_GROUP_NAME_EXISTS`; no second row was created | Workspace/name CAS claim plus legacy metadata sweep; concurrent-winner coverage is permanent |
| Staged edit and Cancel | Several variable/file changes stay local until Save; Cancel discards them | Pass. Cancel caused no write. One later save atomically renamed a key and file and added a generated key | One revision-bearing `PatchEnvGroupEnvironment` mutation with `save_only`; stale/compensation paths are covered by Core repetition/race tests |
| Scope and Move | Group shows Environment scope and can move to a compatible target | Pass. Workspace → Environment A persisted; moving to Environment B while linked to an A service was refused atomically | `MoveEnvGroup`; unrelated Environment membership remained intact |
| Link candidates | Only services compatible with the group's Environment are offered | Pass. Environment B service was absent from the selector; the A service linked successfully. A direct incompatible move remained server-rejected | Dashboard uses the workspace scope index; Core revalidates service and group scope |
| Per-value and bulk export | A named value can be copied; all vars can be copied/downloaded | Pass. One fresh reveal populated the clipboard. Bulk copy/download contained every saved key in stable dotenv form | Fresh `EnvGroupVarValue` calls only; the fail-closed exporter emitted nothing unless every reveal succeeded |
| Clone | Variables/files copy to the chosen workspace/Environment; links do not | Pass. Server-side clone into Environment B had the same secret contents, zero service links, and no values in the response/client cache | `CloneEnvGroup`; equality was checked through explicit reveals after creation |
| Delete | Deleted row disappears immediately | Pass. Clone and source disappeared without reload; final server count and rendered row count were both zero, and the empty state replaced the list | `DeleteEnvGroup`, Apollo eviction, and collection navigation; intentional eviction no longer races the dead-id redirect |
| Bex extensions retained | Render does not expose group-side pre-linking/MCP management in the observed UI | Pass. Create-time service linking and detail-side link/unlink remained available | Bex REST/GraphQL/MCP extensions remain documented and secret-safe |

## Cross-surface contract

The browser uses GraphQL. REST and MCP are thin translations over the same Core verbs and are covered by adapter regressions for the changed shapes:

- literal-or-generated single and batch values reject both-set/neither-set input with the same coded error;
- REST `PATCH /v1/env-groups/{id}/contents`, GraphQL `patchEnvGroupEnvironment`, and MCP `patch_env_group_environment` accept the same sparse patch, opaque revision, and `save_only | deploy | rebuild` modes;
- REST `PATCH .../environment`, GraphQL `moveEnvGroup`, and MCP `move_env_group` share atomic scope validation;
- REST `POST .../clone`, GraphQL `cloneEnvGroup`, and MCP `clone_env_group` clone only on the server and return metadata/names, never values;
- `ENV_GROUP_NAME_EXISTS`, `ENV_GROUP_NAME_AMBIGUOUS`, `ENV_GROUP_METADATA_CONFLICT`, revision conflicts, and restoration outcomes map consistently across adapters.

Render has no official env-group MCP tools in the observed/documented surface. Bex's MCP tools are deliberate API extensions, not a claim about Render wire compatibility.

## Metadata concurrency repair (w4/m97, 2026-09-08)

Source review plus deterministic Core tests (not a fresh authenticated Render walk). Render's docs establish shared configuration, membership, scope, and rollout; they do not document multi-replica metadata CAS. Bex evidence:

| interleaving | expected | coverage |
| --- | --- | --- |
| Content save paused on revision read vs rename | Rename survives; content lands | `TestContentSavePreservesConcurrentRename` |
| Failing content save vs rename | Compensation restores env/files only; rename survives | `TestContentCompensationPreservesConcurrentRename` |
| Two concurrent links | Both service ids in meta; both Apps keep Secret refs | `TestConcurrentLinksPreserveBothMemberships` |
| Stale-link prune vs new link | Stale id removed; new link kept | `TestPruneStaleLinksPreservesConcurrentLink` |
| Delete then delayed meta writer | `ErrNotFound`; no resurrection | `TestDelayedMetaMutationDoesNotResurrectDeletedGroup` |
| Two concurrent renames | One committed name; losing name claim free | `TestConcurrentRenamesLeaveOneCommittedName` |

Dashboard surfaces unchanged (names-only reads, save modes). Live OpenBao/Kubernetes drill notes: [docs/drills/2026-09-08-env-group-metadata-cas.md](../drills/2026-09-08-env-group-metadata-cas.md).

## Cleanup

The acceptance runner deleted each source/clone group, both synthetic services and their App projections, both Environments, the Project, Workspace, Ory identity, and generated tenant namespaces in a `finally` path. A post-run cluster check found no `tea-da1*` namespace. The test-only OpenBao Helm release and `secrets` namespace were removed, its short-lived JWT/config files were deleted, and `bex-controller-manager` was restored and verified at `1/1` Ready. No screenshots or browser profile were retained because the matrix and network assertions are sufficient and carry less secret-handling risk.

The CAPD recovery performed during the pass replaced unhealthy worker Machines; any local-path volumes formerly tied to those disposable workers are not production evidence and were not treated as durable state.
