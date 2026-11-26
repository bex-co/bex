# Capture — Render's new-service wizard (w5/m15 t005)

**Captured:** 2026-07-11 · **Method:** docs-fallback (no live Render login available; grounded in Render's public docs — `render.com/docs/web-services` and `render.com/docs/deploy-from-git` — plus field inspection of Render's dashboard at `dashboard.render.com/web/new`). The design source for the bex `/services/new` wizard.

## Render's `/web/new` flow

Render's new-service wizard is a stepped page, not a multi-tab SPA. The user lands at a source-picker and advances through settings before a final deploy button.

### Step 1 — Source picker

Three options presented as large cards or tabs:

| Source option       | What Render shows                         |
| ------------------- | ----------------------------------------- |
| **GitHub / GitLab** | OAuth-connected repo list with search box |
| **Public Git URL**  | A plain URL input (`https://...`)         |
| **Existing image**  | Docker image reference input              |

If GitHub is not connected, Render shows a "Connect GitHub" button inline; once connected the repo list populates immediately (no page refresh).

### Step 2 — Repo picker (GitHub source only)

- Search box filters by repo name
- Each row: repo name, visibility badge (Private), default branch
- Clicking a row selects it and continues to settings (same page, scrolled or stepped)

### Step 3 — Settings

| Field | Default / behavior |
| --- | --- |
| **Name** | Auto-filled from repo name slug; user can override; must be unique within the workspace — see [`duplicate-service-names.md`](duplicate-service-names.md) for the reject + suggest behavior on collision |
| **Region** | Dropdown of Render's hosting regions (Oregon, Frankfurt, etc.) |
| **Branch** | Auto-filled from repo's default branch |
| **Root directory** | Empty (repo root) |
| **Runtime** | Auto-detected (Node, Python, Go, Docker, etc.) |
| **Build command** | Auto-detected or empty |
| **Start command** | Auto-detected or empty |
| **Instance type** | Radio-group card grid: Free → Starter → Standard → Pro → … |
| **Auto-deploy** | Toggle: on by default |
| **Env vars** | Key/value table inline (add row button) |

Render also shows **Health Check Path**, **Scaling**, and **Advanced** sections collapsed below.

The Advanced section's fields are read from Render's dashboard bundle in [§ The Advanced section](#the-advanced-section-read-2026-11-25-w4m651-t001) below.

### Step 4 — Deploy

Single "Create Web Service" button at the bottom. On success, Render redirects to the service's overview page showing `Deploying` → `Live` phase transition.

## bex `/services/new` parity decisions

| Render feature | bex implementation (w5/m15) |
| --- | --- |
| Source picker tabs | ✅ Three-tab layout: GitHub / Public Git URL / Existing Image |
| GitHub repo list | ✅ `repos` query + search filter + Private badge + branch |
| Connect-GitHub prompt | ✅ Inline empty-state with `installUrl` link when not connected |
| Name auto-fill | ✅ Derived from repo slug / URL slug / image slug |
| Branch field | ✅ Auto-filled from `defaultBranch`; hidden for image source |
| Root directory field | ✅ Empty default; hidden for image source |
| Instance type grid | ✅ Card radio-group from `instanceTypes` query |
| Auto-deploy toggle | ✅ On by default; hidden for image source |
| Env vars inline | ✅ Key-value editor in the Settings step (w5/m19); Render places this in an "Advanced" section; bex renders it inline at the bottom of Settings, after Auto Deploy |
| Region picker | ✖ Not in v1 — bex operator picks the cluster's region automatically |
| Runtime / Build / Start | ✅ Render-native runtime selector plus editable Build Command and Start Command; Docker uses its repository Dockerfile |
| Deploy button | ✅ `createService` mutation → redirect to `/services/$serviceId` |

## The Advanced section (read 2026-11-25, w4/m651 t001)

**Method.** Read without sign-in from Render's public dashboard bundle on 2026-11-25 (no Render session was available to the run): `https://dashboard.render.com/` loads `assets/index-D5V4ZgbE.js`, and following its imports fetches 459 chunks (the method of [project-observability.md § Operators per field](project-observability.md#operators-per-field-read-from-the-bundle-2026-11-19-w3m523)). The service create form's Advanced card is one component in `index-*.js`; find it by its `Advanced Button Clicked` event next to the `Secret Files` heading (the other `Advanced` card in the same chunk is the Workflow create form's). `NewServiceSourceCode-*.js` holds the form's initial values and submit. The hashes change with every Render deploy. Nothing was submitted.

One component serves web services, private services, background workers, cron jobs and static sites. Its fields, in order, with the condition each one renders under:

| # | Field | Shown when | Render's copy and rules |
| --- | --- | --- | --- |
| 1 | **Secret Files** | always | run-time only for an image or Build Source service, build-time only for a static site |
| 2 | **Linked Environment Groups** | the workspace has a group usable in the chosen environment, or one is already linked | "Environment groups are collections of environment variables and secret files that you can share across multiple services." |
| 3 | **Disk** | not a static site, not a cron job, plan not Free | "Attach an SSD to persist your service's filesystem data across deploys." Mount path and size (default 10 GB); choosing Free removes any disk from the draft |
| 4 | **Health Check Path** | a web service (any source) | placeholder `/healthz`; "Provide an HTTP endpoint path that Render messages periodically to monitor your service."; refused unless it begins with `/` ("Path must be an absolute path (begin with /)"), empty allowed |
| 5 | **Registry Credential**, **Docker Build Context Directory**, **Dockerfile Path** | a Docker build from a repository, not a Build Source | the context is "relative to the repo root. Defaults to the root."; both paths refuse a `..` segment |
| 6 | **Docker Command** | a Docker build or an existing image | overrides the Dockerfile's `CMD` and `ENTRYPOINT` |
| 7 | **Pre-Deploy Command** | a web service, private service or background worker (any source) | `$` prompt with the Root Directory; "Render runs this command before the start command. Useful for database migrations and static asset uploads."; on the **Free** plan the input is replaced by a disabled one whose tooltip reads "Pre-Deploy Command is available for paid instances only." |
| 8 | **Auto-Deploy** | not an image, not a Build Source | a Yes/No toggle; the On Commit / After CI Checks Pass / Off selector behind the `expose-auto-deploy-trigger` flag ([dashboard-operations.md](dashboard-operations.md)) |
| 9 | **Build Filters** | not an image, not a Build Source (every repository-backed type, static sites included) | Included Paths and Ignored Paths; "Include or ignore specific paths in your repo when determining whether to trigger an auto-deploy." |

**bex's create form against it** (`dashboard/src/features/services/pages/new-service-page.tsx`):

| Render field | bex before w4/m651 | w4/m651 |
| --- | --- | --- |
| Secret Files | present | unchanged |
| Linked Environment Groups | absent; `createService` takes no environment groups on any surface | filed as [w4/731](../../.pm/w4/731.md) |
| Disk | absent | [w4/m652](../../.pm/w4/m652/README.md) adds it |
| Health Check Path | absent | added, web services only |
| Registry Credential, Docker Build Context Directory, Dockerfile Path | Dockerfile Path and Registry Credential only | Docker Build Context Directory added, the three in Render's order |
| Docker Command | present | unchanged |
| Pre-Deploy Command | absent | added for web services, private services and background workers, disabled on Free with Settings' reason |
| Auto-Deploy | present (toggle) | unchanged |
| Build Filters | static sites only | every repository-backed type |
