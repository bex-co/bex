# w5 · m119 — Env and secret-file writes share one rollback path

**Worker:** worker5 **Goal:** A failed projection or App patch after a single-key env or secret-file write leaves store, Secret and App consistent, without erasing a concurrent committed write. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce the two failure orders of a single-key env write — **DONE** | 45m | — |
| t002 | Make single-key setters reuse the batch compensation and delete projectOrRestore — **DONE** | 1h | t001 |
| t003 | Name the rejected key when a projection is refused — **DONE** | 20m | t002 |
| t004 | Render parity — **DONE** | 15m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- A concurrent `SetEnvVar` committed between this call's CAS write and its failed projection survives.
- An App-patch failure after `upsertSecret` rolls back both the store and the Secret, leaving no mounted-but-unlisted value.
- A refused projection's 400 names the key and the rule.
- `projectOrRestore` is deleted, and single-key and batch paths share one compensation path.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `eb55d1e27` (w4/m168), which added `projectOrRestore` for single-key writes. The reviewer found both failure orders by reading the race and failure paths (medium confidence).
- **Goal linkage:** ADR006 env var API correctness; ADR008 reliability.
- **Expected outcome:** No lost env writes and no hidden mounted values.
- **Why now:** m168 just added this path, and w5/m118 t002 changes key validation in the same code.
- **Render parity included:** env var error shapes change on REST/GraphQL/MCP.

## Evidence — 2026-10-05

**Both failure orders, reproduced then fixed (t001, t002).** `rollback_test.go` drives the real service with a versioned store and an App-patch interceptor.

- **Order 1, a concurrent committed write.** Call A CAS-writes; call B commits another key and answers 200 from inside A's App patch; then A's patch fails.
  - At HEAD, A's restore wrote the map from before A over the store. B's write was erased while the Secret B projected still mounted it (`store after A's failure = map[KEEP:1]`).
  - Now the store and the Secret keep B's write, and A answers 409 `ENVIRONMENT_RESTORATION_FAILED`. Checked for a single-key write and for a batch patch as call A.
- **Order 2, the App patch fails after the Secret update.** At HEAD the store went back but the Secret kept the new value, mounted but not listed, for an env var and a secret file alike. Now both go back.

**One compensation path (t002).**

- `projectOrRestore` and `restoreSourceMaps` are deleted. The single-key setters (`SetEnvVar`, `SetEnvVars`, `SeedEnvVars`, `SetSecretFile`, `SeedSecretFiles`) and the sparse batch patch all call `compensateEnvironment`. The revision-aware patch keeps its own revision-owned branch inside it.
- `updateMapCAS` returns the write it made (`mapWrite`): the map it replaced, the version it wrote, and whether it changed anything. `restoreMap` puts a map back only through a CAS against that version, so a write committed since is never erased: that map is reported superseded and left, with its Secret, to the newer writer.
- A restored map's Secret is put back with it. Only a Secret the write created over an empty, unreferenced map is deleted.
- `SetEnvVars` and `SeedEnvVars` used to write the store unconditionally. They now write through the CAS loop, so their restore can tell a later write apart.

**Named refusal (t003).** A projection Kubernetes refuses as invalid answers 400 `ENVIRONMENT_VARIABLE_INVALID` or `SECRET_FILE_INVALID`, the w5/m118 codes, with the same params as the name's own refusal (`field`, `maxLength`). The message names the key once, then the rule from the refusal's cause; before, every such refusal read "the change cannot be applied to the service". `TestRefusedProjectionNamesTheKeyAndRule` covers a single-key env write, a secret file and a batch patch. Only the service's own env or files Secret is read this way: any other refusal, its App's included, is returned unchanged.

**Render parity (t004).**

