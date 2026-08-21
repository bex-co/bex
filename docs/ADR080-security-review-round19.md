# ADR080: Security review round 19 — N7MdJ2 disposition

- **Status**: Accepted (2026-08-21)
- **Scan**: codex-security `N7MdJ2`, repository revision `0ac1b6b5` (reviewed against HEAD after ADR079), 8 findings (2 high, 5 medium, 1 low)
- **Lineage**: nineteenth pass in the ADR028 → … → ADR079 lineage

## Summary

Seven findings are fixed in place with regression tests. Findings 1/2/4 close a gap in the CI/CD environment-gate pattern ADR079's own lineage established for `infra.yml`/`snapshot.yml`/`cli-release.yml`: three privileged jobs whose own comments named "GitHub-side protected environments" as the authoritative control never actually carried a job-level `environment:`. Finding 3 (fail-open role demotion) was mid-flight in the working tree when this round began; this pass completed it and fixed two test-fixture bugs that were masking whether the new revoke-before-grant/fail-closed-veto logic actually ran. Findings 5 and 7 close a matched pair of CAS gaps in the secrets store — an unconditional metadata `Delete` inside the optimistic-concurrency retry loop, and four sparse secret-file/environment mutation paths that never entered that loop at all. Finding 6 replaces an unanchored cross-tenant Prometheus selector with the exact Traefik service identity the request-log pipeline already computes correctly. Finding 8 is the standing onbex.co PSL residual (fourteenth report).

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | App-cluster reconciliation runs without an environment gate | high | **Fixed** — `environment: production-cluster` on `bootstrap-app-cluster`; required reviewers are an operator action |
| 2 | Platform deployment runs without an environment gate | high | **Fixed** — `environment: production-deploy` on `build-and-deploy`; splitting build from deploy stays a follow-up |
| 3 | Failed role demotions retain higher workspace authority | medium | **Fixed** — revoke-before-grant + fail-closed pending-reconciliation veto, completed this round |
| 4 | OpenBao restore drill runs without an environment gate | medium | **Fixed** — `environment: production-restore` on `restore`; required reviewers are an operator action |
| 5 | Deleting the final environment variable bypasses CAS | medium | **Fixed** — `updateMapCAS`'s empty-map branch now writes through `PutCAS`, never an unconditional `Delete` |
| 6 | Legacy App metrics can include other tenants' Traefik series | medium | **Fixed** — exact `<namespace>-<app>-<port>@kubernetes` selector replaces the unanchored substring match |
| 7 | Sparse secret mutations can overwrite concurrent credential updates | medium | **Fixed** — `SetSecretFile`/`DeleteSecretFile`/`SeedSecretFiles`/sparse `PatchEnvironment` now share the CAS retry loop |
| 8 | Shared tenant hosting suffix permits cross-tenant cookie poisoning | low | **Accepted residual** — onbex.co PSL (fourteenth report); `.pm/DO_NOT_DO.md` `#PSL` |

## Findings 1 / 2 / 4 (high / high / medium) — CI/CD jobs missing their named environment gate

