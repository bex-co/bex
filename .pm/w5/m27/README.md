# w5 · m27 — Blueprints dashboard surface (list · manifest · validate · sync)

**Worker:** worker5 **Goal:** A user can see the workspace's registered blueprints (auto-registered on every repo-backed `deploy`), inspect each manifest, validate it (dry-run), and trigger an idempotent sync — all from the dashboard. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Data layer: `features/blueprints/` hooks over `blueprints` (list) + `syncBlueprint` + `validateBlueprint`                             | 35m | —          |
| t002 | Blueprints list page + route (`/blueprints`) + sidebar nav entry: rows (name/repo/branch/status), empty state explaining auto-register | 45m | t001       |
| t003 | Blueprint detail: read-only manifest (`bex.yml`) view + "Sync" action with confirm + last-sync status                                | 40m | t001       |
| t004 | Validate flow: a "Validate" affordance surfacing per-entry errors from `validateBlueprint` (dry-run, no apply)                       | 35m | t002       |
| t005 | Empty/loading/error states + `en`/`zh` locale strings                                                                                | 30m | t002, t003 |
| t006 | Live verification against the mock cluster: deploy a repo-backed `bex.yml` → appears in list → view manifest → sync → validate       | 30m | t004, t005 |
| t007 | Render parity — full-surface check (REST/GraphQL/MCP already ship validate/list/sync; confirm the new UI matches; NOT the DO_NOT_DO picker) | 20m | t006       |
| t008 | Simplify — `/simplify` over the code this milestone changed                                                                          | 20m | t007       |
| t009 | Test coverage — meaningful tests for the behavior this milestone shipped                                                             | 30m | t007       |
| t010 | Closeout — DoD met → move milestone to `done/`                                                                                       | 10m | t009       |

## Definition of done

A user can list registered blueprints, inspect each blueprint's manifest, validate it (dry-run with per-entry errors, no apply), and trigger an idempotent sync from the dashboard; verified against the mock cluster.

## Source + Goal linkage

- **Source:** `docs/ADR018-render-parity.md` "Blueprint / render.yaml IaC" row (UI ✖: "no Blueprint dashboard surface"). Backend verbs shipped `w2/m15` (`validate_bex_yml`/`list_blueprints`/`sync_blueprint`; GraphQL `validateBlueprint`/`blueprints`/`syncBlueprint`; all three adapters). Proposed via `/pm-brainstorm more tasks for w5 to achieve feature parity` 2026-07-13; user confirmed inclusion same day.
- **⚠️ DO_NOT_DO clarification:** this is **not** the rejected "link-database-to-service picker" (DO_NOT_DO §"Do not build a 'Link database to service' picker" — an env-var insertion flow) that the ledger's parenthetical loosely cross-references. This is a **read / validate / sync management surface** over already-shipped verbs — no blueprint _authoring_ picker, no DB→service env injection.
- **Goal linkage:** closes the last ✖ UI cell for an already-shipped REST/GraphQL/MCP capability in the parity ledger.
- **Expected outcome:** blueprint stacks deployed via chat/API become visible and re-syncable from the dashboard.
- **Why now:** low-priority but genuine; the git-integration + blueprint-verb prerequisites are all done. **Sequence after m26** (Env Groups is the higher-value gap).
- **Render parity closing task: included** — the milestone adds a dashboard UI over an existing REST/GraphQL/MCP contract (`w2/m15` blueprint verbs).
