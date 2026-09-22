# w4 · m130 — `patchServiceEnvironment` applies the write and then reports a conflict

**Worker:** worker4 **Goal:** a revision-aware environment save either applies and says so, or refuses and changes nothing — so a client doing a correct read-modify-write is never told to retry a save that already succeeded. **Status:** blocked (t002–t007 done; t001 needs original production failure attribution; t008 awaits deployment and live verification)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on   |
| ---- | ------------------------------------------------------------------------------------------ | --- | ------------ |
| t001 | Diagnose why a CAS patch that durably applies its write returns `ENVIRONMENT_REVISION_CONFLICT` — **BLOCKED** | 45m | —            |
| t002 | Make the outcome and the response agree: a landed write never reports a conflict — **DONE** | 40m | w4/m130/t001 |
| t003 | Return the new revision from `EnvironmentPatchResult`, as the env-group sibling already does — **DONE** | 25m | w4/m130/t001 |
| t004 | Carry the fix across REST and MCP, which expose the same `expectedEnvRevision` parameter — **DONE** | 30m | w4/m130/t002 |
| t005 | Render parity — the environment save surface across REST/GraphQL/MCP/UI — **DONE** | 30m | w4/m130/t003, w4/m130/t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed — **DONE** | 25m | w4/m130/t005 |
| t007 | Test coverage — a conflict means nothing was written; a success reports success — **DONE** | 45m | w4/m130/t005 |
| t008 | Closeout — close the milestone once the definition of done actually holds — **BLOCKED** | 15m | w4/m130/t007 |

## Definition of done

- **A correct read-modify-write loop succeeds cleanly, repeatedly.** Read `service.envVar(key:).revision`, immediately call `patchServiceEnvironment(saveMode:"save_only", envVars:[one var], expectedEnvRevision:<that revision>)`, read back. Twenty consecutive iterations return no error and land the value. Today **4 of 8** such iterations returned `ENVIRONMENT_REVISION_CONFLICT` — _"the service environment changed; refresh it before saving again"_ — **while landing the write anyway** and advancing the revision.
- **A conflict means nothing changed.** When `ENVIRONMENT_REVISION_CONFLICT` is returned, the value and the revision are both unchanged from before the call — assert both, not just the error. Today the error is returned for writes that landed, which is the defect; a fix that merely suppresses the error would satisfy "no error" while leaving the response still unable to describe the outcome.
- **A genuinely stale revision is still refused, and still writes nothing.** Replaying a revision that a later write superseded returns the conflict and leaves the value untouched. Verified working today (`t2_replayStale`, `t3_otherKeysRev`), so this is the regression guard, not new behavior.
- **The patch result carries the new revision.** `EnvironmentPatchResult` gains a `revision` field, so a client chaining two saves does not have to re-read between them. Its env-group sibling `EnvGroupEnvironmentPatchResult.revision` already does this, and the group path was verified clean 4-for-4 in the same pass — the two results should be shaped alike.
- **REST and MCP behave identically.** Both expose the same parameter (`secrets/rest.go`, and `secrets/mcp.go:101`, whose schema advertises it as an _"optional opaque revision from list_env_vars or get_env_var"_). The same twenty-iteration loop run through each surface returns no spurious conflict.
- **Concurrency is covered by a test that fails without the fix.** Two writers from one observed revision resolve to exactly one success and one conflict — the invariant `secrets/batch.go:177-180` states in its own comment — and the conflicted writer's value is provably absent.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 158 (w4-targeted, `muse.env` credentials), journey 4 (env vars, secret files, env groups). Fixtures `qa-20260921-env158` (`srv-dap0kurbdpcs73f5ecmg`) and env group `qa-20260921-grp158` (`evg-dap0mnbbdpcs73f5eco0`), both deleted; workspace back to baseline.
- **Goal linkage:** ADR006 (bex-api semantics) and the environment-save surface the dashboard's Environment editor, the CLI and MCP all write through. The CAS protocol exists specifically so two editors cannot silently overwrite each other; a protocol whose success path reports failure cannot be relied on for that.
- **Expected outcome:** a client — the dashboard editor, an agent over MCP, or a script — can implement read-modify-write against a service's environment and trust the answer. Today the safest-looking usage (always pass the revision you just read) is the one that produces spurious errors, so the incentive is to omit the parameter and lose the protection.
- **Why now:** the failure mode is the dangerous direction. An error on a **successful** write invites the caller to retry, and the documented remedy in the error text — _"refresh it before saving again"_ — is exactly what the successful trials already did. A client that surfaces the error tells the user the change did not save when it did; a client that retries writes again. Either way the protocol designed to prevent lost updates is producing wrong outcomes. And the sibling implementation being clean means the fix has a working reference in-repo rather than needing a design.
- **Render parity task included** — the environment save is exposed on REST, GraphQL and MCP and is driven by the dashboard's Environment editor, so a change touches all four.

