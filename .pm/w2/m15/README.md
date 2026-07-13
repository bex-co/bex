# w2 · m15 — `/blueprints` verbs: validate · list · sync

**Worker:** worker2 **Goal:** `POST /v1/blueprints/validate` dry-runs a `bex.yml` and returns per-entry errors with no apply; `GET /v1/blueprints` lists known blueprint sources; `sync` re-applies idempotently — all three verbs mirrored on GraphQL and MCP, giving an agent a safe validate-before-apply primitive ahead of a `deploy` call. Closes the Blueprint row's remaining resource-verb gap left open by `w1/m24` (which covers the *deploy* verb only). **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design: `POST /v1/blueprints/validate` (dry-run, per-entry errors, no apply) + `GET /v1/blueprints` (list known sources) + `POST /v1/blueprints/{id}/sync` (re-apply) — mirrors Render's Blueprint resource verbs | 40m | —          |
| t002 | Implement `validate` by reusing `w1/m24`'s all-or-nothing validation path without the apply step                                        | 45m | t001       |
| t003 | Implement `list`/`sync` over the control-plane store (a blueprint is a repo+path source already tracked via git-connect)                 | 1h  | t001       |
| t004 | GraphQL + MCP mirrors of all three verbs — `validate_bex_yml` gives an agent a safe dry-run before it commits to a `deploy` call          | 45m | t002, t003 |
| t005 | Live verification: validate a bad `bex.yml` (per-entry errors, no partial apply); sync a valid one and confirm idempotent re-apply; confirm an agent can validate-then-deploy in two MCP calls with no surprise | 30m | t004       |
| t006 | Docs: close the "`/blueprints` resource stays untracked, low" note in `w1/m24`'s record and `docs/ADR018-render-parity.md`'s Blueprint row | 15m | t005       |

## Definition of done

`POST /v1/blueprints/validate` catches a bad `bex.yml` with per-entry errors and applies nothing; `GET /v1/blueprints` lists known sources; `sync` re-applies idempotently — all three verbs mirrored on GraphQL/MCP, with the MCP `validate` tool usable standalone (no side effects) as a pre-flight check.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w2` 2026-07-13 — re-homed from an earlier `w1`-targeted proposal (`/pm-brainstorm more milestones to work on`, 2026-07-13); `docs/ADR018-render-parity.md`'s Blueprint row ("`/blueprints` resource stays untracked, low") and `w1/m24`'s own note that the resource verbs (validate/list/sync) were left out of that milestone's scope.
- **Goal linkage:** pillar 4 (deploy-from-chat) — gives an agent a safe dry-run primitive before committing to a stack deploy; closes the Blueprint row's remaining resource-verb gap.
- **Expected outcome:** an agent can validate a `bex.yml` with zero side effects before deploying it, and `docs/ADR018-render-parity.md`'s Blueprint row records the resource verbs as shipped.
- **Why now:** the validate-before-apply shape is more naturally an agent-safety feature than platform infra, which is why it's here in `w2` rather than `w1`; `w2` is otherwise fully drained and ready for it.
- **Render parity closing task: included.**

## Depends on

`w1/m24` (Multi-service `bex.yml`: Blueprint-shaped stack deploys) — not yet materialized/done. Sequence this milestone's work after `w1/m24` lands; the `validate` verb (t002) directly reuses its all-or-nothing validation path.
