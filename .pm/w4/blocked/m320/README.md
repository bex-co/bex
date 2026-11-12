# w4 · m320 — An image web service's deploy takes no traffic until it is the live release: HTTP port discovery doesn't put the candidate in front of users, an image change rolls once, and a canceled deploy never answered

**Worker:** worker4 **Goal:** a web service deploy that changes its image behaves as Render's zero-downtime deploy does. The live release receives all traffic while the new one starts. Traffic moves to the new release at the moment its deploy becomes `live`. A deploy canceled before that point never answered a request, and the release that was serving keeps its own pods. **Status:** blocked — t001–t005, t007–t009 and t011 done (shipped); t012 (round 156: a supersede in the settling window) is open; t006 live replay follows it and the next production deploy, then t010 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Keep the Services on the serving release while HTTP discovery is pending, and move them at promotion — **DONE** | 60m | — |
| [t002](done/t002.md) | One rollout per image change: no second ReplicaSet after discovery drains the serving release — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Cancel before promotion keeps the serving pods, and the canceled candidate never answered — **DONE** | 30m | t001, t002 |
| [t004](done/t004.md) | A deploy whose release already holds all traffic reads `live` in the same pass — **DONE** | 40m | t001 |
| [t005](done/t005.md) | Blast radius: every rollout kind through the Service selector and the discovery hold — **DONE** | 40m | t002, t003, t004 |
| [t006](t006.md) | Live verification on an owned image web service | 30m | t005, t012 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007 |
| [t011](done/t011.md) | Regression of w8/m122: every image change on an image web service goes live without `image.sha`, because the row closes `live` before the operator records the digest — **DONE** | 40m | t004 |
| [t012](t012.md) | Regression of w4/405: a second deploy that arrives after an image release already holds all traffic, but before its row reads `live`, closes that release `canceled`. This includes a first deploy, which leaves the service with no live row. After an image change it also puts an older release back and drops requests | 75m | t004 |
| [t010](t010.md) | Closeout | 10m | t008, t009, t011, t012 |

Twelve tasks, 435m (7h15m); t012 added in round 156 and widened in round 157. **Major:** an image change puts a release in front of users before the product says it is live. A canceled release keeps answering after the API has returned `canceled`.

## Finding

Live `/qa-find-bugs w4` loop, round 35, 2026-11-10 18:10–19:16 UTC (`muse.env`, workspace `bex-canary`, pin `c12cd57facb1`).

**Fixture:** Free web service `qa-20261110-dh35` (`srv-dbplsbs5ngvc73e8u12g`), with no port set. It was created in the dashboard from `docker.io/mendhak/http-https-echo:35` and later switched with `PATCH image` to `docker.io/library/nginx:1.27-alpine`. Deleted afterwards (GET 404).

**Probes, run together:**

- A poller read the `Server` header of `https://qa-20261110-dh35.onbex.co/` every ~0.5 s. `nginx/1.27.5` and `nginx/1.26.3` name the release that answered.
- In-page polling of `GET /v1/services/srv-dbplsbs5ngvc73e8u12g/deploys?limit=1` every 1–1.5 s.
- `GET /v1/logs` (`resource=srv-dbplsbs5ngvc73e8u12g`) for the instance label on each access-log line.

**Run A: a hook deploy of a new tag, left to finish.**

1. The live release was 1.27.5 (instance `…2e38qqode71fqfan5u17`).
2. `GET <deploy hook URL>&imgURL=docker.io%2Flibrary%2Fnginx%3A1.26-alpine` returned `200 {"deploy":{"id":"dep-dbpm2mtme2kc73f2ld5g"}}` at 18:24:25.
3. Deploy status: `update_in_progress` from 18:24:43, then `live` at 18:26:13.
4. What actually answered:
   - **Candidate 1** (`…00ipsl8s5h6227hc9pif`) started at 18:24:37. It answered public requests from 18:24:49, alternating with the live 1.27.5 until 18:25:36: 41 × `nginx/1.26.3` and 28 × `nginx/1.27.5` over the whole poll.
   - **Candidate 2**, another 1.26.3 instance (`…slc7ps9odpcgn8bcbisl`), started at 18:25:30. From 18:25:37 every request returned 1.26.3.
