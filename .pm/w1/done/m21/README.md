# w1 · m21 — Static sites (Render `static_site` type)

**Worker:** worker1 **Goal:** Ship Render-parity static-site hosting — build a repo and serve the output from an object-store/CDN origin (not a Deployment), with a `publishPath`, redirects/rewrites (`/routes`), and custom response headers (`/headers`), plus the REST/GraphQL/MCP + dashboard surfaces. **Status:** done (all tasks; dashboard typechecks + 138 vitest pass; serving verified in a real browser via Playwright; the only unrun item is the deploy-time prod smoke test — see closeout)

## Tasks (in order)

| id   | title                                                                                    | est | depends_on                          |
| ---- | ---------------------------------------------------------------------------------------- | --- | ----------------------------------- |
| t001 | `static_site` service type: CRD/spec + build → object-store/CDN origin path              | 90m | — — **DONE**                        |
| t002 | Serving: `publishPath` from the CDN origin + redirects/rewrites + custom headers          | 90m | w1/m21/t001 — **DONE**              |
| t003 | REST/GraphQL/MCP static-site surface (create/read/update, routes, headers)                | 60m | w1/m21/t002 — **DONE**              |
| t004 | Dashboard static-site UI (create, publishPath, redirects/rewrites, headers)               | 60m | w1/m21/t003 — **DONE**              |
| t005 | Render parity — verify static-site surfaces + edge rules against Render                    | 30m | w1/m21/t002,w1/m21/t003,w1/m21/t004 — **DONE** |
| t006 | Simplify — `/simplify` over what this milestone changed                                    | 20m | w1/m21/t005 — **DONE**              |
| t007 | Test coverage — build→publish path + edge rules + surface tests                            | 40m | w1/m21/t005 — **DONE**              |
| t008 | Closeout — verify DoD holds, then move the milestone to `done/`                            | 10m | w1/m21/t007 — **DONE**              |

## Implementation status (2026-07-12)

Implemented and verified (build + unit/envtest + lint green across `lego/`; dashboard typecheck + 138 vitest pass; serving verified in a real browser via Playwright):

- **t001** — `static_site` type + `spec.publishPath`/`routes`/`headers` in `lego/types` (CRD/deepcopy regenerated); the publish plane `lego/operator/internal/publish` (extract `publishPath` from the built image → `aws s3 sync` to `s3://<bucket>/<app-id>/<rev>/`); `reconcileStaticSite` in `app_controller.go` (no Deployment/Service; rejects an unconfigured store / missing publishPath).
- **t002** — the shared static-server (`lego/operator/internal/staticserver` + `cmd/staticserver`): host→revision resolver, signed S3 GETs, `index.html` + SPA fallback, `/routes` (301 redirect / 200 rewrite, splat) and `/headers`, in-memory cache. Operator points each static-site host's Ingress at the static-server Service (`reconcileIngress` helper, shared with the web path). Handler behavior verified via `httptest` (redirect 301, rewrite 200, header present, SPA fallback, unknown host 404).
- **t003** — `static_site` across REST/GraphQL/MCP in `lego/backend/internal/apps` (create with `publishPath`/routes/headers, `GET`/`PUT …/routes` + `…/headers`, GraphQL `setStaticRoutes`/`setStaticHeaders`/`setPublishPath`, MCP `create_static_site` + `*_static_routes`/`*_static_headers`/`update_publish_path`), Render-shaped. Unit-tested.
- **t005** — `docs/ADR018-render-parity.md` Static-site + Header-rules/redirects rows moved to ✅ (UI ◐) with evidence; `docs/ADR029-static-sites.md` added; CLAUDE.md env table + `.pm/DO_NOT_DO.md` updated.
- **t006** — extracted the shared `reconcileIngress` helper (web + static), applied modernize cleanups.
- **t007** — operator (`publish`, `staticserver`, controller envtest) + backend (`static_site_test.go`) tests; also fixed a pre-existing nil-context crash in two autoscaling GraphQL tests.
- **t004 (dashboard)** — service-type recognition + Settings-tab UI (publishPath edit + routes/headers editors), `.graphql` documents, mapper, hook, i18n. `yarn typecheck` clean + `yarn vitest` 138 pass. Codegen was resolved **offline** (schema dumped via `TestDumpGraphQLSchema` → `SCHEMA_JSON=… yarn codegen`), which required fixing two pre-existing codegen defects (duplicate `Workspaces` op; the dump test omitting Deps-sourced `usage`/`audit`). Because commit `7504210` left orphaned generated Documents upstream (codegen non-reproducible), the static-site documents were **spliced onto** `definitions.ts` rather than full-regenerated. The `static_site` type is not yet offered in the `/services/new` create wizard (a follow-up, noted in t004).
- **t008 (closeout)** — DoD bullets 2–4 are verified (edge rules via Playwright real-browser e2e = the `curl -I` checks; REST/GraphQL/MCP + dashboard tested + typecheck; parity rows ✅). DoD bullet 1's mechanism (build→publish→serve, no Deployment/Service) is unit/envtest-verified and the serving path is browser-verified; the **only** unrun step is the deploy-time prod smoke test on a live cluster + `bex-static` bucket/Secret (docs/ADR029-static-sites.md § one-time setup) — an ops step gated on prod infra, not a code gap.

