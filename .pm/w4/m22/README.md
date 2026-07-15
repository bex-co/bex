# w4 · m22 — Close the ADR032-flagged `internal/projects` authz + error-mapping gaps

**Worker:** worker4 **Goal:** `internal/projects` gets the same tenant-isolation test coverage and error-mapping correctness `internal/environments` already has, and the CLI-compat checklist's `environments <id>` row gets its unverified success path closed out. **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on             |
| ---- | ---------------------------------------------------------------------------------- | --- | ----------------------- |
| t001 | Pin cross-tenant-vs-nonexistent 403-before-404 with a regression test              | 30m | —                       |
| t002 | Bring `internal/projects` into the authz-guards-every-verb sweep                   | 30m | —                       |
| t003 | Fix `internal/projects`' `ErrProjectsUnavailable` REST/GraphQL/MCP 500→503 mapping | 40m | —                       |
| t004 | Live-verify the environments success path end to end, close out the checklist row  | 30m | t001, t002, t003        |
| t005 | Render parity: REST/GraphQL/MCP error-mapping consistency                          | 20m | t004                    |
| t006 | Simplify                                                                            | 20m | t005                    |
| t007 | Test coverage                                                                       | 20m | t005                    |
| t008 | Closeout                                                                            | 10m | t006, t007              |

## Definition of done

- `internal/projects.Service.Get` has a regression test proving a cross-tenant-existing project id returns 403 (`core.ErrForbidden`), not 404 — mirroring `apps.TestGetCrossTenantIsForbiddenNotNotFound` (`lego/backend/internal/apps/apps_test.go:246`). `internal/environments.Service.Get` gets the equivalent test (it has none today, only the blanket-deny `TestVerbsDenyWhenUnauthorized`).
- `internal/projects` verbs are covered by an authz-guards-every-verb test (either added to the existing reflection sweep or an equivalent explicit test), closing the gap `docs/ADR032-environments.md:9` names by package: _"`internal/projects` itself is not in that sweep, an existing gap this feature doesn't inherit."_
- A misconfigured (`BEX_CP_DB_URI` unset) projects REST/GraphQL/MCP call returns 503, not a bare 500 — closing the second gap `docs/ADR032-environments.md:25` flags: _"its REST fragment passes it straight to `core.WriteErr` — which doesn't recognize it, so it currently falls through to a bare 500."_
- `render environments <id> -o json` verified end to end against a **real, seeded** project (not just the already-confirmed unknown-id 404 path), and `docs/cli-compatibility-checklist.md`'s `environments <id>` row updated from ◐ with evidence for the success path.
- `cd lego/backend && go test ./...` green; new tests fail on the pre-fix code (proven by running them against a stashed pre-t003 diff, or by inspection of the removed 500 fallthrough).

## Source + Goal linkage

- **Source:** user request 2026-07-14 to implement the `render environments <unknown-project-id>` CLI-compat row "fully end to end"; investigating it (`docs/cli-compatibility-checklist.md:56`) found the row itself is already correctly resolved (RC1's 404-for-unknown-id fix is live and correct), but surfaced two real, ADR-documented gaps behind it in `internal/projects` — zero test files, and an error-mapping fallthrough — both explicitly flagged in `docs/ADR032-environments.md:9,25` as left for "whoever picks it up."
- **Goal linkage:** w4's multi-tenant-security mandate — the same cross-tenant-isolation-hardening class of work as `w4/m19` (duplicate-name leak) and `w4/m20` (read-verb audit gaps), applied to the one control-plane resource (`internal/projects`) that never got it.
- **Expected outcome:** `internal/projects` has real regression tests proving tenant isolation on `Get`, a documented-and-closed 500→503 gap, and the CLI-compat checklist's `environments <id>` row is fully green (both the known-404 and previously-unverified-success paths) instead of partially verified.
- **Why now:** the gap has sat undiscovered since `w1/m32` shipped `internal/environments` layered on top of untested `internal/projects`; ADR032 named it explicitly rather than silently accepting it, and this request is what actually goes and picks it up.
- **Render parity:** included — this milestone changes REST/GraphQL/MCP error-mapping behavior for `internal/projects` (500→503), a surface Render itself returns structured errors on; t005 compares against Render's documented behavior for an unavailable/misconfigured resource class.