5. The candidate answered for **84 s before the deploy was live**, alone for the last 36 s. Meanwhile REST and the dashboard still named the 1.27-alpine deploy as live.

**Run B: canceled while the candidate already answered.**

1. A hook with `imgURL=…nginx%3A1.27-alpine` started `dep-dbpm3slme2kc73f2ld60` at 18:26:58.
2. `nginx/1.27.5` first answered at 18:27:18.
3. `POST /v1/services/srv-dbplsbs5ngvc73e8u12g/deploys/dep-dbpm3slme2kc73f2ld60/cancel` at 18:27:24.18 returned `200` with `"status":"canceled"`.
4. The canceled 1.27.5 still answered at 18:27:28.0, 18:27:36.9 and 18:27:38.9, 14.7 s after the cancel returned.

**Run 0: the same through the dashboard, on the echo image (:35 → :34 via the hook).**

- **Before the cancel.**
  - The :34 candidate (`…67lad53h1n8n8tu1lucg`) answered public `GET /qa-probe` at 18:15:00 and 18:15:02.
  - At the same time the deploy detail page read "Deploy status In Progress" and "Waiting for the container to start before detecting its HTTP port".
  - A second :34 instance (`…tvm1h80ad7avcv1cmg08`) started at 18:15:26.
- **The cancel.** I clicked `button "Cancel deploy dep-dbpltvdme2kc73f2ld40"`. The dialog said "If an earlier deploy succeeded, it stays live". Proceed → toast "Deploy canceled." (`finishedAt` 18:15:31).
- **After the cancel.**
  - `…tvm1h…` (canceled :34) still answered `/qa-probe-1` at 18:15:46 and `/qa-probe-2` at 18:15:50.
  - The prior release came back on a **fresh** pod (`…auiu7jql1kalvl7shudj`, started 18:15:48), because the second roll had already drained the live pod `…5182h8nuvpaqvlb3rsft`. That pod logged its SIGTERM at 18:16:26.

**Control: a same-image redeploy, which needs no discovery.**

- `POST …/deploys` `{}` with the saved image unchanged → `dep-dbpmq1c5ngvc73e8u1f0`.
- One instance switch at 19:14:35 (`…5ejrbnnat4cdg93dvd8p`), with no alternation and no second ReplicaSet. Live at 19:14:59.
- The deploy took 46 s, against 108 s for Run A. The 24 s between the switch and `live` is the gap [w4/405](../../done/405.md) recorded and accepted (see t004).

Poller output is saved locally as `.playwright-mcp/qa-deploys-35-poll-a.txt` and `qa-deploys-35-poll-b.txt` (gitignored). The durable artifact is the probe above, which anyone can re-run.

### Addendum, 2026-11-10 (round 36): git-built Docker services are affected too

Live `/qa-find-bugs w4` loop, round 36, 19:25–19:40 UTC (same pin and workspace).

**Fixture:** Free web service `qa-20261110-lg36` (`srv-dbpmvac5ngvc73e8u1j0`), built from `bex-co/bex`, `rootDir: examples/hello-go`, auto-deploy off. Deleted afterwards (GET 404, URL 404).

- **Its first deploy went through HTTP discovery.** The deploy log read `==> Detected HTTP on port 10000; serving it`.
  - The platform injects `PORT=10000`, which overrides the Dockerfile's `ENV PORT=3000`, as on Render. The app listened on 10000.
  - That injected value doesn't exempt the service from discovery.
- **Two rebuilds, each giving the new release a different body.** For each: `PUT …/env-vars/MESSAGE`, then `POST …/deploys` `{"clearCache":"clear"}`. A poller fetched `GET /qa-poll` about once a second.
  1. `dep-dbpn2ilme2kc73f2ldh0`:
     - `update_in_progress` at 19:33:43;
     - `qa-lg36-v2` first answered at 19:34:00, alternated with `v1` until 19:34:05, and answered alone from 19:34:16;
     - `live` at 19:34:42, **42 s after the candidate first answered**.
  2. `dep-dbpn4pk5ngvc73e8u1t0`:
     - `update_in_progress` at 19:38:29;
     - `v3` first answered at 19:38:33, alternated with `v2` until 19:38:55, and answered alone from 19:39:07;
     - `live` at 19:39:41, **68 s early**;
     - its `Detected HTTP on port 10000` narration is stamped 19:39:41, the same second as `live`.
