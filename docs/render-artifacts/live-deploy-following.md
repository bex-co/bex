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

## Platform progress-line contract (w1/m48/t001)

Render's platform narrates a deploy inside the log feed itself. From [Your First Render Deploy](https://render.com/docs/your-first-deploy) and public build-log excerpts, the canonical lines are `==> Cloning from https://github.com/…`, `==> Checking out commit <sha> in branch <branch>`, `==> Running build command '<cmd>'…`, `==> Build successful 🎉`, `==> Deploying…`, and `==> Your service is live 🎉`. Render's docs do not publish which API log `type` carries them or their label/id shape (the reference workspace's retained logs had expired, so this could not be captured live — recorded uncertainty; bex needs only a compatible _reading experience_, not byte equality).

bex synthesizes from what its control plane actually observes — the deploy row's three timestamps plus terminal status — never inventing clone/checkout moments it cannot see (those arrive as real BuildKit stdout once the pod runs):

| bex phase (deploys row) | timestamp | line (repo-backed) | line (image-backed) |
| --- | --- | --- | --- |
| row created | `created_at` | `==> Build queued` | `==> Deploy queued` |
| `started_at` set | `started_at` | `==> Building from <repo>@<commit\|branch>` | `==> Deploying image <image>` |
| terminal `live` / `deactivated` | `finished_at` | `==> Your service is live 🎉` | same |
| terminal `build_failed` | `finished_at` | `==> Build failed` | same |
| terminal `pre_deploy_failed` | `finished_at` | `==> Pre-deploy failed` | same |
| terminal `update_failed` | `finished_at` | `==> Deploy failed` | same |
| terminal `canceled` | `finished_at` | `==> Deploy canceled` | same |

Labels: `type=build`, `instance=<dep-… id>`, `container=platform`. Ids come from the existing `logID` derivation (instance + timestamp + message hash, `internal/logs/render.go`), so identical lines are deterministic across reads and dedupe against the SSE tail for free. Synthesis applies only when a query explicitly asks for `type=build` (matching which streams the store selector includes), is additive (a missing Loki still reports `buildStoreUnavailable`; platform lines never masquerade as a successful empty build history), and `deactivated` deploys keep their `live` closing line — deactivation is a later replacement event outside the deploy's own window.

## bex live verification (2026-07-17, w1/m48)

Verified on the local CAPD cluster through w1's isolated `dev-1` stack (fresh identity, fresh workspace, repo-backed service `m48-narrate` → `https://github.com/bex-co/hello-go.git`):

- an SSE `type=build` subscription opened seconds after create streamed `==> Build queued` immediately (no build pod existed yet) and `==> Building from https://github.com/bex-co/hello-go.git@main` live as `started_at` landed — the wait-loop transition emission;
- a later subscription caught up on both lines with byte-identical deterministic ids (`dep-…-<timestamp>-<hash>`);
- the dashboard deploy page rendered both platform lines mid-`Building` with the storeless `buildStoreUnavailable` banner still shown (synthesis does not mask it), the w1/029 **Log type** selector present, and its **Build logs** choice retaining the narration;
- the **unmodified official Render CLI** (`render logs --resources srv-… --type build --tail`) printed the same two lines with the same ids through a bex-minted API key.

dev-1 runs storeless, so the Loki-backed history leg (`GET /v1/logs?type=build` merging narration with real BuildKit stdout) is pinned by unit tests only; the prod check after the next bex-api deploy is `w1/032`. The temporary service, user, and API key were removed after verification.

## bex prod verification (2026-07-17, w1/032 — Loki-backed history + trigger-time clone-token remint)

Verified on production after bex-api rolled to `94ea930bc033` (the roll itself had been blocked all day by stale dashboard tests; fixed and shipped the same evening). One real manual deploy of the private-repo service `agentmarketcap-1` (`render deploys create srv-d9bkcspg9s7c73d0n8ug` through the prod-configured official CLI) produced `dep-d9dbej6168vc73dhm3t0` (`trigger=api`, createdAt `23:14:20Z`, startedAt `23:14:29Z`, finishedAt `23:17:29Z`, terminal `live`):

- **Trigger-time clone-token remint (the `w1/031` fix):** the deploy's trigger changed the `tea-…-agentmarketcap-1-clone` Secret's token bytes in **both** `default` and `bex-system` (sha1 `b08484e85c6d` → `1f418f4ad9bf`; the Secret object itself was updated in place, creationTimestamp still 2026-07-15). The build's first BuildKit step fetched the private repo for real (`#1 [internal] load git source …` → `From https://github.com/bex-co/agentmarketcap`) — no `could not read Username`. The four preceding `trigger=api` deploys of the same service (2026-07-16 → 17) had all failed exactly there; this one went `live`, the App reached `Running`, and the public URL answers 200.
- **Loki-backed narration (`type=build` history reads):** `render logs --resources srv-… --type build --start <createdAt> --end <finishedAt>` returned `==> Build queued` at createdAt, `==> Building from https://github.com/bex-co/agentmarketcap.git@a0c8b7f` at startedAt, and `==> Your service is live 🎉` at finishedAt — interleaved with the build's real BuildKit stdout read from Loki (hundreds of lines across pages; the server caps each page at Render's documented `limit=100`, newest first, so the early narration surfaces on a paged/narrower read of the same window).
- **Deterministic ids:** every read was performed twice — mid-flight and after the window closed, across three different slices — and every id was byte-identical across reads (47, 100, and 100 ids diffed clean). The mid-flight ids for the queued/building lines (`dep-…-2026-07-17T23:14:20.362551Z-5cd164f4`, `…-6c9c16d8`) reappeared unchanged in the closed-window reads.
- **Explicit-build-only:** untyped and `type=app` reads over the same window carried zero `==>` lines.
- **The dashboard's read path:** the dashboard deploy page's own GraphQL `Logs` document (`resource`/`type:"build"`/`startTime`/`endTime`) over the closed window returned the same merged feed with `==> Your service is live 🎉` in the newest page — the pure-history read, no SSE tail involved.

No temporary resources were created; the verification rode one real deploy of an existing service.