`app-cluster.yml`'s `bootstrap-app-cluster`, `deploy.yml`'s `build-and-deploy`, and `openbao-restore-drill.yml`'s `restore` each carry a `if: github.ref == 'refs/heads/main'` ref guard and a comment pointing at "GitHub-side protected environments (see infra.yml)" as the authoritative control — but unlike `infra.yml`'s `terraform` job, `snapshot.yml`'s `bake` job, and `cli-release.yml`'s `release` job (all of which already carry an explicit `environment:`, added by an earlier round's `815e003b fix(security): remediate repository security review`), none of these three jobs actually referenced one. The main-ref guard alone is not an independent approval: a malicious change that reaches main runs automatically, with no distinct review gate between repository compromise and cloud/cluster-admin credentials (`bootstrap-app-cluster`, HCLOUD_TOKEN + admin.conf), production release/identity/secret-store authority (`build-and-deploy`), or OpenBao's root token and full Shamir unseal quorum (`restore`).

**Fix.** Each job now carries `environment: production-cluster` / `production-deploy` / `production-restore` respectively, matching the existing three-job pattern byte-for-byte (a job-level `environment:` field, referencing repository secrets exactly as before — environment protection rules are additive, so nothing about secret access changes until rules are configured). This is a code-level fix only: **an operator must still create these three environments in repository settings and attach required reviewers** for the gate to be load-bearing, identical to what `production-infra`/`production-snapshot`/`production-release` already require. Finding 2's fuller remediation (splitting secretless build from credentialed deploy so the environment gate brackets only the credentialed half) is not attempted here — `build-and-deploy` remains one job sharing one set of environment-scoped secrets across build and deploy steps; that split is a larger, riskier refactor left as a follow-up.

## Finding 3 (medium) — fail-open role demotion (completed)

A role demotion's Postgres row commit and OpenFGA tuple reconciliation are not atomic; OpenFGA's model ORs the five role relations, so a revoke failure during a grant-then-revoke ordering left a demoted member's old (higher) tuple effective until the 15s reconciliation worker caught up. A prior, unfinished pass in the working tree had already reordered `ChangeRole`/`reconcileExactRole`/`reconcileInviteRole` to revoke before granting (`lego/backend/internal/members/service.go`, `lego/backend/internal/api/tenancy.go`) and added a `guardCallerRoleSettled` veto — refusing every `members` `can_manage` verb while the **caller's own** role reconciliation is still pending, closing the window ordering alone cannot: a fresh OpenFGA check during that window still answers from the caller's stale, possibly-higher tuple.

That logic was correct, but its own regression tests were not exercising it: three new tests passed `svc(st, g, nil, g)` (a non-nil checker) without ever granting the caller's `can_manage` tuple in the fake granter, so `AuthorizeFreshOn` refused every call before it reached the revoke/grant/veto logic under test — the tests were green for the wrong reason (an authorization gate one line earlier), and one used a `viewer` target role against a fixture plan (`pro`) whose `guardPlanRole` doesn't allow it, hitting the same false-positive pattern for a different reason. **Fixed**: seeded the caller's `can_manage` tuple in `TestChangeRoleRevokesBeforeGrant`, `TestChangeRoleDemotionGrantFailureFailsClosed`, and `TestPendingRoleReconciliationRefusesCallerCanManage`, and swapped the two `pro`-plan-incompatible target roles to `developer`. All three now fail against the pre-fix grant-then-revoke ordering and pass against the fix; `HasPendingRoleReconciliation` (`lego/backend/internal/store/role_reconciliations.go`) and the invite-redemption revoke-before-grant path (`lego/backend/internal/api/tenancy.go`) were already correct and needed no change.

## Finding 5 (medium) — CAS bypassed on the final-key delete

`updateMapCAS`'s optimistic-concurrency retry loop (`lego/backend/internal/secrets/service.go`) read a versioned snapshot, applied the caller's mutation, and wrote back with `PutCAS` at the observed version — except when the mutation emptied the map, where it instead called `s.Store.Delete`, OpenBao's unconditional metadata delete (all versions, no CAS parameter). A `DeleteEnvVar` on a service's last variable could therefore read version N, have a concurrent `SetEnvVar` commit N+1, and then delete the path anyway — destroying the newer credential and every retained version.

**Fix.** The empty-map branch is gone; an empty result now writes through the same `PutCAS(path, current, snapshot.Version)` as every other mutation, so a concurrent writer's newer version produces `core.ErrConflict` and a retry (re-reading the fresh map, which correctly no longer needs deleting) instead of data loss. `TestDeleteEnvVarFinalKeyRaceSurvivesConcurrentSet` pins the race with a `versionedFakeSecretStore.afterGet` hook that lands a concurrent `SetEnvVar` in the read/write gap and asserts the concurrent key survives.

## Finding 7 (medium) — sparse secret mutations bypassed CAS entirely

`SetSecretFile`, `DeleteSecretFile`, and `SeedSecretFiles` (`lego/backend/internal/secrets/files.go`) and the no-revision `PatchEnvironment` path (`patchEnvironmentSparse`, `lego/backend/internal/secrets/batch.go`) each did a plain `readMap` followed by an unconditional `storeMap` — read-modify-write with no compare-and-set at all, unlike `SetEnvVar`/`DeleteEnvVar`, which already used `updateMapCAS`. A concurrent writer committing between the read and the write was silently discarded.

**Fix.** All four now go through `updateMapCAS`'s `GetVersioned`/`PutCAS` retry loop. `patchEnvironmentSparse` keeps its sparse contract (no caller-supplied `ExpectedEnvRevision`, no conflict surfaced) by re-applying the env/secret-file patch against the freshly re-read map on every retry, tracking the pre-write snapshot for its existing best-effort compensation path. `TestPatchEnvironmentSparseSurvivesConcurrentSingleFieldWrite` pins the batch race the same way finding 5's test does; the existing `secrets` package suite (including the CAS compensation/rollback tests) passes unchanged.

## Finding 6 (medium) — legacy App metrics cross-tenant Traefik selector

`promQueryFor` and `NewPrometheusFilterValuesSource` (`lego/backend/internal/metrics/source.go`) built an unanchored `service=~".*<app>.*"` PromQL selector with no namespace, so a legacy bare-name App (`web`) could aggregate or enumerate request metrics from any other tenant's Traefik service whose label happened to contain that text. `docs/ADR010-observability.md` already documents the correct, ground-truthed identity — "Traefik names an Ingress-backed service `<namespace>-<app>-<port>@kubernetes`" — and the request-log pipeline (`deploy/gitops/base/log-shipper.yaml`'s `stage.regex`) already reconstructs it exactly; only the Prometheus request-metrics path had drifted to a substring heuristic.

