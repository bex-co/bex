# w1 · m2.5 — Refactor bex-api into feature packages (one package per feature)

**Worker:** worker1 **Goal:** Reorganize `lego/backend/internal/api` from a flat ~20-file grab-bag into **one Go package per feature** (`apps`, `logs`, `metrics`, `apikeys`, `postgres`, `authz`), each holding its own **service** (business logic) + **models** + **REST/GraphQL/MCP registration fragments** — while the three transports stay **single artifacts** (one GraphQL schema, one REST router, one MCP registry, one auth gate) composed in a thin root. Behavior-preserving; keeps the load-bearing "one Core, three adapters, Render-consistent" invariant enforceable as features grow. **Status:** done (shipped in `06f247e` 2026-07-06; verified + board synced 2026-07-08)

> **Shape decided = "A" (architecture review, 2026-07-06).** A feature = **one package**, with the layers as _files_ inside it (`service.go` / `models.go` / `rest.go` / `graphql.go` / `mcp.go`) — **not** per-feature `api/`+`data/`+`service/` sub-packages. Rationale (Go-idiomatic): the package is the unit of encapsulation and the unit of cycle-prevention, so one package per feature lets a feature's files share unexported helpers, avoids forcing types public across an `api→service→data` chain, and avoids a `shared/` dumping ground for `authorize()`/error sentinels. Matches the Go community's "package-by-feature" consensus and Ben Johnson's Standard Package Layout (domain types + dependency-adapter subpackages), not the Java/Clean-Architecture "layer-per-package" split.

> **Done (verified 2026-07-08).** Shipped in commit `06f247e` (2026-07-06). DoD verified against the tree: flat `internal/api` grab-bag gone — feature packages `internal/{apps,logs,metrics,apikeys,postgres}` over leaf `internal/core` + `internal/authz` (+ `internal/gqlutil`), with `internal/api` reduced to the thin composition root (`server.go`/`auth.go`); transports remain single artifacts; the parity guard exists as `TestSurfaceParityAndWiring` (+ `TestAuthzGuardsEveryVerb` reflective sweep) in `internal/api/server_test.go`; backend + `internal/api` CLAUDE.md updated; `go build ./... && go test ./...` green. Board metadata was stale (tasks `todo`), fixed during this reconciliation.

> **The invariant this refactor must not break.** bex-api is "one Core, three thin adapters (REST/GraphQL/MCP) that can't drift" ([internal/api/CLAUDE.md](../../../lego/backend/internal/api/CLAUDE.md), [docs/bex-api.md](../../../docs/bex-api.md)). The three transports are **single artifacts** mounted once (`server.go`: one `/v1/` router, one `/graphql` code-first `newSchema()`, one `/mcp` registry, one auth gate). Each feature's `rest.go`/`graphql.go`/`mcp.go` are **registration fragments** that _contribute_ to those shared roots (e.g. `graphql.Fields` merged into the root Query/Mutation), **never** standalone servers. The composition root owns the roots; features plug in.

## Tasks (in order)

| id   | title                                                                                        | est | depends_on             |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Leaf kernel + thin composition root: `internal/core`, `internal/authz`, registration contract — **DONE** | 40m | —                      |
| t002 | Extract `internal/apps` — read side (List/Get + AppView + shared fetch/patch) + package skeleton — **DONE** | 25m | t001                   |
| t003 | Extract `internal/apps` — write side (Restart/Suspend/Resume + single-writer wiring) — **DONE**  | 25m | t002                   |
| t004 | Extract `internal/logs` (Logs/QueryLogs/FollowLogs + PodLogSource/Stream + fragments) — **DONE** | 30m | t001                   |
| t005 | Extract `internal/metrics` (metrics + 4 sources + fragments + tests) — **DONE**                  | 40m | t001                   |
| t006 | Extract `internal/apikeys` + `internal/postgres` (the two smaller features + fragments) — **DONE** | 30m | t001                   |
| t007 | Finalize: delete the flat `internal/api`, rewire `cmd/api`, update the 3 CLAUDE.md, green — **DONE** | 30m | t003, t004, t005, t006 |
| t008 | Simplify — `/simplify` over the refactored packages — **DONE**                                   | 20m | t007                   |
| t009 | Test coverage — three-surface parity + registration-wiring tests — **DONE** (`TestSurfaceParityAndWiring`) | 30m | t007                   |

## Definition of done

- `cd lego/backend && go build ./... && go test ./...` is green; `make docker-build` and `kustomize build config/api` are unaffected (byte-for-byte behavior, no wire changes).
- The flat `internal/api` grab-bag is gone: business logic + models + transport fragments live in **one package per feature** (`internal/apps`, `internal/logs`, `internal/metrics`, `internal/apikeys`, `internal/postgres`), with the shared gate/base in a **leaf** `internal/core` (+ `internal/authz`) that features import, and a **thin composition root** (`internal/server` or a slimmed `internal/api`) that features do _not_ import (no import cycle).
- The three transports remain **single artifacts**: one GraphQL schema, one REST router, one MCP registry, one auth gate — assembled from per-feature registration fragments in the root.
- A **parity test** asserts every verb reachable over REST is reachable over GraphQL and MCP after recomposition (guards the invariant the refactor exists to preserve).
- The three CLAUDE.md files (`lego/backend`, `internal/api`→new layout, root repo map) describe the feature-package layout and the "fragments register into single roots" rule.

## Source + Goal linkage

- **Source:** this `/pm` architecture discussion (2026-07-06); shape "A" chosen after researching Go layout idioms (package-by-feature consensus; Ben Johnson _Standard Package Layout_; the "too many small packages" anti-pattern). Supersedes the user's first-cut `features/<x>/{api,data,service}` triple, rejected as against Go's grain.
- **Goal linkage:** protects the bex-api product guarantee — one Core + three non-drifting, Render-consistent adapters ([docs/bex-api.md](../../../docs/bex-api.md), [internal/api/CLAUDE.md](../../../lego/backend/internal/api/CLAUDE.md)). Sustains the backend as the control-plane source of truth (m2) and the base that later API-surface milestones land on (m4 scale-to-zero, m5 build-from-git).
- **Expected outcome:** `internal/api`'s flat ~20 files become cohesive feature packages; each feature is discoverable and deletable in one place; the drift-guard invariant is testable, not just documented; no behavior/wire change.
- **Why now:** m2 just dropped a large `internal/store` + core/authz wiring into the backend (currently **uncommitted**); m3–m7 will pile more API surface on. Refactor **after m2 commits** (hence "m2.5") and **before** the grab-bag deepens — the surface is understood now and the restructuring cost only grows with each feature added.
