# w1 · m55 — Retire deprecated product compatibility surfaces

**Worker:** worker1 **Goal:** Match Render's current contracts and remove bex-only public aliases, stateful MCP workspace selection, old Blueprint dialects, browser migration residue, and proven-dead compatibility shims. **Status:** done (2026-07-27)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | MCP tools accept Render's explicit per-call workspaceId — **DONE** | 60m | — |
| t002 | Remove stateful MCP workspace selection, persistence, and deprecated tools — **DONE** | 45m | t001 |
| t003 | Remove bex-only public REST aliases while preserving the internal control-plane API — **DONE** | 60m | — |
| t004 | Remove the deprecated list_key_value_instances MCP alias — **DONE** | 30m | t001 |
| t005 | Migrate examples to the Render Blueprint dialect and remove legacy bex.yml parser branches — **DONE** | 60m | — |
| t006 | Remove dashboard workspace localStorage migration and dead compatibility helpers — **DONE** | 45m | — |
| t007 | Remove low-risk Go, script, and production-config compatibility shims — **DONE** | 45m | — |
| t008 | Refresh contract evidence, migration notes, and deprecation inventory — **DONE** | 30m | t002, t003, t004, t005, t006, t007 |
| t009 | Render parity — cross-surface consistency check — **DONE** | 30m | t008 |
| t010 | Simplify — run /simplify over the changed code — **DONE** | 20m | t009 |
| t011 | Test coverage — canonical contracts succeed and removed contracts fail closed — **DONE** | 45m | t009 |
| t012 | Closeout — verify DoD, mark done, move milestone — **DONE** | 15m | t010, t011 |

## Definition of done

The current official Render MCP schemas work with an explicit workspaceId on every workspace-scoped call, and no request depends on transport-session workspace state. The public bex-api registers only canonical Render REST routes and MCP tool names from this milestone's inventory; the internal control-plane /v1/apps route remains available only on its internal listener. Repository examples use services and the canonical Blueprint fields, and removed legacy Blueprint inputs fail with a clear validation error. Dashboard workspace selection is cookie-backed without localStorage migration code, the identified dead helpers are absent, and the full backend, operator, and dashboard suites are green. Documentation names every intentionally retained compatibility field so future sweeps do not delete valid Render contracts.

## Source + Goal linkage

**Verified 2026-07-27:** `go test ./...` in `lego/backend`; operator `make test` and `make lint`; dashboard typecheck, 265 test files / 1663 tests, and lint; every shipped `examples/*/bex.yml` through `DRY_RUN=1 scripts/app-apply.sh`; `scripts/cli-compat.sh services-parity-self-test`; changed-shell-script syntax; manager kustomize; and `git diff --check`. Changed dashboard files pass Prettier. The repository-wide dashboard Prettier baseline still reports unrelated pre-existing formatting drift and is not part of this milestone's DoD.

- **Source:** User-requested read-only deprecated-code audit on 2026-07-27; Render's 2026-07-27 MCP change made workspaceId explicit and marked select_workspace plus implicit session fallback for removal. The audit also traced bex-only REST aliases, the old apps-root Blueprint dialect, dashboard localStorage migration residue, and dead compatibility helpers to concrete call sites.
- **Goal linkage:** GOAL.md #1 and ADR008's Render-compatible, AI-native surface: canonical APIs reduce behavioral drift and make agents and users portable between Render and bex.
- **Expected outcome:** one canonical contract per product operation, a smaller public attack and maintenance surface, no session-affinity requirement for workspace-scoped MCP calls, and no repository-owned examples teaching obsolete bex syntax.
- **Why now:** the upstream MCP contract changed today; bex's session-selection subsystem now moves opposite to official Render, while bex-only REST aliases bypass the shared Render OpenAPI validation layer. Removing the paths together gives one documented breaking-change boundary.
- **Render parity task included:** yes — this milestone changes REST, MCP, Blueprint, and dashboard behavior and must verify every retained surface against current Render evidence.
