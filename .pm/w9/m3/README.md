# w9 · m3 — Managed Postgres rename: stable identity + mutable name

**Worker:** worker9 **Goal:** Make `render postgres update <database> --name <new-name>` work end to end by separating a managed Postgres resource's immutable identity from its mutable display name, without recreating or renaming the live Kubernetes/CNPG objects that hold its data; roll the compatible schema and API through production and all ten isolated `dev-*` environments. **Status:** todo (t001–t006 done; t007 next)

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on      | status    |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | --------------- | --------- |
| t001 | Split Database CR identity from display name; register the `dpg-…` id kind and legacy fallback         | 45m | —               | — **DONE** |
| t002 | Rewire Postgres create/read/reference paths around stable ids and workspace-scoped name uniqueness     | 45m | t001            | — **DONE** |
| t003 | Implement REST partial-update rename, including dry-run, authorization, errors, and CLI verification   | 45m | t002            | — **DONE** |
| t004 | Carry rename semantics through GraphQL, MCP, and the dashboard without changing resource URLs          | 45m | t003            | — **DONE** |
| t005 | Build an idempotent, non-destructive legacy-Database name backfill and rollout verifier                | 40m | t004            | — **DONE** |
| t006 | Upgrade and verify `dev-1` through `dev-10`, including official-CLI rename smoke tests                  | 45m | t005            | — **DONE** |
| t007 | Roll out to production and prove existing CNPG data-plane identities stayed unchanged                  | 45m | t006            | —         |
| t008 | Update ADR009/ADR020 and the CLI compatibility checklist with migration and live evidence              | 30m | t007            | —         |
| t009 | Render parity — audit REST · GraphQL · MCP · dashboard · official CLI behavior after the identity split | 30m | t008            | —         |
| t010 | Simplify — run `$simplify` over the identity, lookup, migration, and UI changes                         | 30m | t009            | —         |
| t011 | Test coverage — harden rename, compatibility, no-recreation, and rollout regression coverage           | 45m | t009            | —         |
| t012 | Closeout — verify the DoD, move tasks/milestone to `done/`, and sync the w9 roadmap                    | 15m | t010, t011      | —         |

## Definition of done

New managed Postgres resources are created with an immutable, typed `dpg-<xid>` identity minted by `lego/backend/internal/id` and a separate mutable user-facing name. Existing Database CRs remain in place: compatibility reads preserve their current API identity, an idempotent backfill records their display name, and no migration deletes or recreates a Database, CNPG Cluster, PVC, credential Secret, pooler, route, backup prefix, export, or recovery reference. `PATCH /v1/postgres/{id}` with a changed `name` has Render's partial-update and dry-run semantics, rejects invalid/duplicate/cross-workspace requests with specific errors, returns the same id with the new name, and is exercised successfully by the pinned official Render CLI. REST, GraphQL, MCP, and the dashboard share the same core behavior; database detail URLs and project/environment memberships remain stable across a rename. The updated CRD/operator/API and legacy backfill are verified in `dev-1` through `dev-10` and in production, with before/after evidence that pre-existing Kubernetes UIDs and data-plane object names did not change. `docs/ADR009-postgresql-management.md`, `docs/ADR020-identifiers.md`, and `docs/cli-compatibility-checklist.md` describe the new contract and mark `postgres update --name` ✅. `make test`, backend `go test ./...`, dashboard `yarn typecheck && yarn lint && yarn test`, relevant shell checks, and the CLI compatibility verifier all pass.

## Verification (2026-07-15)

- The unmodified pinned Render CLI completed create → resolve old name → `postgres update --name` → resolve new name → delete in `dev-1` through `dev-10`. Every run returned one stable `dpg-…` id. The hardened reruns in `dev-7` and the newly added `dev-10` required and compared a Database, CNPG Cluster, PVC, credential Secret, and CNPG `-rw` Service before accepting the result.
- `scripts/postgres-name-migrate.test.sh` exercises dry-run, spec-name-only apply, second-run idempotence, cross-workspace same-name acceptance, same-workspace duplicate rejection, invalid legacy-name rejection, and no unrelated spec-value output without a cluster. The live all-namespace dry-run reports the local cluster already complete.
- The shared mock cluster ran the generated CRD with `spec.name` and the current operator with all three controllers started. `dev-5` and the rebuilt `dev-7` finished with their auth pods Ready and their pre-existing `srv-d9bhpuq9086r22a6406g` projected service Running.
- Operational incident: starting fresh control-plane projectors during `dev-5`/`dev-7` verification pruned that pre-existing projected service CR in both namespaces. It was restored from the surviving control-plane row with the same service id and desired spec, and the missing `dev-7` control-plane row/deploy history was restored before a live projector resync. Kubernetes assigned new App UIDs; no Database/CNPG/PVC/Secret identity in the Postgres rename smoke changed.
- Full backend `go test ./...`, operator `make test`, dashboard `yarn typecheck && yarn lint && yarn test` (1,063 tests), `make lint-backend` (0 issues), shell syntax checks, Markdown formatting, and `git diff --check` pass. Full `make lint` still reports the repository's pre-existing untouched operator warnings.
- Production is deliberately not claimed: no commit, push, or production mutation is authorized without `$ship`. t007 remains the next task; the checklist stays ◐ until its production migration and identity comparison pass.

## Source + Goal linkage

- **Source:** User request 2026-07-14: “rearchitect to support `postgres update --name` (rename) in `docs/cli-compatibility-checklist.md`” and “fix prod and all the `dev-*` environments as well”; checklist RC11 and the command-census rename row document the live official-CLI failure and its current name-as-id cause. This promotes the Postgres half of `.pm/w1/done/021.md`'s typed-datastore-id design question after the Service half was fixed and that inbox note closed; KeyValue remains explicitly outside this milestone.
- **Goal linkage:** Render compatibility across bex-api and the official Render CLI (`docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`, `docs/cli-compatibility-checklist.md`) plus ADR020's invariant that ids are stable opaque references and names are mutable. This removes a documented datastore exception without putting business logic in the operator or coupling the operator to the backend.
- **Expected outcome:** Users can rename a managed Postgres by CLI, API, agent, or dashboard while every stable reference and every byte of database data remains attached to the same underlying resource. New databases use Render-shaped `dpg-…` ids; legacy databases keep working through an explicit compatibility contract instead of a destructive metadata-name migration.
- **Why now:** The completed CLI audit has isolated the remaining failure as an architectural identity coupling, not a handler bug. Every additional name-keyed Database makes a later migration riskier, and the requested production/dev rollout needs the compatibility and no-recreation rules designed before another API patch lands. Render parity is included as t009 because the change is user-facing and spans REST, GraphQL, MCP, the dashboard, and the official CLI.
