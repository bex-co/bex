# w1 · m32 — Environments: named subsets of a Project's services

**Worker:** worker1 **Goal:** A Project's services (w1/m31) can be further grouped into named Environments (e.g. staging/production), readable and filterable over REST, GraphQL, and MCP; assigning a service to an environment also joins it to the environment's project. **Status:** done (2026-07-13)

## Tasks (in order)

| id   | title                                                                                                             | est | depends_on | |
| ---- | ------------------------------------------------------------------------------------------------------------------- | --- | ---------- |---|
| t001 | Store: `environments` table + nullable `environment_id` FK on `apps` (mirrors `0014_projects.up.sql`'s shape), migration `0019_environments` | 45m | —          | — **DONE** |
| t002 | Core verbs: `internal/environments` — Create/Rename/Delete/List/Get/SetServices, mirroring `internal/projects`' shape | 1h  | t001       | — **DONE** |
| t003 | REST/GraphQL: CRUD surface mirroring `internal/projects`' own REST/GraphQL shape                                    | 1h  | t002       | — **DONE** |
| t004 | MCP: full CRUD tools (list/get/create/rename/delete/set_environment_services), matching Projects' already-full MCP surface | 30m | t002       | — **DONE** |
| t005 | Wire into the composition root (`internal/api/server.go`, `cmd/api/main.go`) + the authz reflection sweep (`sweepableServices`) | 20m | t003, t004 | — **DONE** |
| t006 | Live verification: group three real services into a project with two environments on the CAPD mock cluster, verify REST/GraphQL/MCP agree, verify the auto-join-to-project behavior and the store-less 503 case | 45m | t005       | — **DONE** |
| t007 | Docs: `docs/ADR032-environments.md` + update `docs/ADR018-render-parity.md`'s Projects & environments row              | 20m | t006       | — **DONE** |
| t008 | Render parity                                                                                                        | 25m | t007       | — **DONE** |
| t009 | Simplify                                                                                                             | 15m | t008       | — **DONE** |
| t010 | Test coverage                                                                                                        | 35m | t008       | — **DONE** |
| t011 | Closeout                                                                                                             | 10m | t009, t010 | — **DONE** |

## Definition of done

A workspace's Project can hold named Environments; assigning a service to an environment groups it under both the environment and its project, readable and filterable via `?projectId=`/environment-scoped reads on REST, GraphQL, and MCP — verified live against a real cluster, with a real Postgres control-plane store, across all three surfaces plus the store-less 503 case.

## Source + Goal linkage

- **Source:** `w6/m16` (2026-07-13, `/pm-brainstorm more milestones to work on`) was independently implemented as full Projects+Environments in one session, but was discarded on `/ship` after a rebase conflict revealed `w1/m31` had concurrently shipped the SAME grouping feature — narrower in scope: Projects only, with Environments explicitly deferred as "a much larger architectural change" (`w1/m31`'s own README). This milestone rebuilds just the missing Environments half, composing with (not duplicating) `w1/m31`'s already-shipped Projects code, per user direction after the collision was surfaced.
- **Goal linkage:** pillar 1 (Render parity) — closes the Environments half of `docs/ADR018-render-parity.md`'s "Projects & environments (grouping)" row, which `w1/m31` left at ✖ across the board (row was also never updated for the Projects work that DID ship — fixed here too).
- **Expected outcome:** the parity row moves to ✅ REST/GraphQL/MCP (dashboard UX stays a deliberate follow-on, matching Projects' own scope boundary).
- **Why now:** the gap was discovered mid-session while reconciling the `w6/m16`/`w1/m31` collision; every prerequisite (Projects' store shape, id kind registry, composition root) already exists and the code was already built once (informing this rebuild), so closing it now is low-risk and avoids a second collision with a future independent attempt.
- **Render parity closing task: included** (t008) — the milestone adds a REST/GraphQL/MCP surface. **Dashboard UX is explicitly out of scope**, matching `w1/m31`'s own drawn boundary — a future follow-on, not this milestone's.

## Verification notes (t008–t011)

- **Render parity (t008):** `docs/ADR018-render-parity.md`'s "Projects & environments (grouping)" row updated to ✅ REST/GraphQL/MCP, ✖ UI; the gap-backlog line split so Projects & environments no longer reads as "untracked (low)" alongside unrelated items. Verified field-for-field against `internal/projects`' own already-shipped shape (the two features now read as one consistent system) rather than against Render's live API directly (Render's own environments object — `protectedStatus`/`networkIsolationEnabled`/`ipAllowList` — is explicitly out of scope per `w1/m31`'s own precedent).
- **Simplify (t009):** ran inline during implementation — extracted a shared `writeErr` REST helper (fixing a real 500-vs-503 gap `ErrEnvironmentsUnavailable` would otherwise have inherited from `projects.ErrProjectsUnavailable`'s equivalent, unfixed, gap), matched the `requireProject`/`requireEnvironment` two-gate authorize pattern to the codebase's `GetApp`-established convention rather than copying `internal/projects`' own (unswept) `AuthorizeOn`-only shape, and added `environments.Service` to `internal/api/server_test.go`'s `sweepableServices` (a real coverage gain — `internal/projects` itself was never added to that sweep, an existing gap this milestone does not inherit for its own feature).
- **Test coverage (t010):** `internal/environments/environments_test.go` (9 unit tests — CRUD lifecycle, duplicate-name conflict, unknown-project 404, deny-checker/unauthorized, store-unavailable, the REST 503-mapping fix specifically) + `internal/store/store_pg_test.go`'s new `assertProjectsAndEnvironments` (real-Postgres integration: environment creation under a project, service assignment incl. the auto-project-join, reassignment, rename, delete-cascade, and the defensive `project_id`+`environment_id` co-filter in `ListEnvironmentServices`) — run against a real throwaway Postgres, not just the in-memory fake.
- **Closeout (t011):** DoD verified live via `scripts/environments-verify.sh` against the CAPD mock cluster (real k8s App CRD + in-cluster operator, throwaway control-plane Postgres, real Hydra OAuth2 token) — REST, GraphQL, and MCP (a real streamable-HTTP client session) all agree on: two environments under one project, service assignment split across them, the auto-join-to-project side effect, and the corrected store-less 503. Full `go test ./...` (unit + real-Postgres integration) green at close.
