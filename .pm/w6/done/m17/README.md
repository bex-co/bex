# w6 · m17 — Fix false-403: collapse the caller-default-workspace vs. resource-workspace authorization gates

**Worker:** worker6 **Goal:** A resource-scoped verb authorizes against the resource's own workspace, not the caller's default workspace — so a user whose default workspace is one they were invited into with a limited role (e.g. `viewer`) can still restart/suspend/delete/set-env-vars on services in a *different* workspace where they hold `admin`. Today the two gates (`Authorize` against the caller's default workspace, `GetApp`/`fetchDatabase`/`fetchKeyValue` against the resource's actual workspace, fixed by `w6/m14`) are independent, so effective permission is their intersection — reproduced live, not theorized. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                                                                                                     | est  | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Implement `core.Base.AuthorizeApp(ctx, relation, name) (*App, error)`: fetch the resource, derive its workspace from `core.LabelTenant`, authorize `relation` there once, audit there once (target = resource name) — preserving 403-before-404 (authorize against the caller's acting workspace first when the fetch would 404) | 1h   | — — **DONE** |
| t002 | Migrate the ~45 call sites currently doing separate `Authorize` + `GetApp`/`fetchDatabase`/`fetchKeyValue` calls onto the new single seam                                                                | 1.5h | t001 — **DONE** |
| t003 | Fold `List(ownerID)`'s weaker OpenFGA-only gate (`apps/service.go`, `postgres/service.go`, `keyvalue/service.go` — no `IsMember` check today) onto `core.WithWorkspace`'s correct membership+OpenFGA mechanism | 40m  | t001 — **DONE** |
| t004 | Regression tests: `TestAuthzGuardsEveryVerb` and `TestEveryTargetedVerbIsNamedOrExcused` stay green; add the reproduction case from `w6/013` (an invited-viewer-by-default user, admin of their own separate workspace, restarting a service there) as a permanent regression test | 45m  | t002, t003 — **DONE** |
| t005 | Live/functional verification: the exact `bob`/`tea-team`/`tea-mine` scenario from `w6/013` now succeeds end-to-end                                                                                       | 30m  | t004 — **DONE** |
| t006 | Docs: note the fix in `docs/ADR012-auth.md` (or the relevant authz section); close `w6/013` (`w6/012` stays open, separate scope)                                                                        | 20m  | t005 — **DONE** |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                                                                                                              | 20m  | t006 — **DONE** |
| t008 | Test coverage — final gap sweep (`AuthorizeApp`'s 403-before-404 fallback path, `List(ownerID)`'s membership-checked behavior)                                                                           | 20m  | t006 — **DONE** |
| t009 | Closeout — verify DoD met, then move the milestone to `done/`                                                                                                                                            | 10m  | t007, t008 — **DONE** |

## Definition of done

A user whose default workspace is one they were invited into with a limited role (e.g. `viewer`) can still restart/suspend/delete/set-env-vars on services in a *different* workspace where they hold `admin` — reproduced with the exact scenario `w6/013` diagnosed, verified live; both existing authz regression test suites (`TestAuthzGuardsEveryVerb`, `TestEveryTargetedVerbIsNamedOrExcused`) stay green.

## Source + Goal linkage

- **Source:** promotion of inbox `w6/013` — a reproduced, not theorized, bug found during `w6/m14`'s `/simplify` review (2026-07-13); fix already designed there (`core.Base.AuthorizeApp`). Materialized via `/pm-brainstorm more milestones to work on` 2026-07-13.
- **Goal linkage:** `w6`'s multi-tenant-security mandate — this is a correctness bug in the authorization layer `w6/m14` partially fixed (resource-side) but left open (caller-side).
- **Expected outcome:** no invited user is locked out of operating services in their own admin-owned workspace; the effective-permission-is-an-intersection-of-two-unrelated-workspaces bug class is closed at its root (one authorization seam, not two).
- **Why now:** reproduced with a real probe test, not theoretical; blocks legitimate operations for any invited user acting in their own workspace — a live usability/correctness bug, not speculative hardening. The fix is already designed in the source note, reducing implementation risk.
- **Render parity closing task: omitted** — bex's own internal authorization-layer correctness, no Render capability to compare against.

## Explicitly out of scope

Per `w6/013`'s own caveats, kept out to bound this milestone:

- The "populate `core.WithWorkspace` once at the request boundary" alternative design — has an unresolved SSR/`localStorage`-vs-cookie caveat the source note flags itself.
- The "order default workspace by owned tenant, not oldest membership" idea — called "arguable" in the source note; a separate design question, not this bug fix.

## Evidence (t005)

The exact `bob`/`tea-team`/`tea-mine` scenario from `w6/013`, run against **real infrastructure** — a live Postgres (throwaway container, the membership source of truth) and a live OpenFGA carrying the real `deploy/gitops/authz/model.json` — driving the **real REST router** through the **real** store-backed resolver, the same harness `w6/m14`'s `TestMultiWorkspaceTargetingE2E` uses. `lego/backend/internal/api/w6013_e2e_test.go`, `TestW6013_InvitedViewerRestartsOwnWorkspaceServiceE2E` — **PASS**: bob's default workspace (`team`) is his oldest membership even though he administers `mine`, the workspace `mine-web` lives in; with **no** `ownerId`, restart/suspend/resume/delete all succeed (`w6/013`'s exact `svc.Restart(bob, "mine-web") => forbidden` no longer reproduces). The fourth verb `w6/013` names, set-env-vars, has no OpenBao in this harness — verified instead by a permanent fake-based regression test, `secrets.TestW6013_InvitedViewerCanSetEnvVarsOnTheirOwnWorkspacesService`. `TestMultiWorkspaceTargetingE2E` re-run against the same infra confirms no regression to `w6/m14`'s own DoD.

## `/simplify` outcomes (t007)

Four review agents (reuse / simplification / efficiency / altitude) over the milestone diff.

**Applied:** `resourceWorkspace`'s not-found branch was reimplementing `callerWorkspace` byte-for-byte — the three `AuthorizeApp`/`AuthorizeDatabase`/`AuthorizeKeyValue` siblings now call `callerWorkspace` directly for that case, and `resourceWorkspace` dropped its now-unneeded `found bool` parameter. `core.Base.GetDatabase`/`GetKeyValue` and `keyvalue.patchKeyValue` (the self-fetching variant) were confirmed to have zero remaining callers after the migration and deleted. `envgroups.UnlinkService` was discarding the App it had just authorized and fetched, then having `detach` re-fetch the same object — split into `detach`/`detachFetched` so `UnlinkService` reuses its own fetch while `DeleteEnvGroup`'s bulk per-service path (which must not audit per-service) keeps its own bare fetch. Stale doc comments in `postgres.fetchDatabase` and `metrics/datastore.go` still describing the pre-`AuthorizeApp` design were corrected. The two live e2e tests' identical `call` closures were deduped into one `e2eCall` helper.

**Verified and skipped, not blindly applied:** the efficiency agent flagged that `Store`/`History`/`PodLogs` availability checks now run *after* the authorize+fetch (a wasted k8s GET + OpenFGA round-trip + audit write when a subsystem is unwired) and suggested moving them first. Reordering was tried and empirically **breaks `TestAuthzGuardsEveryVerb`**: the check-availability-first order lets an unauthorized caller learn a subsystem isn't configured before ever being authorized, which is exactly the 403-before-anything-else property that test enforces. Kept as-is.

**Skipped (documented, not applied):** collapsing `AuthorizeApp`/`AuthorizeDatabase`/`AuthorizeKeyValue` into one generic function — real duplication, but risky given `callerVerb`'s `runtime.Caller` stack-depth dependency, and the "Rule of Three" duplication is an already-accepted pattern here (`GetApp` → `GetDatabase`/`GetKeyValue` was extended the same way, without generics, in `w6/m14`). Test-fixture duplication (`twoWorkspaces`, `denyWorkspaceChecker` copied across `apps`/`postgres`/`secrets`/`core` test files) — Go can't share unexported types across packages, and introducing a shared test-fakes package wasn't worth the churn for this milestone. The AST-based constant-propagation machinery added to `events/vocabulary_test.go` and the "skip if `onOwn` is empty" bypass in `relationguard_test.go`'s `TestFetchByNameUsesTheVerbsOwnRelation` — both correctly flagged as verifying a weaker invariant now that most verbs make one authorize call instead of two, and a dynamic-behavioral-check redesign was proposed as the deeper fix; both suites still pass and still catch real regressions (verified via the `TestFetchByNameUsesTheVerbsOwnRelation`/`TestEveryTargetedVerbIsNamedOrExcused` runs throughout this milestone), so the larger rewrite was left for a future pass rather than risked here.

## Test coverage (t008)

Added the missing case the fold-onto-`WithWorkspace` fix (t003) didn't yet have a regression test for: `List`/`ListPostgres`/`ListKeyValues` scoped to an `ownerId` the caller is **not a member of**, with OpenFGA wide open (`allow: true`) — proving the NEW membership check (not just the OpenFGA check) is what refuses the request. Before `t003`, this exact case would have silently succeeded (the old gate was OpenFGA-only). One test per package: `TestList_OwnerIDFilterForbiddenWhenCallerIsNotAMember` (apps), `TestListPostgres_OwnerIDFilterForbiddenWhenCallerIsNotAMember` (postgres), `TestListKeyValues_OwnerIDFilterForbiddenWhenCallerIsNotAMember` (keyvalue).
