# w6 · m17 — Fix false-403: collapse the caller-default-workspace vs. resource-workspace authorization gates

**Worker:** worker6 **Goal:** A resource-scoped verb authorizes against the resource's own workspace, not the caller's default workspace — so a user whose default workspace is one they were invited into with a limited role (e.g. `viewer`) can still restart/suspend/delete/set-env-vars on services in a *different* workspace where they hold `admin`. Today the two gates (`Authorize` against the caller's default workspace, `GetApp`/`fetchDatabase`/`fetchKeyValue` against the resource's actual workspace, fixed by `w6/m14`) are independent, so effective permission is their intersection — reproduced live, not theorized. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                                                     | est  | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Implement `core.Base.AuthorizeApp(ctx, relation, name) (*App, error)`: fetch the resource, derive its workspace from `core.LabelTenant`, authorize `relation` there once, audit there once (target = resource name) — preserving 403-before-404 (authorize against the caller's acting workspace first when the fetch would 404) | 1h   | —          |
| t002 | Migrate the ~45 call sites currently doing separate `Authorize` + `GetApp`/`fetchDatabase`/`fetchKeyValue` calls onto the new single seam                                                                | 1.5h | t001       |
| t003 | Fold `List(ownerID)`'s weaker OpenFGA-only gate (`apps/service.go`, `postgres/service.go`, `keyvalue/service.go` — no `IsMember` check today) onto `core.WithWorkspace`'s correct membership+OpenFGA mechanism | 40m  | t001       |
| t004 | Regression tests: `TestAuthzGuardsEveryVerb` and `TestEveryTargetedVerbIsNamedOrExcused` stay green; add the reproduction case from `w6/013` (an invited-viewer-by-default user, admin of their own separate workspace, restarting a service there) as a permanent regression test | 45m  | t002, t003 |
| t005 | Live/functional verification: the exact `bob`/`tea-team`/`tea-mine` scenario from `w6/013` now succeeds end-to-end                                                                                       | 30m  | t004       |
| t006 | Docs: note the fix in `docs/ADR012-auth.md` (or the relevant authz section); close `w6/013` (`w6/012` stays open, separate scope)                                                                        | 20m  | t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                                                                                                              | 20m  | t006       |
| t008 | Test coverage — final gap sweep (`AuthorizeApp`'s 403-before-404 fallback path, `List(ownerID)`'s membership-checked behavior)                                                                           | 20m  | t006       |
| t009 | Closeout — verify DoD met, then move the milestone to `done/`                                                                                                                                            | 10m  | t007, t008 |

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
