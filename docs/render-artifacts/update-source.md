# Update Source — Render's dashboard bundle, 2026-11-23

## Sources

Read without signing in, on 2026-11-23, from Render's public dashboard bundle (`https://dashboard.render.com/` → `assets/index-BR2q8yex.js`, `assets/server-8p_bC2pd.js`, and `assets/cronjob-DfrSsyzb.js`), for `w4/m593`. Chunk hashes change on every Render deploy, so locate the code by the strings quoted below:

- `index-BR2q8yex.js`:
  - `rF` — the Settings → Build / Image **Source** row for both `Server` (Web Service, Private Service, Background Worker, Static Site) and `CronJob`.
  - `EP` — the **Update Source** modal wrapper (`title: "Update Source"`, `preventCloseOnOutsideEvent: true`, error banner `yg`, and `Edit` button gated on `updatePermission`).
  - `TP` — step-1 source tabs (`Git Provider`, `Public Git Repository`, `Existing Build Source`, `Existing Image`).
  - `wpe` — step-2 Git confirmation router dispatching to `Cpe` (Static Site), `vpe` (Docker), and `xpe` (native runtimes), each wrapped in footer `eP` (`Cancel`, `Back`, `Deploy` / `Deploying`).
  - `sme` — **Existing Image** tab (`Image URL`, `Credential (Optional)`, button `Connect` / `Connecting`).
  - `rme` / `nme` — **Existing Build Source** picker and step-2 confirmation (native `Start Command` / cron `Command`, protected-environment warning `NN`, buttons `Back`, `Cancel`, `Deploy` / `Deploying`).
- `server-8p_bC2pd.js`: `updateServer($id: String!, $patch: ServicePatchInput!)` returning `...serverFields`, which includes `deploy { id }`.
- `cronjob-DfrSsyzb.js`: `updateCronJob($id: String!, $patch: ServicePatchInput!)` returning `...cronJobFields` and `build { id }`.

