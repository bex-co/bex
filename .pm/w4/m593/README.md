# w4 · m593 — Updating a service's source in Settings deploys it, reconfirms its build settings, and covers static sites, as on Render

**Worker:** worker4 **Goal:** submitting a new source in Settings → Source deploys the service immediately with the new source and confirmed build settings (Runtime, Build Command, Start Command / cron Command, or static-site Publish Directory) and navigates to the new deploy, on every service type Render offers it for — including static sites — while REST `PATCH` and MCP keep their save-without-deploy contract. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's current Update Source contract from its dashboard bundle | 30m | — |
| [t002](t002.md) | Backend: GraphQL source-update mutations accept Render's build confirmations and trigger one deploy when requested | 1h | t001 |
| [t003](t003.md) | Dashboard: add the Source section to static-site Settings and match its pending skeleton | 40m | t001 |
| [t004](t004.md) | Dashboard: Update Source dialog reconfirms build settings and deploys on submit for every service type (8 locales) | 1h30m | t002, t003 |
| [t005](t005.md) | Live check on dev-4 across service types and widths (1440 and 390) | 45m | t004 |
| [t006](t006.md) | Render parity | 20m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 40m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 6h15m**.

## Definition of done

Verified by backend + dashboard suites and replayed live on `dev-4` (at 1440 and 390):

- Every service type (`web_service`, `private_service`, `background_worker`, `cron_job`, and `static_site`) shows the Settings **Source** card, and a static site's Settings pending skeleton matches its ready layout at 1440 and 390.
- For a **static site**, Settings → Source → Edit shows only the Git tabs (GitHub and Public Git URL — no Existing Image and no Build Source), reconfirms Branch, optional Build Command, and required Publish Directory, and clicking **Deploy** saves the source + build settings and starts **one** deploy, navigating to it.
- For a **non-static** service, picking a Git source reconfirms Branch and Runtime (Docker or native) and, when native, Build Command and Start Command (`Command` for cron jobs); submitting a Git source, an Existing Image, or a Build Source saves and starts **one** deploy, navigating to it (suspended services and build sources awaiting their first build are handled cleanly without leaving partial state or an unhandled error).
- REST `PATCH /v1/services/{id}` (and MCP `update_service` / `link_service_to_build_source`) keep their save-without-deploy contract.
- `docs/render-artifacts/update-source.md` records Render's bundle behavior, `docs/ADR018-render-parity.md` drops the unapproved 2026-08-25 divergence note, `docs/ADR026-github-integration.md` §8 is updated, and `docs/render-artifacts/render-changelog.md` marks the 2026-05-11 entry `matched`.

## Source + Goal linkage

- **Source:** inbox note w4/718 ([`source-718.md`](source-718.md)), filed by w4/m553 t002 on 2026-11-22 from Render's 2026-05-11 changelog entry. Promoted by `/loopx w4` on 2026-11-23 after verifying Render's current dashboard bundle and sizing the work across backend GraphQL + deploy orchestration, static-site settings + skeleton parity, the two-step Update Source dialog across 8 locales, live `dev-4` replay, and ADR updates (~6h15m across 9 tasks).
- **Premise checked 2026-11-23 on `5ac4627faa`:**
  - **Render's current dashboard bundle** (`https://dashboard.render.com/assets/index-BR2q8yex.js`, `server-8p_bC2pd.js`, `cronjob-DfrSsyzb.js`, read 2026-11-23 without signing in): `rF` renders Update Source (`EP`) for **all** service types (`web`, `worker`, `pserv`, `cron`, and `static`, omitting only PR preview services). For static sites (`$g(e.env)` / `e.env.isStatic`), `TP` hides Existing Image (`c !== Wc.static`) and Existing Build Source (`a = i && !$g(e.env)`), the modal reads *"Select a Git repository to back this static site."*, and step 2 (`Cpe`) asks for Branch, optional Build Command, and required Publish Directory (`staticPublishPath`), submitting `{ repo, branch, serviceDetails: { envSpecificDetails: { buildCommand }, publishPath } }` with button **Deploy** (`Deploying`). For non-static Git sources, step 2 (`wpe`) asks for Branch + Runtime (`vpe` for Docker omits commands; `xpe` for native requires Build Command and Start Command, labeled `Command` for `CronJob`). Existing Image (`sme`, button **Connect**) and Build Source (`rme` → `nme`, button **Deploy**) submit through the same `updateServer` / `updateCronJob` mutation returning `deploy { id }` / `build { id }`, and `onMutationCompleted` navigates straight to the new deploy.
  - **bex today:** `dashboard/src/features/services/lib/settings-sections.ts:78` skips `"source"` for static sites (`if (!staticSite) sections.push("source")`). `dashboard/src/features/services/components/service-source-card.tsx` states *"Saving does not trigger a deploy"* (`services.sourceNoAutoDeploy`), has no Runtime picker or Publish Directory step, asks for commands only when switching from an image back to a native Git repo (`needsCommands`), and calls `setRepo` / `setImage` / `linkBuildSource` without deploying. Backend `lego/backend/internal/apps/graphql.go:1760-1807` and `build_sources_graphql.go:228` stage the source (`markSourcePending` / `patchUntracked`), accept no `runtime` or `publishPath` on `setRepo`, and open no deploy. `docs/ADR018-render-parity.md:132` carries a 2026-08-25 "Verified divergence" without user sign-off.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). An unapproved divergence cannot stand in ADR018; customers updating a service's source in Settings expect the new code to deploy immediately and static sites to be editable like every other service type.
- **Expected outcome:** updating a service's backing source in Settings works identically to Render across all five service types — one dialog, build-setting confirmation, immediate deploy, and redirect to the new deploy — while API/MCP patches keep their save-without-deploy contract.
- **Why now:** surfaced by w4/m553's changelog audit (`docs/render-artifacts/render-changelog.md:53`) as an unowned Render parity gap with an unsigned-off ADR018 divergence.
- **Render parity:** included (t006). The change faces tenants on GraphQL and the dashboard and preserves REST/MCP parity.
