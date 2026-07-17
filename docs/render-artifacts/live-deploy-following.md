# Render live deploy following

**Captured:** 2026-07-15

**Method:** Render's public documentation (no authenticated production service was mutated for this capture).

## Observed contract

- Creating a service kicks off its first deploy and opens the new service's deploy-progress log explorer. Render tells users to “open your new service's page” and follow the build and start commands as they run.
- The deploy log feed updates in real time. A successful deploy changes its displayed status to **Live**; a failed deploy changes to **Failed** while leaving its log feed available for diagnosis.
- A dashboard Manual Deploy starts immediately from the service's Events page. An individual deploy is opened by selecting **Deploy** in its event entry; that page is the deploy-specific log explorer and is also where an in-progress deploy can be canceled.
- Render's general log explorer exposes **Live tail** as a time-range mode, and the deploy-specific explorer is documented as the place to follow build/start progress.

These sources establish the observable outcome—remain on the deploy-specific progress view while logs and status advance—but do not publish the dashboard's private navigation implementation. bex therefore treats the landing URL itself as UI behavior and keeps its public wire contract unchanged.

## Terminal behavior

- Success: status becomes **Live** and the accumulated deploy log remains visible.
- Failure: status becomes **Failed** and the accumulated log remains visible for troubleshooting.
- Cancellation: the deploy details page exposes **Cancel deploy** while the deploy is in progress.

## Sources

- [Your First Render Deploy — Monitor your deploy](https://render.com/docs/your-first-deploy#monitor-your-deploy)
- [Deploying on Render — Manual deploys and canceling a deploy](https://render.com/docs/deploys)
- [Logs in the Render Dashboard — individual deploy logs and Live tail](https://render.com/docs/logging)

## bex comparison (before w3/m14)

- Deploy detail already polled all non-terminal statuses every three seconds and stopped at every terminal status.
- Manual Deploy already navigated to the returned deploy id.
- The detail log pane polled Loki history; `GET /v1/logs/subscribe?type=build` refused build logs as store-only.
- Create navigated to the service overview even though store-managed create opened its first deploy row transactionally.

w3/m14 closes the last two gaps: build SSE follows the active build pod, and create returns/navigates to the first deploy with an honest service-page fallback when no deploy id is available.

## bex live verification (2026-07-15)

Verified on the local CAPD cluster through w3's isolated `dev-3` identity/API stack and a real headless Chrome session against the dashboard:

- a create-triggered git build produced 128 `type=build` SSE frames and transitioned `build_in_progress` → `live`; the App reached `Running`;
- the dashboard Manual Deploy confirmation navigated directly to `/services/<service>/deploys/<deploy>`; its build panel visibly appended BuildKit lines while Chrome remained on that page;
- the same open page updated its deploy header, timestamps, and timeline to terminal `Build Failed` without a refresh when exercised once without the temporary registry;
- a subscription after the build had ended emitted the named terminal `no running build is available to follow` error instead of hanging.

The temporary test services, registry, pull/push Secrets, and browser profile were removed after verification. No production service was mutated.

## 2026-07-17 production incident + Render deploy-page recheck

A real production deploy (`dep-d9d8r3h07a5s73dj32ag`) showed only old-revision app lines while its build ran. Three compounding causes, verified against prod Loki and the deploy row:

- The build pod stayed Pending for ~2 minutes (cold buildkit image pull), emitting nothing; `followBuildLogs` answered the deploy page's one SSE subscription with the terminal `no running build` event, and the client never retried — the tail was dead before the build's first line.
- The build then failed within seconds; the deploy closed its window and the page stopped polling immediately, racing Loki ingest for the few in-window build lines.
- The still-running previous revision kept logging into the open window — correct interleaving, but the only visible content.

An authenticated dashboard.render.com walk (`/web/srv-…/deploys/dep-…`) confirmed the deploy page is Render's general log explorer scoped to the deploy's absolute window (`?r=start~end`) with an **All logs** default filter offering **All logs / Application logs / Build logs** — bex's interleaving is parity-correct. Render additionally synthesizes platform progress lines (`==> Cloning from…`, `==> Checking out commit…`, `==> Running build command…`), so its feed is never silent during scheduling; bex emits no platform lines (open parity gap).

Fixes shipped with this recheck: `followBuildLogs` now waits for a Pending build pod to start instead of terminating (completed-only or no pods remains the immediate named terminal outcome), the dashboard build tail reopens a terminated subscription every 5s while the deploy is still `build_in_progress`, and the deploy page's windowed queries keep polling 15s past `finishedAt` to absorb store ingest lag.