The 2026-05-11 changelog entry ([Change your service's backing repo or image in the Render Dashboard](https://render.com/changelog/change-your-services-backing-repo-or-image-in-the-render-dashboard)) describes the same dashboard flow in prose, while Render's public REST API (`PATCH /v1/services/{serviceId}`) saves source updates without deploying.

## Render's behaviour

### Service-type matrix

| Service type | Settings section | Shown? | Allowed tabs in Update Source | Mutation & redirect target |
| --- | --- | --- | --- | --- |
| Web Service (`web_service`) | Build / Image → Source | Yes (omitted on PR previews) | Git Provider, Public Git Repository, Existing Build Source, Existing Image | `updateServer` → `deploy.id` |
| Private Service (`private_service`) | Build / Image → Source | Yes (omitted on PR previews) | Git Provider, Public Git Repository, Existing Build Source, Existing Image | `updateServer` → `deploy.id` |
| Background Worker (`background_worker`) | Build / Image → Source | Yes (omitted on PR previews) | Git Provider, Public Git Repository, Existing Build Source, Existing Image | `updateServer` → `deploy.id` |
| Cron Job (`cron_job`) | Build / Image → Source | Yes | Git Provider, Public Git Repository, Existing Build Source, Existing Image | `updateCronJob` → `build.id` |
| Static Site (`static_site`) | Build → Source | Yes (omitted on PR previews) | Git Provider, Public Git Repository only (`c !== Wc.static` hides Existing Image; `a = i && !$g(e.env)` hides Existing Build Source) | `updateServer` → `deploy.id` |

### Modal copy, tabs, and permissions

- **Edit button:** disabled only when `updatePermission.hasPermission === false` (`updateServer` / `updateCronJob`), showing `noPermissionMessage` in a tooltip. A suspended service does not disable the button.
- **Modal title:** **Update Source**. Outside click does not close the modal.
- **Step-1 description (`modalDescription`):**
  - Static site: _"Select a Git repository to back this static site."_
  - Non-static with Build Sources enabled: _"Select a Git repository, an existing build source, or a Docker image to back this `<type>`."_
  - Non-static without Build Sources: _"Select a Git repository or an existing Docker image to back this `<type>`."_
- **Initial tab (`Hme`):** `Existing Build Source` if currently linked to a build source (non-static), else `Existing Image` if currently image-backed, else `Git Provider`.

### Step-2 build-setting confirmations and payloads

1. **Git source — Static Site (`Cpe`):**
   - Fields: **Branch** (required), **Build Command** (optional), **Publish Directory** (required, placeholder `e.g. build`, prefilled from `staticPublishPath`).
   - Footer (`eP`): **Cancel**, **Back**, **Deploy** (`Deploying` while in flight).
   - Patch: `{ repo, branch, serviceDetails: { envSpecificDetails: { buildCommand }, publishPath } }`.
2. **Git source — Non-static Docker (`vpe`):**
   - Fields: **Branch** (required), **Runtime** (set to Docker). Build Command and Start Command are omitted.
   - Footer (`eP`): **Cancel**, **Back**, **Deploy** (`Deploying`).
   - Patch: `{ repo, branch, serviceDetails: { runtime: "docker" } }`.
3. **Git source — Non-static Native (`xpe`):**
   - Fields: **Branch** (required), **Runtime** (any non-static, non-image runtime), **Build Command** (required), **Start Command** (required; prefilled from existing value when keeping the same runtime, or sample/suggestion when switching runtimes; cron jobs prefill from `command`).
   - Footer (`eP`): **Cancel**, **Back**, **Deploy** (`Deploying`).
   - Patch: `{ repo, branch, serviceDetails: { runtime, envSpecificDetails: { buildCommand, startCommand } } }`.
4. **Existing Image (`sme`, non-static only):**
   - Single-step tab: **Image URL** (placeholder `docker.io/library/nginx:latest`) and **Credential (Optional)**.
   - Primary button: **Connect** (`Connecting` while in flight).
   - Patch: `{ externalImage: { imageURL, registryCredentialID, ownerID }, serviceDetails: { runtime: "image" } }`.
5. **Existing Build Source (`rme` → `nme`, non-static only):**
   - Step 1 lists workspace build sources (searchable, project-grouped, filtered by region compatibility for Git-backed build sources).
   - Step 2 (`nme`) confirms the choice:
     - For a native-runtime build source: required command input labeled **Start Command** (`Server`) or **Command** (`CronJob`).
     - In a protected environment (`NN`): inline warning _"Build Sources sit outside Protected Environment controls. Anyone with access to this source can influence what deploys to this service in `<environment>`."_
     - Footer: **Back**, **Cancel**, **Deploy** (`Deploying`).
   - Patch: `{ artifactSourceID, ...(startCommand !== null && { serviceDetails: { envSpecificDetails: { startCommand } } }) }`.

### Submit, deploy, error handling, and navigation

- Every Update Source submission runs one GraphQL mutation (`updateServer` or `updateCronJob`), which persists the source + confirmed build settings and immediately starts **one** deploy/build when the service can deploy.
- **Navigation:** `onMutationCompleted` redirects straight to the newly created deploy (`deploy.id` for servers, `build.id` for cron jobs) whenever an ID is returned. When no deploy ID is returned (for example, a suspended service or a build source with no succeeded build yet), the modal closes cleanly without navigating to a nonexistent deploy.
- **Validation and server errors:** caught in `EP`, rendered as an inline error alert inside the modal (`"Something went wrong. Please try again."` fallback), keeping the modal open so the user can correct the input without losing their draft.

### REST API and MCP vs Dashboard split

- Render's REST API `PATCH /v1/services/{serviceId}` saves source and build-setting fields **without** triggering a deploy; API callers trigger a deploy explicitly via `POST /v1/services/{serviceId}/deploys`.
- Render's MCP `update_service` follows the REST contract (save without deploying).
- The immediate deploy-on-submit behavior is specific to the Dashboard Update Source flow.

### Explicit non-goals for bex

- **PR preview services** (`!d` gate in Render's server settings) — deliberate non-goal (`.pm/DO_NOT_DO.md`).
- **GitLab and Bitbucket providers** — deliberate non-goal (`.pm/DO_NOT_DO.md`).
- **Per-member Git deployment credentials** — signed-off divergence (`.pm/DO_NOT_DO.md` `#GIT-WORKSPACE-OWNED`).
