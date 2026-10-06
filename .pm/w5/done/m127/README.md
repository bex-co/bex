# w5 · m127 — Every env and secret-file projection carries its source revision

**Worker:** worker5 **Goal:** A Kubernetes env or secret-file Secret never moves back to an older store revision, because every writer projects through one revision-owned primitive and every rollback touches only a Secret its own write still owns. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Inventory every projection writer and the revision each can stamp — **DONE** | 45m | — |
| t002 | One revision-owned projection primitive for env and secret files — **DONE** | 1h | t001 |
| t003 | Route the batch patch, setters, file materialization and create-time seeding through it — **DONE** | 1h | t002 |
| t004 | Compensation rolls back only a Secret its write still owns — **DONE** | 1h | t003 |
| t005 | Deletes, which project before they commit, under the same ownership — **DONE** | 45m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 1h | t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- Writer A commits revision v5 and writer B commits v6 and projects it. When A then projects, B's v6 stays in the Secret, which still matches the store.
- A restore racing a newer writer's projection leaves the newer projection in place: w5/m119's remaining window, ADR013.
- Env and secret-file Secrets carry their source revision, and an update or delete of one carries UID and resourceVersion preconditions.
- A store without versions (test doubles) keeps today's unconditional behaviour.
- ADR013 records that the window is closed.

## Source + Goal linkage

- **Source:** promoted from inbox w5/091, filed by w5/m119 (2026-10-05).
  - Only the revision-aware env patch (`projectCASEnv`) owns its Secret by source revision.
  - `projectEnv`, `projectFiles` and `compensateEnvironment` call `upsertSecret`, which neither checks nor updates a revision.
  - So a stale projection can land over a newer one, and a restore's Secret write can overwrite a projection that landed after its CAS.
- **Goal linkage:** ADR006 env var API correctness; ADR013 secret store; ADR008 reliability.
- **Expected outcome:** The env and secret-file values a pod mounts always match the store's latest revision, or a newer one still in flight, never an older one.
- **Why now:** w5/m119 made the store's rollback revision-safe and named this as the window it left. The revision-aware env path already shows the shape.
- **Render parity omitted:** no surface changes. The same verbs answer the same codes, and only which projection wins a race changes.

## Inventory and decisions (t001, 2026-10-06)

Every writer of a `<service>-env` or `<service>-files` Secret, in `internal/secrets`:

| Writer | Order | Revision it knows |
| --- | --- | --- |
| `patchEnvironmentCAS` → `projectCASEnv` | commit, then project | the CAS write's version (revision-owned today) |
| `patchEnvironmentSparse` → `finalizeEnvironmentPatch` → `projectEnv` / `projectFiles` | commit, then project | `txn.env.version` / `txn.files.version` |
| `SetEnvVars` → `projectEnv`; `SetEnvVar`, `SeedEnvVars` → `materializeEnv` | commit, then project | `updateMapCAS`'s `mapWrite.version` |
| `SetSecretFile`, `SeedSecretFiles` → `materializeFiles` | commit, then project | `mapWrite.version` |
| `DeleteEnvVar`, `DeleteSecretFile` → `deleteMapKeyAfterProjection` | **project, then commit** | none yet: the commit comes after |
| `prepareCreateEnvVars`, `prepareSecretFiles` → `prepareProjection` | store, then create the Secret | none: `storeMap` returns no version |
| `compensateEnvironment` → `restoreMap`, then `upsertSecret` / `deleteSecret` | restore, then project | none: `restoreMap` drops `PutCAS`'s version |

`updateMapCAS` reports version 0 for a store without versions (test doubles).

**Decisions.**

1. **One primitive, `projectSource`, by kind.** Env keeps `app.bex.co/env-source-revision`; files get `app.bex.co/files-source-revision`.
   - A Secret already at a newer revision is left to that write: the projection is superseded.
   - The same revision with the same data is a no-op; with other data it is a conflict.
   - An update or delete carries the read object's resourceVersion, and a delete also its UID.
   - Revision 0 keeps today's unconditional upsert. (As built, t002 tells a store without versions by its type, so a versioned path never written, at version 0, is ordered like any other revision.)
2. **The CAS env patch** keeps its store pre-check and turns "superseded" into its revision conflict. Every other writer treats superseded as success: its write committed, and a newer write owns the Secret.
3. **Deletes take the setters' discipline:** commit the reduced map through `updateMapCAS`, project at the revision committed, and compensate on failure. Projecting first would have to stamp the revision its commit will create, and a concurrent writer that commits that revision first would then fail its own projection on a revision collision. An already-absent key still re-projects the current map before answering not-found.
   - **Revised in t005:** deletes keep projecting first. Committing first broke the contract `TestDeleteSecretFileRetryRollsAfterAppPatchFailure` pins: a failed delete leaves the store holding the key and the Secret without it. The collision above is avoided by marking the delete's revision `app.bex.co/source-revision-provisional`: a write that commits that revision replaces it, and the delete's commit then conflicts and re-reads.
4. **Create-time seeding** stamps the version its store write produced.
5. **Compensation** projects the restored map at the revision its restore committed (`restoreMap` returns `PutCAS`'s version), so it cannot overwrite a newer writer's projection.