- **Errors.** Run 1 saw three connect timeouts (19:33:46–19:33:57). They didn't recur in run 2 or in 40 steady-state requests, so no downtime is claimed.

So every Docker deploy that produces a new digest re-enters discovery: every push to a git-built Docker service with no user-set `PORT`, and every image tag change. Poller output: `.playwright-mcp/qa-deploys-36-poll-a.txt` and `-b.txt` (local).

## Root cause

- **The candidate becomes Ready during discovery.**
  - An image web service without an explicit port (`PortModeWebImageV1`) re-enters HTTP discovery on every image change. `retainWebHTTPSelection` keeps a decision only for an identical digest-qualified image (`lego/operator/internal/controller/web_image_http.go:62-89`, `retainableWebHTTPSelection` `:93`).
  - The backend gives that mode to both git-built and prebuilt web services (`lego/backend/internal/apps/service.go:2734-2746`).
  - With a serving release, discovery gives the candidate a provisional HTTP `GET /` probe (`deployment_projection.go:351-358`, `webImageHTTPSelectionGuardsServing` `web_image_http.go:54-56`). nginx and the echo server pass it at once.
- **The hold keeps the old pods, but not the traffic.**
  - The Deployment's `MinReadySeconds` is set to `webImageHTTPSelectionHoldSeconds` (150 s; `deployment_projection.go:627-628`, `web_image_http.go:38-42`).
  - `MinReadySeconds` delays *availability*, not endpoint membership: a Ready pod joins the Service at once.
  - `privateServiceProjection` pins the selector to a revision only for port-changing rollouts (`image_network.go:316-325`, `changesImagePorts` `:353`, `replacedImagePorts` `:368`). Discovery leaves the selector on `labelApp`/`labelAppID`, so both releases serve.
- **Detection rolls the release a second time.**
  - `selectWebImageHTTP` returns "reproject probes and routing on the chosen port first" (`web_image_http.go:164`).
  - The template drops the provisional probe, and `MinReadySeconds` returns to 0. That rolls a second ReplicaSet, which becomes available at once, so the Deployment drains the serving release while the deploy is still in progress.
  - [w2/078](../../../w2/done/078.md) removed this second roll only for a first deploy (comment at `deployment_projection.go:353-357`).
- **Status lags traffic.** While the Service already routes only to the new revision, the operator reports `RolloutSettling` and keeps phase `Deploying` (`app_controller.go:3706-3715`). The backend counts `RolloutSettling` as serving for availability (`lego/backend/internal/store/reconciler.go:2210-2216`), but the row still waits to close `live`.
- **Cancel puts the prior release back by re-projecting it.**
  - `settleCanceledRelease` (`app_controller.go:789`) restores the prior release's snapshot.
  - The candidate's pods keep answering until that roll completes.
  - If the second roll had already drained the prior pods, the restore creates fresh ones.

## Target behavior

This is Render's documented sequence (render.com/docs/deploys, zero-downtime deploys): "your _original_ instance continues to receive all incoming traffic while the new instance is spinning up". Then: "Once the new instance becomes healthy, Render updates its networking configuration so that your _new_ instance begins receiving all incoming traffic."

- **While the deploy is not live, traffic stays on the serving release.** During HTTP discovery, every public and private Service selects only the serving revision's pods, whatever the candidate's readiness.
- **One step moves traffic and status.** When the candidate is chosen, routing moves to it and its deploy row turns `live` in that same step. Old pods then drain, as on Render.
- **An image change rolls once.** No second ReplicaSet drains the serving release before promotion. If discovery must change the template, the serving release keeps its pods and its traffic until promotion.
- **Cancel before promotion has nothing to undo.** The canceled candidate answered no request. The serving pods are the same pods before and after (same instance IDs in `GET …/instances` and in the logs).
- **REST, GraphQL, MCP and the dashboard keep their current shapes.** Only *when* rows become `live` or `canceled` changes. The cancel dialog's "If an earlier deploy succeeded, it stays live" becomes literally true.

## Definition of done

