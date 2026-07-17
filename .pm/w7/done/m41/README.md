# w7 · m41 — Static site create: Build Command + Build Filters

**Worker:** worker7 **Goal:** bex's static site create wizard and settings match Render's — a Build Command field appears prominently in the create form (currently hidden by `!isStaticType` guard), the create payload sends it, the settings page exposes an editor for it, and Build Filters appear in the create form's Advanced section (currently absent from any create wizard). **Status:** done

## Tasks (in order)

| id   | title                                                                                         | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Create wizard: show Build Command field for static sites + fix payload                        | 30m | —          | — **DONE** |
| t002 | Backend: `setBuildCommand` mutation + `buildCommand` in server query + `buildFilter` in `createService` mutation | 45m | —          | — **DONE** |
| t003 | Settings: Build Command editor in `BuildDeploySection` for static sites                       | 45m | t002       | — **DONE** |
| t004 | Create wizard: Build Filters in Advanced section for static sites                             | 45m | t002       | — **DONE** |
| t005 | Render parity — verify create form + settings match Render's static site flow                 | 20m | t003, t004 | — **DONE** |
| t006 | Simplify — `/simplify` over the milestone's diff                                              | 15m | t005       | — **DONE** |
| t007 | Test coverage — meaningful tests for the shipped behavior                                     | 30m | t005       | — **DONE** |
| t008 | Closeout — move to `done/` when the DoD holds                                                 | 15m | t007       | — **DONE** |

## Definition of done

In the dashboard, when creating a `static_site`:

1. A **Build Command** field appears in the create wizard (above Publish Directory, matching Render's layout); the submitted payload includes it when filled.
2. A **Build Filters** (Included/Ignored Paths) accordion appears in the Advanced section of the create wizard; the submitted payload includes it when configured.
3. In Settings → Build & Deploy, a **Build Command** inline editor is present for git-sourced static sites (same pencil-edit pattern as other command editors), reads the current value, and saves via `setBuildCommand`.
4. All new fields round-trip correctly: create → read-back in settings → save from settings → re-read.
5. Build Filters in settings already work (existing `BuildDeploySection` with `BuildFilterEditor`) — regression tests confirm they still do.

## Source + Goal linkage

- **Source:** User request 2026-07-16 — Playwright research on `https://dashboard.render.com/static/new` revealed that Render's static site create form shows Build Command prominently and Build Filters in its Advanced section; bex's create wizard hides both. Backend REST/GraphQL/MCP already accepts all these fields (`w1/m21`, `w1/m34`). Only the dashboard is the gap. Prior parity walk (w5/m32) marked static site UI as ✅ based on publishPath + routes + headers, not checking Build Command parity in the create form.
- **Goal linkage:** Render dashboard parity (`docs/ADR018-render-parity.md` — static site and Build Filters rows); the missing build command in the create wizard means static sites created via the dashboard cannot have a build step configured at creation time, making the most common use-case (compile-from-source) require an extra API/settings round-trip.
- **Expected outcome:** Static site creation via the dashboard is self-contained — a user can configure build command + build filters at the moment of creation without needing a post-create settings edit.
- **Why now:** Static site was marked ✅ in ADR018 based on publish + routes + headers; the build-command gap in the create wizard was discovered during a live Playwright comparison and is the most common friction point for git-backed static site creation. The backend already supports these fields — this is pure dashboard polish.
- **Render parity:** included (t005) — UI-surface feature work touching the create wizard and settings.