- REST, GraphQL and MCP share these verbs, so they answer the same codes: REST `code`, GraphQL `extensions.code`, MCP error text.
- The dashboard's environment editor already relays the server's refusal (`mutationErrorMessage`), so the named key and the 409 reach the user with no UI change.
- Render's env-var API documents no restoration semantics. The 409 and the codes are bex's existing error superset, now also on the item verbs. A successful write answers exactly as before.
- Recorded in ADR013, including the window these Secret rollbacks still leave (they are not revision-owned), filed as w5/091.

**`/simplify` (t005)**, three reviews. Applied:

- The quality review found four bugs in my first cut. Each is now pinned by a test that fails on revert:
  - Compensation deleted a Secret whenever the App this call had read did not reference it. A concurrent first write that committed before this call's CAS and referenced its Secret lost that Secret, so its pods could not start. A Secret over a non-empty restored map is now put back instead (`TestRestoredWriteKeepsTheSecretAConcurrentFirstWriteReferences`).
  - The batch patch undoes its env write when the secret-file write fails. That undo ignored a superseded restore and answered the file's 400, although the env change survived. It now answers 409 `ENVIRONMENT_RESTORATION_FAILED` (`TestBatchPatchWhoseEnvWriteWasSupersededSaysSo`).
  - `refusedProjection` treated any invalid refusal as a key refusal, picking the kind by a `-files` name suffix: an App refused on a service named `x-files` answered `SECRET_FILE_INVALID`. It now reads only the service's own two Secrets (`TestRefusedProjectionLeavesOtherRefusalsAlone`).
  - `restoreMap` wrote unconditionally whenever the version was 0, even against a versioned store. Now only a store without versions is written that way.
- The refusal message repeated the key (`"POISON" …: Invalid value: "POISON": …`) and had its own params. It is now core's `EnvKeyProjectionRefused` / `SecretFileProjectionRefused`, beside the name rule it shares a code with, and over-long keys are not echoed.
- One `mapWrite` record replaces the per-caller `prior` clones, three different derivations of "changed", the mirrored `envWriteTxn`/`filesWriteTxn` constructors, `casWriteVersion`'s pointer-as-flag and the seven-argument restore closure.
- `core.InvalidDetails` is shared by `InvalidFieldsError` and `refusedProjection`.
- Tests reuse `patchCountingClient` (a new one-shot `beforeFail` hook) and `secretData`.
- Stale comments fixed: `updateMapCAS`, `compensateEnvironment`, `PatchEnvironment` and the ADR013 bullet. A `gofmt` double blank line in `core/config.go` is gone.

Changed test: `TestPatchEnvironmentCompensatesStoreAndProjectionsWhenAppPatchFails` now has two cases:

- over stored maps, each Secret holds the restored map;
- over empty maps, the Secret the patch created is removed.

It used to expect deletion over stored maps too. From the failing call that case looks exactly like the concurrent first write above. The operator reads only Secrets an App references, so an unreferenced Secret that mirrors the store does nothing.

Skipped:

- A failed store restore still answers the cause joined with the compensation error, as the batch patch always has. Giving it the revision-aware path's `ENVIRONMENT_RESTORATION_FAILED` is a separate decision.
- Sharing `restoreMap` with `compensateCASEnvironment`: the revision-aware path needs the restored version for its ownership check and maps every failure to one code.
- Efficiency: nothing to change. One `a.DeepCopy()` per write is negligible.

**Tests (t006).**

- New: `rollback_test.go` (both failure orders, the concurrent first write, the superseded batch env write), `TestRefusedProjectionNamesTheKeyAndRule` and `TestRefusedProjectionLeavesOtherRefusalsAlone`.
- Each fix fails its test when reverted:
  - an unconditional restore fails both concurrent-write tests;
  - deciding deletion from the stale App fails two;
  - ignoring a superseded env undo fails one;
  - suffix classification fails one;
  - keeping the repeated key fails one.

**Gates.**

- The backend suite on fresh Postgres, OpenFGA and OpenBao is green (68 packages); `/ship`'s gate runs it again before the push.
- `make lint`: 0 issues in all four modules, including the whole-program dead-code pass.