Live on production after the release pins this build, on an owned Free image web service in `bex-canary`: `docker.io/library/nginx:1.27-alpine`, no port. Run the Server-header poller from the Finding, with in-page deploy polling at ≤1.5 s.

- **No early traffic.** A hook deploy with `imgURL=…nginx%3A1.26-alpine`, left to finish:
  - every response is `nginx/1.27.5` until the poll that first reads the deploy `live`;
  - after that, every response is `nginx/1.26.3` within one poll interval plus Render's drain;
  - no alternation appears while the row is `update_in_progress`.
  - The same holds on a git-built `examples/hello-go` service with no user-set `PORT`: across `PUT …/env-vars/MESSAGE` + `POST …/deploys {"clearCache":"clear"}`, the new body never answers before the row reads `live`.
- **One instance per change.** `GET /v1/logs` for the deploy window shows exactly one new instance for that deploy, and the deploy log keeps its "Detected HTTP on port 80" narration.
- **Cancel before promotion.** Trigger the next tag with the hook and cancel it with `POST …/deploys/{id}/cancel` before it is live. Then:
  - not one response names the candidate's version, before or after the cancel;
  - `GET …/instances` lists the same instance ID before and after;
  - the logs show no new instance for the prior release.
  - Repeat once through the dashboard's Cancel → Proceed.
- **Same-image redeploy control.** The row turns `live` within one poll of the instance switch seen in the logs; today the gap is 24 s.
- **Unchanged neighbours.** A port-changing image deploy (w4/428), an env save and a restart still roll with no response error.
- **Image digest (t011, added from loop round 133).** Each image change goes live with its own `image.sha`: an `imageUrl` deploy, a saved-image change plus a standard deploy, and a rollback to another digest. Today both read none, though the create and a same-image redeploy carry one. REST, GraphQL `imageSha` and the dashboard agree.
- **Supersede after promotion (t012, added from loop round 156).** Hook the next tag, and when the poller first reads the new version, hook another tag (repeat at +5 s and +12 s):
  - every poll answers `200`;
  - the version only moves forward;
  - the first deploy's row reads `live`, then `deactivated`, never `canceled`.
  - The same on a first deploy (round 157): `POST …/deploys {}` the moment a new service first answers leaves the first row `live`, then `deactivated`. When the second deploy fails, the first stays `live` with a Rollback action.
- **Checks.** The operator suite (`make test`) and lint pass.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Unverified (carried into tasks)

- **What exactly closes the row `live` 24 s after the switch.** Deployment status convergence, the operator requeue or the backend reconcile period. Not traced (t004).
- **Network errors during the runs.** The poller saw a few curl errors in Run B and in a later uncontrolled run. My own machine lost connectivity in that window (the dashboard console logged `ERR_INTERNET_DISCONNECTED`), so no downtime is claimed. t006 re-checks for errors with a stable client.
- **Multi-instance services.** Paid plans weren't exercised. The pin must hold for every replica (t005).

## Dedupe

