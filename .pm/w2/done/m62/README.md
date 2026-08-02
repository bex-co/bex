# w2 · m62 — Git-connected Blueprints: Render dashboard + API parity

**Worker:** worker2 **Goal:** turn bex Blueprints from a stored-manifest re-apply record into Render's actual product — a Git-connected instance (repo + branch + path) you can create from the dashboard, that auto-syncs on push, records sync history, and can be updated/disconnected — closing every gap between `dashboard.bex.co/blueprints` and `dashboard.render.com/blueprints`. **Status:** done

## Background (researched 2026-08-01)

bex today (w1/m24 + w2/m15 + w2/m41 + w7/m27): a `blueprints` row is auto-registered on stack deploy keyed `(tenant, repo, branch)`, status is always `'active'`, the manifest is a stored copy, sync re-applies the stored/POSTed YAML, and the dashboard is read-only list/detail + Validate + Sync. Render's Blueprint is the inverse: the **repo file is the source of truth**. Verified against the live Render dashboard + docs + OpenAPI (2026-08-01):

- **Blueprint object:** `id (exs-…) · name · status ∈ {created, paused, in_sync, syncing, error} · autoSync · repo · branch · path · lastSync · resources[] {id,name,type}` (`GET /v1/blueprints`, `GET /v1/blueprints/{id}`).
- **Verbs:** `PATCH /v1/blueprints/{id}` (`name`/`autoSync`/`path`), `DELETE /v1/blueprints/{id}` (disconnect: stops syncing, **resources remain**), `GET /v1/blueprints/{id}/syncs` (cursor/limit; sync = `id (exe-…) · commit.id · startedAt · completedAt · state ∈ {created, pending, running, error, success}`), `POST /v1/blueprints/validate` (bex has this one).
- **Dashboard create flow** (`/select-repo?type=blueprint`): pick a Git-connected repo or public repo URL → Render reads `render.yaml` at the configured path → name + branch form → change review (resources to be created) → prompt for `sync: false` env-var values → Deploy Blueprint → progress.
- **Auto-sync:** on by default; a push to the linked branch that modifies the blueprint file triggers a sync of added/changed resources. **Sync never deletes** — removed resources are left for manual deletion (so bex's "no sync-delete" is parity, not a gap).

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Model parity: `path`/`auto_sync`/`last_sync_at`/status lifecycle + `blueprint_syncs` table     | 45m | —          | — **DONE** |
| t002 | Git-sourced manifest fetch: read `bex.yml` at repo+branch+path (GitHub App token or public)    | 45m | —          | — **DONE** |
| t003 | Create-blueprint-instance verb: from repo+branch+path, with `sync:false` env-value supply      | 60m | t001, t002 | — **DONE** |
| t004 | Sync engine: recorded sync runs, status transitions, pull-from-repo manual sync, syncs reads   | 60m | t001, t002 | — **DONE** |
| t005 | Auto-sync on push: trigger blueprint sync from the git webhook intake when the file changes    | 45m | t004       | — **DONE** |
| t006 | `PATCH /v1/blueprints/{id}` (name/autoSync/path) + `DELETE` disconnect across REST/GraphQL/MCP | 45m | t001       | — **DONE** |
| t007 | `resources[]` (id/name/type) on blueprint by-id reads across REST/GraphQL/MCP                  | 30m | t001       | — **DONE** |
| t008 | Dashboard: New Blueprint Instance flow (repo picker → review plan → env prompts → deploy)      | 90m | t003       | — **DONE** |
| t009 | Dashboard: detail-page parity — sync history, managed resources, settings card, manual sync    | 60m | t004, t006, t007 | — **DONE** |
| t010 | Render parity — cross-surface consistency check vs render.com                                  | 30m | t005, t008, t009 | — **DONE** |
| t011 | Simplify — `/simplify` over the changed code                                                   | 30m | t010       | — **DONE** |
| t012 | Test coverage — meaningful tests for the shipped behavior                                      | 45m | t010       | — **DONE** |
| t013 | Closeout                                                                                       | 15m | t012       | — **DONE** |

## Definition of done

- A user can create a Blueprint instance from `dashboard.bex.co/blueprints` by picking a GitHub-connected repo (or public repo URL), see the parsed plan, supply `sync: false` env values, and deploy — landing on the new blueprint's detail page.
- A push to the linked branch that modifies the blueprint file triggers an automatic sync when `autoSync` is on; the sync appears in the blueprint's recorded history with commit id, state, and timestamps; `lastSync`/status (`in_sync`/`syncing`/`error`/`paused`) reflect reality on all surfaces.
- `GET/PATCH/DELETE /v1/blueprints/{id}` and `GET /v1/blueprints/{id}/syncs` behave per Render's OpenAPI (PATCH: name/autoSync/path; DELETE disconnects without touching resources), with GraphQL + MCP counterparts and cross-workspace deny guards; by-id reads include `resources[]`.
- The dashboard detail page shows sync history, managed resources (linked), a status badge with the Render lifecycle vocabulary, manual Sync that pulls the latest manifest from the repo, and a settings card (rename / autoSync toggle / path / disconnect with confirm).
- All suites green (`cd lego/backend && go test ./...`, dashboard `yarn test`, `make lint`); `docs/ADR018-render-parity.md` Blueprint row updated with the new evidence.

## Source + Goal linkage

- **Source:** user-directed research 2026-08-01 — live walk of `dashboard.render.com/blueprints` (+ `/select-repo?type=blueprint`), render.com docs (`infrastructure-as-code`, `blueprint-spec`), Render OpenAPI (`list/retrieve/update/disconnect-blueprint`, `list-blueprint-syncs`, `validate-blueprint`), vs. a full code map of `lego/backend/internal/apps/blueprint.go`, `internal/store/blueprints.go` (migration 0016), and `dashboard/src/features/blueprints/`.
- **Goal linkage:** Render parity (`docs/ADR018-render-parity.md` Blueprint row's remaining divergences) + pillar 4 deploy-from-git (`docs/ADR008-vision.md`); rides w2/m8–m9's GitHub App integration (ADR026) and the existing `/v1/webhooks/git` intake.
- **Expected outcome:** the bex `/blueprints` dashboard is functionally indistinguishable from Render's for the GitHub + public-repo case: create-from-repo, auto-sync on push, sync history, settings, disconnect. The parity ledger's Blueprint row gains dashboard-create + lifecycle evidence instead of the "stored-manifest re-apply" caveat.
- **Why now:** the Blueprint family's API verbs (m15/m41) and GitHub integration (m8/m9) are done and stable — this is the last Render dashboard top-level page whose core flow (create) is missing entirely in bex; every prerequisite mechanism (repo listing, installation tokens, push webhook, stack deploy, protected-env confirm) already exists, so the work is composition, not new substrate.
- **Render parity closing task included:** feature work spanning REST/GraphQL/MCP + dashboard.
- **Guardrails honored:** GitHub-only git provider (DO_NOT_DO: no GitLab/Bitbucket); PR preview environments stay excluded (DO_NOT_DO); sync-delete is a confirmed non-gap (Render never deletes on sync either).
