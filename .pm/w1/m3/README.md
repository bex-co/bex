# w9 · m3 — Managed Postgres rename: stable identity + mutable name

**Worker:** worker9 **Goal:** Make `render postgres update <database> --name <new-name>` work end to end by separating a managed Postgres resource's immutable identity from its mutable display name, without recreating or renaming the live Kubernetes/CNPG objects that hold its data; roll the compatible schema and API through production and all nine isolated `dev-*` environments. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on      |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | --------------- |
| t001 | Split Database CR identity from display name; register the `dpg-…` id kind and legacy fallback         | 45m | —               |
| t002 | Rewire Postgres create/read/reference paths around stable ids and workspace-scoped name uniqueness     | 45m | t001            |
| t003 | Implement REST partial-update rename, including dry-run, authorization, errors, and CLI verification   | 45m | t002            |
| t004 | Carry rename semantics through GraphQL, MCP, and the dashboard without changing resource URLs          | 45m | t003            |
| t005 | Build an idempotent, non-destructive legacy-Database name backfill and rollout verifier                | 40m | t004            |
| t006 | Upgrade and verify `dev-1` through `dev-9`, including official-CLI rename smoke tests                   | 45m | t005            |
| t007 | Roll out to production and prove existing CNPG data-plane identities stayed unchanged                  | 45m | t006            |
| t008 | Update ADR009/ADR020 and the CLI compatibility checklist with migration and live evidence              | 30m | t007            |
| t009 | Render parity — audit REST · GraphQL · MCP · dashboard · official CLI behavior after the identity split | 30m | t008            |
| t010 | Simplify — run `$simplify` over the identity, lookup, migration, and UI changes                         | 30m | t009            |
| t011 | Test coverage — harden rename, compatibility, no-recreation, and rollout regression coverage           | 45m | t009            |
| t012 | Closeout — verify the DoD, move tasks/milestone to `done/`, and sync the w9 roadmap                    | 15m | t010, t011      |

## Definition of done

New managed Postgres resources are created with an immutable, typed `dpg-<xid>` identity minted by `lego/backend/internal/id` and a separate mutable user-facing name. Existing Database CRs remain in place: compatibility reads preserve their current API identity, an idempotent backfill records their display name, and no migration deletes or recreates a Database, CNPG Cluster, PVC, credential Secret, pooler, route, backup prefix, export, or recovery reference. `PATCH /v1/postgres/{id}` with a changed `name` has Render's partial-update and dry-run semantics, rejects invalid/duplicate/cross-workspace requests with specific errors, returns the same id with the new name, and is exercised successfully by the pinned official Render CLI. REST, GraphQL, MCP, and the dashboard share the same core behavior; database detail URLs and project/environment memberships remain stable across a rename. The updated CRD/operator/API and legacy backfill are verified in `dev-1` through `dev-9` and in production, with before/after evidence that pre-existing Kubernetes UIDs and data-plane object names did not change. `docs/ADR009-postgresql-management.md`, `docs/ADR020-identifiers.md`, and `docs/cli-compatibility-checklist.md` describe the new contract and mark `postgres update --name` ✅. `make test`, backend `go test ./...`, dashboard `yarn typecheck && yarn lint && yarn test`, relevant shell checks, and the CLI compatibility verifier all pass.

## Source + Goal linkage

- **Source:** User request 2026-07-14: “rearchitect to support `postgres update --name` (rename) in `docs/cli-compatibility-checklist.md`” and “fix prod and all the `dev-*` environments as well”; checklist RC11 and the command-census rename row document the live official-CLI failure and its current name-as-id cause. This promotes the Postgres half of `.pm/w1/done/021.md`'s typed-datastore-id design question after the Service half was fixed and that inbox note closed; KeyValue remains explicitly outside this milestone.
- **Goal linkage:** Render compatibility across bex-api and the official Render CLI (`docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`, `docs/cli-compatibility-checklist.md`) plus ADR020's invariant that ids are stable opaque references and names are mutable. This removes a documented datastore exception without putting business logic in the operator or coupling the operator to the backend.
- **Expected outcome:** Users can rename a managed Postgres by CLI, API, agent, or dashboard while every stable reference and every byte of database data remains attached to the same underlying resource. New databases use Render-shaped `dpg-…` ids; legacy databases keep working through an explicit compatibility contract instead of a destructive metadata-name migration.
- **Why now:** The completed CLI audit has isolated the remaining failure as an architectural identity coupling, not a handler bug. Every additional name-keyed Database makes a later migration riskier, and the requested production/dev rollout needs the compatibility and no-recreation rules designed before another API patch lands. Render parity is included as t009 because the change is user-facing and spans REST, GraphQL, MCP, the dashboard, and the official CLI.
