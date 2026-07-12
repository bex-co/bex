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
| **Name** | Auto-filled from repo name slug; user can override |
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
| Env vars inline | ✖ Not in v1 — add env vars from the service settings tab post-create |
| Region picker | ✖ Not in v1 — bex operator picks the cluster's region automatically |
| Runtime / Build / Start | ✖ Not in v1 — CNB builder auto-detects; advanced fields deferred |
| Deploy button | ✅ `createService` mutation → redirect to `/services/$serviceId` |