- **[w8/m141](../../../w8/done/m141/README.md)** (done) built HTTP discovery and the 150 s hold. Its DoD says "Pending discovery cannot promote a binary TCP listener or drain the prior release". The second roll now drains the prior release before promotion, so the drain half of that guarantee regressed. Its other DoD bullets aren't touched: lowest HTTP port, no-EXPOSE fallback, per-digest reuse, narration, and the RabbitMQ/nginx hosted checks. t005 keeps their tests. Where traffic goes during discovery was never specified.
- **[w2/078](../../../w2/done/078.md)** (done) removed the second ReplicaSet for a first deploy only.
- **[w4/428](../../done/428.md)** (done) pinned routing for port-changing rollouts. That pin is the precedent for t001.
- **[w4/405](../../done/405.md)** (done) stamps `activeRevision` early so a superseded row reads `live`. It accepted "serving before Live" as correct; t004 revisits that against Render's sequence.
- **[w1/m152](../../../w1/done/m152/README.md)** (done) made cancel restore the prior configuration. It didn't cover which pods serve.
- `grep -rliE "second ReplicaSet|rolls twice|serv(es|ing) traffic before" .pm` finds no open item.
- **Not deploy lag:** `git log c12cd57facb1..origin/main` on `web_image_http.go`, `deployment_projection.go` and `image_network.go` is empty.
- No `.pm/DO_NOT_DO.md` entry applies.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 35, 2026-11-10 (the probes above; fixture deleted).
- **Goal linkage:**
  - [ADR004](../../../../docs/ADR004-app-deployment.md) (deploys and rollouts);
  - [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) (Render's zero-downtime sequence);
  - truthful product state: the deploy list says which release users are getting.
- **Expected outcome:** a CI pipeline that ships image tags through the deploy hook never exposes an unverified release early. Cancel means the change never reached users.
- **Why now:** the deploy hook with `imgURL` is the documented CI path for image services, so every such deploy hits this. A bad build reaches users while the dashboard still calls it "In Progress". The second roll also doubles deploy time (108 s against 46 s).
- **Render parity included:** the deploy states on REST, GraphQL, MCP and the dashboard change timing, and the cancel dialog's promise depends on it (t007).

## Post-release probe (QA loop round 121, 2026-11-11)

The release gate cleared at `6ee0197c7` (14:43 UTC): production is pinned to `f609163ada8e`, which contains `7dec58bb5`. A `/qa-find-bugs w4` round ran Run A once against it. This is a record for t006, not a replacement for it, and t006 stays todo.

- **Fixture:** Free web service `qa-20261111-r121-ngx` (`srv-dbq9qsfpvbns739s7fmg`) from `docker.io/library/nginx:1.27-alpine`, no port. Live at 16:54:41. Deleted afterwards (DELETE 204, then GET 404).
- **Run A.**
  - **Method:** `POST /v1/services/srv-dbq9qsfpvbns739s7fmg/deploys {"imageUrl":"docker.io/library/nginx:1.26-alpine"}` (`dep-dbq9rng3u30c73cf8rmg`). It shares the image-override path with the hook's `imgURL` but is not the hook.
  - **Polling:** a curl poller read the `Server` header every ~0.5 s, and the page polled the row every 1.2 s.
  - **Row:** `created` 16:54:54.8 → `update_in_progress` 16:55:22.8 → `live` 16:56:24.0 (`finishedAt` 16:56:22.96).
  - **Responses:** `200 nginx/1.27.5` until 16:55:58.7, then `200 nginx/1.26.3` on every poll. There was no alternation and no error.
- **Against the DoD:**
  - **"No early traffic" fails by about 24 s.** Every response was `nginx/1.26.3` from 16:55:58.7, while the row read `update_in_progress` until 16:56:24.0. The switch itself is now clean: one cut-over, no mixing, unlike round 35. But the deploy still reads live about 24 s after it holds all traffic. That is the same gap the same-image control bullet records ("today the gap is 24 s"), and t004's "reads `live` in the same pass" is not met live.
  - **"One instance per change" fails.**
    - `GET /v1/logs?…&type=app&startTime=2026-11-11T16:54:50Z` shows two new instances for this one deploy, each starting nginx: `…-sjrmnnbtrru601glogre` (first line 16:55:12.8) and `…-48s95tq3f6gt9au147rg` (first line 16:55:47.9).
    - `GET …/instances` afterwards lists only `…-48s95tq3f6gt9au147rg` (created 16:55:45). The traffic switch at 16:55:58.7 matches that second instance becoming Ready.
- **Not run this round:** Run B (cancel before promotion), the git-built case, the neighbours, and the hook `imgURL` path itself.
- **Run B (QA loop round 122, 17:09–17:11 UTC) passes through the hook path.**
  - **Fixture:** `qa-20261111-r122-ngx` (`srv-dbqa1snpvbns739s7ghg`), nginx `1.27-alpine`. Serving instance `…-q12q0dhvskrddhe094l4`, created 17:08:23. Deleted afterwards (DELETE 204, then GET 404).
  - **Steps:** the deploy hook with `&imgURL=docker.io%2Flibrary%2Fnginx%3A1.26-alpine` → 200, `dep-dbqa2og3u30c73cf8sig`. The row went `created` 17:09:55 → `update_in_progress` 17:10:11.5. `POST …/deploys/dep-dbqa2og3u30c73cf8sig/cancel` at 17:10:17.8 → 200, then the row read `canceled` at 17:10:19.2.
  - **Results:**
    - All 91 Server-header polls from 17:09:50 to 17:11:20 answered `200 nginx/1.27.5`. The candidate version never answered.
    - The candidate instance `…-3lf5tigv7uf7lgtu9c6u` (created 17:10:08; nginx started 17:10:10.7) showed in `GET …/instances` right after the cancel. By 17:11:30 the list was back to the serving instance alone.
  - **Not run:** the dashboard Cancel → Proceed repeat.
- **Git-built case (QA loop round 123, 17:22–17:29 UTC) fails the same way as Run A.**
  - **Fixture:** `qa-20261111-r123-go` (`srv-dbqa8u03u30c73cf8tb0`), built from `bex-co/bex` `examples/hello-go` with no user-set `PORT`, `MESSAGE=r123-one`. Deleted afterwards (DELETE 204, then GET 404).
  - **Steps:** `PUT …/env-vars/MESSAGE {"value":"r123-two"}`, then `POST …/deploys {"clearCache":"clear"}` at 17:25:58 → `dep-dbqaa9g3u30c73cf8tkg`. The row went `created` 17:26:08 → `build_in_progress` 17:26:11 → `update_in_progress` 17:27:12 → `live` (`finishedAt` 17:28:42.2).
  - **Responses** (body poller every ~0.5 s): `r123-one` until 17:28:07, then `r123-two` on every poll from 17:28:08.0, with no alternation. The new body answered about **34 s** before the row read `live`.
  - **Instances:**
    - `GET /v1/logs?…&type=app` shows a new instance `…-rtvu0adlfj393lchkli9` logging "hello-go listening on 10000: "r123-two"" at 17:27:08.
    - `GET …/instances` afterwards lists only `…-qtgrk9qi0a8fmhegt3a0`, created 17:27:41. That is two new instances for one deploy, as in Run A.

## Post-release probe (QA loop round 156, 2026-11-12)

Pin `8330744bd643` contains `7dec58bb5`, but not t011's `f24e6e7f7`.

- **Fixture:** Free web service `qa-20261112-r156-rb` (`srv-dbqhk3qv3e1c73bps6q0`), nginx from `1.26-alpine`, no port. Deleted afterwards (DELETE 204, then GET 404).
- **Clean deploys:** a hook `imgURL` deploy from a settled live release gave 59 of 59 polls `200`, with one switch. A dashboard rollback gave 58 of 58 `200`, with one switch.
- **Filed as t012: a second hook in the settling window.** Fired 5 s, 12 s and 15 s after the first new version answered.
  - Twice, the release that answered closed `canceled` ("Superseded by …"). A 503 `no available server` followed, and an **older** release then answered for about 60–70 s.
  - Once, the first row closed `live`, but every poll from 01:49:13 to 01:50:06 answered `503`.
  - Probes and timelines are in t012.
- **Not run:** the DoD's no-early-traffic and one-instance bullets. They are left for t006.

## Post-release probe addendum (QA loop round 157, 2026-11-12)

- **t012 widened.** It is a regression of [w4/405](../../done/405.md) and is not limited to image changes.
  - A busybox httpd service's **first** deploy, superseded by `POST …/deploys {}` in the second it first answered, closed `canceled` ~20 s later while still serving.
  - When the superseding deploy crash-looped (exit 3), the service read `Failed` everywhere, with no live row and no Rollback action, while its first release answered `200` throughout.
  - `releaseInFlight` never pins a prebuilt-image release, and the backend closes such a row `live` only when `activeRevision` names it.
  - Probes are in t012's round 157 addendum. All three fixtures were deleted (GET 404).

## Post-release probe addendum (QA loop round 161, 2026-11-12): t011's digest bullet, two of three arms

Pin `e7a56b07e0d7` contains `f24e6e7f7`.

- On `qa-20261112-r161-rbd` (`srv-dbqiraq140ds739rb7v0`, Free, `traefik/whoami`, deleted afterwards), every row carried `image.sha`:
  - three `imageUrl` deploys (v1.10.2 → v1.10.4);
  - the rollback `dep-dbqivki140ds739rb83g` to v1.10.3's digest (`sha256:43a68d10…`, live 03:19:39).
- **Not run:** the saved-image change plus a standard deploy, and the rest of this DoD (t006).