**Fix.** A new `traefikServiceLabel(namespace, app, port)` helper reproduces the same identity, and `promQueryFor`/`NewPrometheusFilterValuesSource` now match it with an exact `service=%q` equality selector (never a regex). `RequestMetricsRequest` gained a `Port` field and `MetricsFilterValuesSource`'s signature gained a `port int32` parameter, both populated from `app.Spec.EffectivePort()` — the same port value `app_controller.go` gives the App's backing Service and Ingress backend, so this is a reconstruction of a known identity, not a new heuristic. `TestPromQueryForAndFilterValuesAreNamespaceScoped` pins that the selector is exact (never `=~`) and that a same-named App in a different namespace, or a service name that merely contains the App's name as a substring, cannot match.

## Finding 8 (low) — onbex.co PSL (accepted, fourteenth report)

Unchanged standing residual: ADR055 F9 → ADR072 #1 → ADR061 #4 → ADR063 #3 → ADR064 #8 → ADR069 #3 → ADR073 #6 → ADR076 #10 → ADR077 #8 → ADR079 #9 → here. `.pm/DO_NOT_DO.md` `#PSL` holds the decision; re-open at the open-signup gate.

## Follow-ups

- **Operator action**: create the `production-cluster`, `production-deploy`, and `production-restore` GitHub environments (repository Settings → Environments) and attach required reviewers, matching `production-infra`/`production-snapshot`/`production-release`. Without this, findings 1/2/4's code-level fix is present but not yet load-bearing.
- Finding 2's fuller remediation (secretless build separated from credentialed deploy) remains open.
- onbex.co PSL submission (finding 8, fourteenth report): `.pm/w1/050.md` — do not unset `BEX_BASE_DOMAIN` (`.pm/DO_NOT_DO.md` `#PSL`).