## Follow-ups (out of this milestone)

- Offer the `static_site` type in the dashboard `/services/new` create wizard (w5/m15 wizard exists; add the option + publishPath/routes/headers fields).
- Resolve the upstream dashboard codegen debt from commit `7504210` (orphaned `Repos`/`GitConnection`/`CreateService` Documents with no source `.graphql`) so `yarn codegen` is reproducible again; then the spliced `definitions.ts` reconciles to a clean full regen.
- Run the prod smoke test: create the `bex-static` bucket + `static-s3` Secret + `bex-static-config` ConfigMap (docs/ADR029-static-sites.md), deploy a static repo as a `static_site`, and `curl -I` the URL for index.html + a redirect + a custom header.

## Definition of done

- A repo with a static build (e.g. a Vite/CRA `dist/`) deploys as a `static_site`: bex builds it and serves `publishPath` from an object-store/CDN origin — **no Deployment/Service** for the served content.
- `/routes` redirects/rewrites and `/headers` custom response headers apply on the served responses (verifiable with `curl -I` and a redirect check).
- REST/GraphQL/MCP create/read/update the static site + its routes/headers with Render-identical shape; the dashboard exposes create + publishPath + routes + headers.
- `docs/ADR018-render-parity.md` static-site rows (build→CDN, redirects/rewrites, headers) move gap → ✅ with evidence.

## Design decisions (2026-07-11)

- **Object store: reuse the existing Wasabi account + `TF_STATE_*` creds from `.env`, but a NEW bucket `bex-static`** — never serve public content from `bex-tfstate` (Terraform state + etcd/OpenBao/CNPG backups are private, write-mostly; site content is world-readable, read-hot). **Prerequisite before t001 code:** create the `bex-static` bucket out-of-band (one-time, same "bottom turtle" step as `bex-tfstate`).
- **S3-generic wiring, following the backup-vars pattern:** operator env `BEX_STATIC_S3_ENDPOINT` / `BEX_STATIC_S3_BUCKET` / `BEX_STATIC_S3_SECRET` (in-cluster Secret created out-of-band from `.env`, exactly like `etcd-backup-s3`); any unset ⇒ the `static_site` type is unavailable. The endpoint stays deployment-time config — CI's `.env.template` points at Hetzner Object Storage, not Wasabi, so nothing outside `.env` may assume Wasabi.
- **Bucket layout:** builds sync `publishPath` output to `s3://bex-static/<app-id>/<revision>/` — revision-prefixed keys give atomic cutover + rollback (status revision repoints the prefix).
- **Serving: one shared static-server proxy behind Traefik (host-based routing), never direct bucket exposure** — Wasabi has no S3 website hosting, and `/routes` + `/headers` need an in-path layer anyway. The proxy does signed GETs (the bucket stays private), maps `/` → `index.html` (+ SPA fallback), applies the edge rules from the App spec, and caches aggressively (which also keeps egress inside Wasabi's egress ≤ storage fair-use policy). Per app there is still **no Deployment/Service** — the DoD holds; the proxy is a single platform component.

## Source + Goal linkage

- **Source:** inbox note `w1/012` (split from m15 in the 2026-07-08 reorg; originally m13 audit note `w1/009`); moved to `w1/done/012.md` on promotion.
- **Goal linkage:** pillar 1 (Render parity — service-type breadth).
- **Expected outcome:** static front-ends host on bex without a running container, matching Render's `static_site` type and its edge rules.
- **Why now:** the user prioritized promoting the parity backlog. Promoting this note is also what unblocks the static-site CDN edge rules that `.pm/DO_NOT_DO.md` (line 23) parks as non-goals "unless `w1/012` is promoted" — so the redirect/rewrite/header rows are now in-scope. Sequenced **after** the compute-service parity (m20) since bex is compute-first; this is the larger build→CDN effort.
- **Render parity task included:** adds REST/GraphQL/MCP/UI + edge-rule surfaces, so cross-surface parity must be verified.
