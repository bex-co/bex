# w8 · m21 — Blueprint dashboard completion: sync:false prompts, pre-sync diff, settings edit

**Worker:** worker8 **Goal:** the dashboard stops trailing its own blueprint API — `blueprints/new` prompts for `sync: false` secret values at creation (Render behavior) instead of silently deploying them empty, the detail page shows a plan/diff before manual sync instead of applying blindly, and name/path become editable in settings (the PATCH verb already supports both). **Status:** done

## Tasks (in order)

| id   | title                                                                     | est | depends_on       |
| ---- | ------------------------------------------------------------------------- | --- | ---------------- |
| t001 | `sync: false` prompt step on blueprints/new + `envVarValues` wiring — **DONE**       | 45m | —                |
| t002 | Pre-sync plan/diff on the blueprint detail page (reuse `PreviewBlueprint`) — **DONE** | 45m | —                |
| t003 | Editable name + path in blueprint detail settings — **DONE**                         | 30m | —                |
| t004 | Render parity check (create-prompt/sync-review flows vs render.com; envVarValues across surfaces) — **DONE** | 30m | t001, t002, t003 |
| t005 | Simplify (`/simplify` over the changed code) — **DONE**                              | 30m | t004             |
| t006 | Test coverage (route tests: prompt step, preview dialog, settings PATCH) — **DONE**  | 45m | t004             |
| t007 | Closeout — **DONE**                                                                  | 15m | t006             |

## Definition of done

Creating a blueprint whose manifest carries `sync: false` env vars from the dashboard shows one input per prompt key on the review step (secret-masked), sends the supplied values as `envVarValues` on `createBlueprint`, and the deployed service has them set (seed-once — a later sync never overwrites). Pressing Sync on the detail page first shows the computed plan (create/update groups + estimated pricing + validation errors) and applies only on confirm. Name and path are editable on the detail page via the existing PATCH; branch stays read-only (identity key). All three verified by dashboard route tests plus one live dev-stack walkthrough.

## Source + Goal linkage

- **Source:** blueprint lifecycle-semantics verification 2026-08-16 (follow-up to the 2026-08-15 parity review, scoped to what remains after m19/m20): the backend `sync: false` channel is complete — prompt keys classified (`deploy.go`), seed-once honored (`secrets/service.go:406`), `envVarValues` accepted on REST `POST /v1/blueprints` and GraphQL `createBlueprint` — but the dashboard's `CreateBlueprintDocument` (`dashboard/src/features/blueprints/api/operations.ts:187-227`) has no `envVarValues` variable and `blueprints.new.tsx` never renders the `syncFalseVars` the validation plan already returns, so dashboard-created blueprints deploy secret placeholders empty. The detail page's Sync posts directly with only the protected-resource dialog (`blueprints.$blueprintId.tsx:107-118`); `useBlueprintPreview` is create-page-only. Branch/path render read-only despite PATCH support. (Code evidence predates m19/m20 landing — re-verify line numbers at task start.)
- **Goal linkage:** Render parity (ADR018 Blueprint row — Render prompts for sync:false values at first creation and shows a resource diff) + finishing the w7/m27→m18→m19 dashboard blueprint surface.
- **Expected outcome:** the dashboard blueprint flow is self-sufficient — no post-create trip to the env editor to un-break a deploy, no blind syncs.
- **Why now:** m19/m20 just landed in the same files; the sync:false gap actively breaks first deploys of any real-world manifest with secrets (the exact manifests m19's promotions now let in). Render parity task included: feature work on the dashboard + a GraphQL/REST-consumed payload.
- **DO_NOT_DO constraints honored:** no new backend surface — this consumes existing verbs; no preview environments or other excluded features touched.