## Evidence (live, 2026-09-21 pass 158)

**1. The loop, run exactly as the error text instructs — refresh immediately before every save.** Each trial reads the current revision, sends one env-var update with it, then reads back:

```
trial   sentRevision          response                                    valueAfter  writeLanded
r1      evr1_AAAAAAAAAAY      OK                                          z1          true
r2      evr1_AAAAAAAAAAc      ERROR ENVIRONMENT_REVISION_CONFLICT         z2          true   ←
r3      evr1_AAAAAAAAAAg      ERROR ENVIRONMENT_REVISION_CONFLICT         z3          true   ←
A       evr1_AAAAAAAAAAo      OK                                          q1          true
B       evr1_AAAAAAAAAAs      OK                                          q2          true
C       evr1_AAAAAAAAAAw      ERROR ENVIRONMENT_REVISION_CONFLICT         q3          true   ←
D       evr1_AAAAAAAAAA0      OK                                          q4          true
(t1)    evr1_AAAAAAAAAAU      ERROR ENVIRONMENT_REVISION_CONFLICT         y1          true   ←
```

**Eight calls, eight writes landed, four reported a conflict.** The revision advanced on every one, including the four that errored. Intermittent, not deterministic: 60-second idle periods before `A` and `D` did not prevent `C` from failing between them.

**2. Control — omitting the parameter is always clean.** `patchServiceEnvironment` with no `expectedEnvRevision` returned `OK` and landed the value. So the spurious error is specific to the revision-aware path, not to environment saves generally.

**3. Control — a genuinely stale revision behaves correctly.** Replaying a superseded revision, and using a revision observed for a different key, both returned the conflict and left the value at `y1` — unchanged. So the conflict detection is not simply broken; it fires correctly on real conflicts **and** spuriously on non-conflicts.

**4. The decisive control — the sibling CAS path is clean, 4 for 4.** `patchEnvGroupEnvironment` on an env group, same protocol, same store, same pass:

```
g1  egr1_AAAAAAAAAAI → OK newRevision=egr1_AAAAAAAAAAM   gg1  landed
g2  egr1_AAAAAAAAAAM → OK newRevision=egr1_AAAAAAAAAAQ   gg2  landed
g3  egr1_AAAAAAAAAAQ → OK newRevision=egr1_AAAAAAAAAAU   gg3  landed
g4  egr1_AAAAAAAAAAU → OK newRevision=egr1_AAAAAAAAAAY   gg4  landed
```

No spurious conflict, **and** the result returns the new revision so the next save needs no re-read. This is what the service path should look like and is the strongest evidence the defect is in the service path's extra work, not in CAS or OpenBao.

**5. The revision token is service-wide, not per-key** — `QA_PATCH_PROBE` and `QA_MARKER` both reported `evr1_AAAAAAAAAAU` in the same read. Worth stating because the natural first guess is a per-key token, and it is wrong.

## Where the cause is not

t001 starts here rather than from scratch. `ENVIRONMENT_REVISION_CONFLICT` comes from `envRevisionConflict()` (`secrets/batch.go:350-356`), which has exactly two call sites in `patchEnvironmentCAS`:

