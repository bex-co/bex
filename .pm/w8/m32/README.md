# w8 · m32 — Static-site create wizard offers an invalid Docker-image source — gate source by service type (UI + API + CRD)

**Worker:** worker8 **Goal:** A `static_site` can only be sourced from a Git repo — the "Existing Image" (Docker) source is unreachable in the create wizard and the source-edit card, and both the bex-api create path and the CRD refuse `type: static_site` + `image`, while the four image-valid service types (web · private · worker · cron) keep working. **Status:** in progress — t001–t007 done 2026-09-06 (all four layers guarded + tested; suites green: backend `go test ./...`, operator `make test` incl. new envtest, `make lint`, dashboard `yarn lint`+`yarn test`); t008 closeout awaits the live dev-8/dashboard DoD probes and ship.

## Tasks (in order)

| id   | title                                                                   | est | depends_on       |
| ---- | ----------------------------------------------------------------------- | --- | ---------------- |
| t001 | Gate create-wizard Source tabs by service type (omit image for static) — **DONE** | 30m | —                |
| t002 | Harden create submit gate + payload builder against static+image — **DONE** | 30m | t001             |
| t003 | Backend + CRD guard: reject `type: static_site` with an image source — **DONE** | 45m | —                |
| t004 | Gate the "Update Source" settings card for existing static sites — **DONE** (card already unmounted for static; API-side gap closed instead — see task Outcome) | 30m | t001             |
| t005 | Render parity — source-vs-type across REST · GraphQL · MCP · UI — **DONE** (blueprint drift fixed too) | 30m | t002, t003, t004 |
| t006 | Simplify — the create/source-picker code this milestone changed — **DONE** | 20m | t005             |
| t007 | Test coverage — static+image refused, four image types still create — **DONE** | 45m | t005             |
| t008 | Closeout — land when the live DoD holds                                 | 10m | t007             |

## Definition of done

Each bullet names the target behavior and is a click or command the next person can repeat.

- On `https://dashboard.bex.co/services/new?type=static_site`, the **Source** selector shows exactly two tabs — **GitHub** and **Public Git URL** — with no "Existing Image" tab and no reachable "Docker Image" field. On `?type=web_service`, `private_service`, `background_worker`, and `cron_job`, all three tabs (including "Existing Image") still render.
- With **Static Site** selected, no field combination enables **Deploy Service** through an image source; enabling Deploy for a static site requires a Git repo (GitHub or Public Git URL) plus a publish directory.
- `POST /v1/services`, the GraphQL `createService` mutation, and the MCP service-create tool, given `type: static_site` + an `image` and no repo, are **rejected with a clear 4xx validation error** naming that static sites build from a Git repo — not a created App. Verify with an API probe (curl/fetch) against **dev-8**, never production. _(Unverified this hunt: backend acceptance was established by a code trace — `service.go:2395-2403`, `resolveBuildStrategy` `service.go:2459-2463`, and the absent `XValidation` in `app_types.go:268-273` — not by creating a broken resource on prod. This bullet is that live check, run in dev-8.)_
- Editing an existing static site (Settings → **Update Source**) offers only Git-repo sources — no "Existing Image" option. _(Unverified this hunt: the sibling gap was found by citation `service-source-card.tsx:202-209`, not exercised live — no static site existed to edit in the QA workspace.)_
- The CRD refuses a `static_site` App carrying `image` (envtest/validation test proves the API server rejects it), so the guard holds even if the API layer is bypassed.
- The render.yaml / blueprint compile path is checked for the same `static_site` + image combination (ADR049); note in t003/t005 whether it already refuses or needs its own guard (`blueprint_compiler.go:279-288` does not currently forbid `publishPath` on an image source).
- Regression: web_service, private_service, background_worker, and cron_job each still create successfully from an "Existing Image" source, proven by tests (the allowlist did not over-reach).

## Source + Goal linkage

