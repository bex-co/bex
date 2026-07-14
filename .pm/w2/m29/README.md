# w2 · m29 — Dry-run/preview mode for service, Postgres, and Key Value create/update

**Worker:** worker2 **Goal:** An agent can pass `dryRun`/`dry_run` on a create/update call for a service, managed Postgres, or Key Value across REST/GraphQL/MCP and get back the resolved spec as a preview, with zero Kubernetes/store side effects — the same safety net `validate_bex_yml` (`w2/m15`) already proved for the `bex.yml` path, extended to the more common direct-create path. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on           |
| ---- | ------------------------------------------------------------------------------------------------ | --- | --------------------- |
| t001 | Core: `DryRun` plumbing in `internal/core` (mirror `blueprints.Validate`'s short-circuit-before-apply pattern) | 45m | —                      |
| t002 | REST: `dryRun` query/body param on service/Postgres/Key-Value create + update endpoints → preview envelope, zero writes | 45m | t001                   |
| t003 | GraphQL: matching `dryRun: Boolean` arg on the equivalent mutations                             | 35m | t001                   |
| t004 | MCP: `dry_run` arg on `create_web_service`/`create_postgres`/`create_key_value`/`update_service_plan` etc., same preview shape as `validate_bex_yml` | 35m | t001                   |
| t005 | Regression test: dry-run makes zero Kubernetes/store side effects across all three resource kinds, on all three surfaces | 35m | t002, t003, t004       |
| t006 | Render parity — cross-bex-surface consistency check (REST/GraphQL/MCP agree; no Render equivalent exists) | 20m | t005                   |
| t007 | Simplify — `/simplify` over the code this milestone changed                                    | 20m | t006                   |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped                       | 30m | t006                   |
| t009 | Closeout — DoD met → move milestone to `done/`                                                 | 10m | t008                   |

## Definition of done

An agent can pass `dryRun`/`dry_run` on a create or update call for a service, a managed Postgres instance, or a Key Value instance — across REST, GraphQL, and MCP — and get back the resolved spec/preview instead of a live create, with **zero** Kubernetes or control-plane-store side effects. Proven by a test asserting identical resource counts (Apps/Databases/KeyValues, and their k8s-side CRs) before and after a dry-run call, for all three resource kinds on all three surfaces.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on for each everyone of workers` 2026-07-13. `internal/blueprints`' `validate_bex_yml` dry-run (`w2/m15`) already proved this pattern's value for the `bex.yml` path; most agent-driven deploys go through direct `create_web_service`/`create_postgres`/`create_key_value` calls instead, which have no preview safety net today.
- **Goal linkage:** `docs/ADR008-vision.md` pillars 3–4 (AI-native surface) — agents are more failure-prone than humans at "did this actually deploy" mistakes; this extends the preview idiom already proven valuable to the more common direct-create path.
- **Expected outcome:** an agent can validate its own create/update inputs before committing, catching a bad spec (wrong plan tier, malformed env var, etc.) with no live side effect; a new reusable `dryRun` idiom other future write verbs can adopt.
- **Why now:** the pattern and its value are already proven and shipped once (`w2/m15`); this is the same idiom applied to the larger, more commonly used surface.
- **Render parity closing task: included, scoped to cross-bex-surface consistency** — Render has no `dryRun` concept on any surface (checked its OpenAPI spec: no such param anywhere), so this is a bex-ahead-of-Render superset, not a parity gap; the closing task confirms REST/GraphQL/MCP agree with each other rather than comparing against a Render equivalent that doesn't exist. No dashboard UI is added — this is an agent-ergonomics feature; a human create-flow already sees the real result immediately, so a preview control has no dashboard use case.