```go
snapshot, err := versionedStore.GetVersioned(ctx, envPath(service))
...
if snapshot.Version != expectedVersion {
    return EnvironmentPatchResult{}, envRevisionConflict()        // (a) BEFORE any write
}
...
newVersion, putErr := versionedStore.PutCAS(ctx, envPath(service), env, expectedVersion)
if putErr != nil {
    if errors.Is(putErr, core.ErrConflict) {
        return EnvironmentPatchResult{}, envRevisionConflict()    // (b) PutCAS reported failure
    }
```

- **(a) cannot be the source** — it returns before `PutCAS`, so no write could have landed.
- **(b) means `PutCAS` reported a conflict.** `openBaoStore.PutCAS` (`secrets/store.go:236-264`) maps OpenBao's 400/409 onto `core.ErrConflict` and has no retry of its own. Its `kv()` wrapper retries **once**, but only on `403`, which is a denied request that cannot have applied a write.
- **The post-write failure paths return different errors** — `envUpdateRestored()` / `envRestorationFailed()` (`batch.go:358-370`) — and the observed `extensions.code` was `ENVIRONMENT_REVISION_CONFLICT`, so compensation is not what fired.

So the live behavior contradicts a straightforward reading of the code, which per the hunt's own rule makes this a fork to resolve rather than a footnote. The leading hypothesis, stated as a hypothesis: **a concurrent writer to `envPath(service)`** — the derived-Secret claim in `projectCASEnv`, or the App write / rollout in `finalizeEnvironmentPatch`, neither of which the env-group path performs — advances the stored version around the CAS window, so a write lands and a subsequent comparison still fails. t001 must confirm or refute that by instrumentation, not by inspection.

## Not part of this milestone

- **Whether `saveMode:"deploy"` behaves the same** was not probed; every trial used `save_only` to avoid rollout churn. t001 should check both, since the deploy mode does strictly more work and the hypothesis above implicates exactly that extra work.
- **The 20-iteration DoD loop is deliberately longer than the 8 trials run here.** Four failures in eight is a high enough rate to characterize, but not to prove a fix; the DoD asks for a run long enough that the current failure rate would almost certainly appear.

## Diagnosis (2026-09-21, implementation investigation)

The original "exactly two call sites" premise is incorrect in the current code. `projectCASEnv` can fail after the accepted source CAS, and `compensateCASEnvironment` returns `ENVIRONMENT_REVISION_CONFLICT` when its restoring CAS or projection ownership check loses to a newer writer. `TestPatchEnvironmentCASRollbackDoesNotClobberProjectionLandedAfterSourceRestore` explicitly asserted that post-write error. A newer writer can preserve this request's submitted key while changing another key, leaving the submitted value stored although compensation reports a revision refusal. This reachable mechanism is being exercised against real OpenBao; it does not by itself prove the cause of the original unattended production failures.

The source-path census is 12 mutation sites across 11 logical operations: `SetEnvVars`, `SetEnvVar`, `DeleteEnvVar`, `SeedEnvVars`; `patchEnvironmentCAS`, `patchEnvironmentSparse`, `compensateCASEnvironment`, `restoreSourceMaps`; `prepareEnvVars` (write and failure cleanup), `abortEnvVars`, and the purger's normal/legacy-path deletion loop. `projectCASEnv` only reads OpenBao. Successful finalization writes Kubernetes; it reaches source writes only on compensation. REST, GraphQL, and MCP are the three public patch adapters. `OryTransport` is an ordinary Go transport, the POST has no idempotency headers, and `kv` retries only a rejected 403. No evidence supports an applied POST retry here.

The correction reserves `ENVIRONMENT_REVISION_CONFLICT` for rejection before this request's source write is accepted. Incomplete post-write compensation uses the existing `ENVIRONMENT_RESTORATION_FAILED` error, preserving newer writes and requiring a refresh; successful compensation retains `ENVIRONMENT_UPDATE_RESTORED`. A failed projection must not be misreported as a successful deploy. The original production frequency and deployed 20-iteration proof remain open until verified, regardless of local results.

### Reproduction and verification evidence