- **Source:** 2026-09-03 live QA hunt `/qa-find-bugs for w8` on production `https://dashboard.bex.co`. Reproduced fresh on `/services/new?type=static_site`: selecting the **Existing Image** source shows a "Docker Image" field beside the static "Publish Directory" field and, after entering `docker.io/library/nginx:latest`, the **Deploy Service** button flips from disabled to enabled — i.e. the invalid `type: static_site` + `image` + `publishPath` combination is submittable. Evidence screenshot `.playwright-mcp/qa-static-1.png` (gitignored, local to that session; the durable artifacts are the repro steps and the code citations below).
- **Root cause (code trace):**
  - UI — `dashboard/src/routes/services.new.tsx:135-142` passes the `image={{…}}` prop to `ServiceSourcePicker` unconditionally, never gated on `form.serviceType`; the picker renders the image tab whenever that prop is present (`dashboard/src/features/services/components/service-source-picker.tsx:150`, tab union `:24`, docstring `:48-51`). The correct pattern already exists: the Blueprint caller omits `image` (`dashboard/src/routes/blueprints.new.tsx:162`).
  - UI submit/payload — `dashboard/src/features/services/lib/create-service-input.ts:114` accepts an image source with no service-type check; `:130` bypasses the plan requirement for static; `buildCreateServiceInput` emits `runtime:"image"` (`:168`) and still attaches `publishPath` (`:192`); no static-vs-image conflict guard in `buildShape` (`:67-90`).
  - Sibling entrypoint — the "Update Source" settings card `dashboard/src/features/services/components/service-source-card.tsx:202-209` also passes `image` unconditionally.
  - Backend (one create path shared by REST + GraphQL + MCP — ADR006) — `validateTypeSpecificCreate` static branch `lego/backend/internal/apps/service.go:2395-2403` requires only `publishPath`, never rejects `image`; `resolveBuildStrategy` `case "image"` `service.go:2459-2463` has no static exclusion; `validateCreateSource` `service.go:2337-2367` only requires one of repo/image; `specFromCreate` copies both `service.go:2292-2293`.
  - CRD — `lego/types/v1alpha1/app_types.go`: `Image` `:356-363`, `Type` enum includes `static_site` `:292`, `PublishPath` required for static `:321-328`, doc says "Either Repo or Image must be set" `:346-347`, but the spec `XValidation` rules `:268-273` do not exclude image for static_site.
- **Goal linkage:** Render parity ([ADR018](../../../docs/ADR018-render-parity.md)) and correct static-site semantics ([ADR029](../../../docs/ADR029-static-sites.md): a static site is built from a repo and published to an object-store origin — `:3`, `:18`, `:112`; there is no prebuilt-external-image path). Render's static sites are Git-repo-only, so offering a Docker source is a parity divergence and a logic bug. Also touches render.yaml parity ([ADR049](../../../docs/ADR049-render-yaml-parity.md)).
- **Expected outcome:** the create wizard, the source-edit card, bex-api, and the CRD all agree that a static site is Git-only; a user can no longer build (or submit) an impossible static-site-from-image resource, and the four image-valid service types are unaffected.
- **Why now:** w8/m31 (In-place GitHub Credentials menu, done 2026-09-03) reworked this exact `ServiceSourcePicker` and the create wizard is actively evolving; the invalid combination is live on production today (verified against HEAD — not a deploy-lag; no fix exists on `main`).
- **Blast radius:** the source-tab list is shared across all five service types (`dashboard/src/features/services/lib/create-context.ts:4-10`). "Existing Image" is valid for web_service, private_service, background_worker, and cron_job (image runtime supported — `app_types.go:254-258`) and invalid only for static_site, so the fix is allowlisted to static_site and the four valid types get regression tests (t007).
- **Render parity task included** because the change spans REST, GraphQL, MCP, the dashboard UI, and the CRD — every surface a caller could reach must agree.