Real OpenBao 2.5.5, with the production store adapter and controlled fake Kubernetes resources, reproduced the misleading error before the fix: source version 1; request GET 200, submitted CAS POST 200 (version 2), injected other-key CAS POST 200 (version 3), projection GET 200, restoring CAS POST 400; final version 3 retained the submitted key. The old response was `ENVIRONMENT_REVISION_CONFLICT`. The corrected response is `ENVIRONMENT_RESTORATION_FAILED`, without overwriting either key. The recorded sequence comes from `TestPatchEnvironmentRealOpenBaoPostWriteConflict`; a Go overlay of the old implementation failed this regression as expected. No secret values or paths were logged.

`TestPatchEnvironmentRealOpenBaoSequentialAndStale` ran 20 chained writes on each of REST/GraphQL/MCP in each of `save_only` and `deploy` (120 total, alternating changes/no-ops), zero spurious errors in either mode. Each iteration verified the returned revision and persisted value, then replayed the stale revision and checked the exact map and version remained unchanged. `TestPatchEnvironmentRealOpenBaoConcurrentWriters` observed two initial reads, one POST 200, one POST 400, and one persisted winner. These tests use real KV storage and fake Kubernetes; they do not validate Kubernetes rollout or production auth. They are wired into backend CI with a digest-pinned ephemeral OpenBao.

Existing env-group tests now tie concurrent success to the final value/revision and cover its intentional no-op behavior: env groups retain a logical content revision for unchanged content, whereas service CAS advances the physical store revision to consume one observed token. The production report's original single-caller frequency remains unexplained by this injected interleaving. A deployed trace identifying the competing writer or other failure is still owed; the milestone must remain blocked until that diagnosis and its deployed definition of done are verified.

### Review and validation notes

Three `/simplify` reviews covered reuse, quality, and efficiency. Applied all findings: reuse the store HTTP helper for mount setup and `StrField` for GraphQL; remove compensation's unused cause argument; correct the names-only documentation; join both concurrent test writers before assertions can tear down their mount. The revision-field mutation overlay fails all six API/mode tests without the new contract; the compensation overlay fails all three targeted post-write cases. Existing unaffected assertions are regression controls, not claimed to fail on old code.

The first full-suite run caught an API-level expectation of the old post-write error, which was updated. Its workspace lifecycle failure came from reusing `BEX_TEST_OPENBAO_URL`, which enables a distinct Kubernetes-auth test; the new ephemeral KV fixture now uses `BEX_TEST_OPENBAO_KV_URL`/`BEX_TEST_OPENBAO_KV_TOKEN`. A separate mount attempt timed out on the overloaded local Docker host. The native Postgres instance also retained the old full run's fixed gateway test role privileges in the `postgres` database; those test-only grants were cleared before final verification. No production store or other workstream's fixture was changed.

The remaining KV test setup failure was traced to OpenBao's explicit HTTP400 “Upgrading from non-versioned to versioned data” response immediately after mounting KV-v2. The fixture now waits on the read-only configuration endpoint before its first write; there is no retry around CAS mutations. This startup race is separate from the observed production service-save defect.

The readiness correction passed three consecutive complete real-OpenBao regression runs. It follows OpenBao 2.5.5's [`config` read upgrade check](https://github.com/openbao/openbao/blob/v2.5.5/builtin/logical/kv/path_config.go) and [asynchronous KV upgrade](https://github.com/openbao/openbao/blob/v2.5.5/builtin/logical/kv/upgrade.go), polls only the read-only endpoint under a deadline, and leaves tested writes single-attempt. The dedicated no-op regression also asserts zero App patches, zero audit events, no restart timestamp, and an advanced returned revision.

### Final local gate

Passed the full backend suite with `GOWORK=off go test -p 1 ./...`, real native Postgres, real OpenFGA, and the real OpenBao KV fixture enabled (`BEX_TEST_DB_URI`, `BEX_TEST_OPENFGA_URL`, `BEX_TEST_OPENBAO_KV_URL`, `BEX_TEST_OPENBAO_KV_TOKEN`). Backend lint reports zero issues; image-pin validation passes88 references; workflow YAML parses; Markdown formatting and diff whitespace checks pass. Logs for this run are `/tmp/bex-w4-m130-green.log` and `/tmp/bex-w4-m130-lint-green.log`. These are local results, not a claim about CI or production rollout. No dashboard source changed, so no dashboard suite was needed.
