# w3 · m555 — A cron job's header and actions are Render's: Trigger Run while it runs, the schedule in words, Manual Build on its Deploys page

**Worker:** worker3 **Goal:** every page of a running cron job carries Render's one header action, Trigger Run, which runs at once and opens the Logs on Live tail; the header says "Runs `<command>` <schedule in words>"; and a repo-backed cron job builds from its Deploys page's Manual Build, as on Render. **Status:** blocked (t001–t006 and t008–t010 done; t007 step 2 walked production on 2026-11-25 and found three defects, fixed in `a43d36790f`; its re-check waits for a pin that carries the fix)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's cron job header and actions from the walk and the bundle — **DONE** | 30m | — |
| [t002](done/t002.md) | Header: Trigger Run while the cron job runs, no Connect or Manual Deploy menu, and Render's disabled reasons — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | Trigger Run's click: no confirm, then Logs on Live tail; the Runs card loses its own button — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Header facts: "Runs `<command>` <schedule in words>" and "Last successful run <date>" — **DONE** | 45m | t001 |
| [t005](done/t005.md) | Deploys page: Manual Build for a repo-backed cron job, and Render's "Search deploys and commits" on every Deploys page — **DONE** | 1h | t001, w3/m554/t003 |
| [t006](done/t006.md) | Skeletons: the header's action slot and facts line, and the Deploys toolbar — **DONE** | 30m | t002, t004, t005 |
| [t007](t007.md) | Verify in a real browser at 1440 and 390, on dev-3 and on production | 45m | t003, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 30m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 7h10m**.

## What Render does

- Evidence: a signed-in, read-only walk of four cron jobs in a Render workspace on 2026-11-21 (nothing that acts was clicked), plus Render's public dashboard bundle read the same day without sign-in (`CronJob-D3WeWa8V.js` sha256 `23d3ad7d7c97…`, `index-BR2q8yex.js` `f19b12ced9c1…`, `urls-B5ihPFNO.js` `3ab9b8053a93…`, `helpers-C_tFPHBn.js` `302444d539f3…`; the hashes change with every Render deploy, so find the chunks by name, as `metrics-page.md` § Status-code label describes).
- Walked: one header action, Trigger Run, while the cron job runs; none while suspended (a notice carries Resume service); the facts line "Runs `<command>` at 09:00 AM UTC" then "Last successful run <date>"; Manual Build beside "Search deploys and commits" on Deploys; a suspended cron job's disabled Trigger Run says "Resume this cron job to trigger a run."
- Bundle only: no confirm, `logs?r=live` after a trigger, the no-build and permission reasons, the two failure toasts, Manual Build's hidden and disabled states.

## bex today

- The cron header carries Connect, Resume while suspended, and the Manual Deploy menu (`service-detail-header.tsx`, `manual-deploy-button.tsx`).
- Trigger Run sits only in the Runs card's header, behind a confirm (w4/m114 t003), and stays on Runs.
- The header shows "Schedule: <expression>"; `describeCron` exists in `features/services/lib/cron.ts`.
- Deploys search reads "Search…".

## Live walk

### Local, against `yarn local-bex` (2026-11-22, t007 step 1)

**dev-3 could not run.** The shared kind/CAPD cluster's app cluster was not Available and CAPI kept replacing its workers: this host has `fs.inotify.max_user_instances` = 128, under the 512 `scripts/mock-cluster.sh` requires (root `AGENTS.md`, Local cluster workflow), and raising it needs root on the host. `bash scripts/dev-env.sh 3 status` answered nothing on any port. So the walk ran the dashboard from this tree (`vite dev`) against the repository's `local-bex` stub, in Google Chrome headless through `playwright-core`, `en-US`, zone America/Los_Angeles. A Playwright route answered what the stub lacks: the cron job `nightly-report` as repo-backed (`node` runtime, start command `node report.js`, `0 9 * * *`, last success 2026-11-22T09:00:12Z), suspended or not, with or without a live release; `ViewerCapabilities`, `ServerActions` and `DeployActions` allowed; `RunCronJob` answering a pending run. Pending frames were drawn by holding the `Server` read (header) and the Deploys route's chunk, then releasing them.

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| Header actions, running | Trigger Run alone (1440 at x 1315, 101×32; 390 at y 122, 101×44); no Connect, no Manual Deploy | holds |
| Trigger Run's click | no dialog, one `RunCronJob`, landed on `/services/nightly-report/logs?range=live` with Live tail selected; the Runs page then had one Trigger Run (the header's) and no confirm | holds |
| No build | repo-backed with an empty revision: Trigger Run disabled, tooltip "No successful build detected. View Deploys." | holds |
| Suspended | no action; the notice "Manually suspended You are not billed for suspended services. Resume service" (1184×46 at 1440, 358×102 at 390) | holds |
| Facts | "Runs `node report.js` at 09:00 AM UTC" and "Last successful run November 22, 2026 at 1:00 AM" (the viewer's zone), no next run | holds |
| Header pending against ready | band 189/189 px at 1440 and 209/209 at 390 (running), 239/239 and 255/255 (suspended); the action slot, both facts lines and the notice at the same boxes. The first pass at 390 differed by 30–42 px: the frame's status row wrapped onto a second line where the ready one did not, and the facts' second line wrapped beside Details; t006 fixed both | holds |
| Deploys | "Search deploys and commits" on the cron job's and the web service's pages; Manual Build only on the cron job's (1440: x 1303 after the search and filter; 390: on the count's line, 113×44); the toolbar row 1136×36 at 1440 and 358×96 at 390 in both states; a click called `TriggerDeploy` once, with no dialog or menu, and opened `/services/nightly-report/deploys/<new deploy>` | holds |

No page errors at either width.

### Production (2026-11-25, t007 step 2)

Pins `7b046987bcd8` (07:31–07:39 UTC: the no-build state, the first Trigger Runs) and `9cc529d10432` (14:28–14:45 UTC: everything else, every point re-walked), both carrying `ad7f6e2b36`; the `dashboard` pod ran each pin's digest. As the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`), over REST: the repo-backed cron job `qa-20261125-m555-repo` (disposable, `srv-dc38tnpa7e1c7381dh0g`, `render-examples/express-hello-world` on `main`, Node, build `yarn`, command `node -e "console.log('m555 run ok')"`, `0 9 * * *`, Starter) and the image-backed `qa-20261125-m555-img` (disposable, `srv-dc38u0guqmis73a84kig`, `docker.io/library/busybox:1.36`, `echo m555-image-run-ok`, `0 9 * * *`, Starter). `hello-go` was the web service. Google Chrome headless through `playwright-core`, `en-US`, zone America/Los_Angeles, at 1440 and 390.

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| Header actions, running | Trigger Run alone on Runs, Deploys, Events, Logs and Settings of both cron jobs (1440 at x 1315, 101×32; 390 at y 152, 101×44); no Connect, no Manual Deploy. `hello-go` kept Connect and Manual Deploy | holds |
| No build | the repo-backed cron job during its first build (Latest deploy Building): Trigger Run disabled, tooltip "No successful build detected. View Deploys." linking `/services/<id>/deploys` | holds |
| Trigger Run's click | four clicks (each cron job at each width): no dialog, one `RunCronJob`, `/services/<id>/logs?range=live` within 340 ms with Live tail selected, then the run's own line (`m555 run ok`, `m555-image-run-ok`) streamed in 13–21 s with no reload. Afterwards the Runs page had one Trigger Run, the header's | holds |
| Facts | "Runs `node -e "console.log('m555 run ok')"` at 09:00 AM UTC" (and `echo m555-image-run-ok`), "No successful runs yet." before a run, then "Last successful run November 25, 2026 at 1:00 AM" after the 09:00 UTC run (the viewer's zone) and 6:30 AM after a triggered one | holds at 1440; at 390 the line wrapped, see below |
| Suspended | suspended over REST: no action; "Manually suspended You are not billed for suspended services. Resume service" (1136×46 at 1440, 358×102 at 390) on Runs, Deploys and Settings. Resume service in the notice resumed it at once (`ResumeService`, no dialog) and Trigger Run came back | holds |
| Deploys | "Search deploys and commits" on both cron jobs' and `hello-go`'s pages; Manual Build only on the repo-backed one (1440: x 1305 after the search and filter, 111×32; 390: on the count's line, 111×44), absent on the image-backed one; a click at each width called `TriggerDeploy` once with no menu or dialog and opened `/services/<id>/deploys/<new deploy>`, which built from `039c347` and went live | holds |
| Deploys toolbar pending against ready | the route's chunk (`services._serviceId.deploys.index-*.js`) held on a sidebar click from Runs, then released: toolbar 1136×36 and Manual Build 111×32 at 1440, 358×96 and 111×44 at 390, the same in both states | holds |
| Header pending against ready | the `Server` read held on a click from the workspace's service list, then released. 1440: running 189/189 px, suspended 239/239, `hello-go` 145/145. 390: running 209 against 239, suspended 255 against 297, `hello-go` 189 against 219 | **fails at 390** |
| Page errors | React #418 on every load of either cron job once it had a successful run (20 of 20 loads), none before its first run and none on `hello-go` | **fails** |

**Three defects, fixed in `a43d36790f`:**

- **React #418.** "Last successful run <date>" sits in a `<p>`, and `LocalDateTime`'s pre-hydration placeholder was a `<div>`, which the HTML parser moves out of a paragraph. The server's HTML carried `<p …cron-last-run>Last successful run <time …><span><div data-slot="skeleton">`. The local walk ran `vite dev`, whose server render drew the header's loading frame, so it never hydrated this markup. The placeholder is now phrasing content.
- **The name row at 390.** The frame drew no "Latest deploy" item, so its row fit one line while every real service's ready row wraps (t006 had sized it to the local stub, which has no deploys). It now reserves "Service" and "Latest deploy" in their own words and lays the cached name out invisibly at the heading's size.
- **The facts line at 390.** "Runs `<command>` at 09:00 / AM UTC" wrapped while the command still had room to truncate. The command now gives up its width first; the words never shrink, and from `sm` the Runs line is its own line, so the frame no longer depends on the command's length. An empty action slot (a suspended cron job's) is hidden, so it adds no row gap.

Checked locally against a copy of `local-bex` serving a cron job with a long name, a long command, a last successful run and a deploy (running and suspended), and the web service, the dashboard from this tree on `vite dev`: the header frame equals the ready band at 390, 640, 768, 1024 and 1440 for both cron states and at 390, 768, 1024 and 1440 for the web service (at 640 the web service's frame is 22 px taller, as it was before this change: filed as w3/112). The hydration tests in `local-time.test.tsx` and `service-detail-header.test.tsx` fail without the fix.

**Cleanup:** both cron jobs were deleted (`DELETE` 204; by-id `GET` 404 for each and for the last build); the app cluster kept no App, CronJob, Job or Pod of either. The QA session was logged out (`qa-login.sh --logout`, after which its cookie answered 401), and the session jar, the state file and the fetched kubeconfig were deleted.

**Left for the next pin** (t007 step 2's re-check, once production carries `a43d36790f`): as the QA user in `bex-canary`, with a cron job created for it (disposable) that has run once: no page error on its pages; the header's pending frame equal to the ready band at 390 and 1440, running and suspended, and for `hello-go`; the facts line on one line at 390.

Found on the way, not this milestone's: at 390 a building deploy's Cancel sits over "Created 1 minute ago" on Deploys, filed as w3/111.

## Definition of done

1. A running cron job's header shows Trigger Run on every cron page and nothing else; a suspended one's shows no action and Render's suspension notice with Resume service; other types' headers are unchanged.
2. Trigger Run is disabled with Render's reason when suspended, when a repo-backed cron job has no successful build, and when refused; a click runs at once and opens the Logs on Live tail; a failure toasts until dismissed. No other Trigger Run or confirm remains on the Runs page.
3. The header reads "Runs `<command>` <schedule in words>" and "Last successful run <date>".
4. A repo-backed cron job's Deploys page has Manual Build (hidden for an image-backed one), which builds and opens the new build; every Deploys page's search reads "Search deploys and commits".
5. Pending and ready match at 1440 and 390; walked on dev-3 and on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-21), filed by `/pmx the verified parity to w3`, item 2.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a cron job is run and built from the places Render puts those actions, and its header says what it runs and when in words.
- **Why now:** it follows w3/m554's routes (Manual Build lives on the Deploys page m554 restores), and w3/m556's empty states reuse the Trigger Run this milestone builds.
- **Open decision, recorded as a recommendation:** dropping the confirm w4/m114 t003 added follows Render's bundle. That is the proposal's recommendation under ADR008's parity-first rule (parity work is not parked on an optional decision), not a user decision. Keeping the confirm would be a divergence awaiting the user's sign-off in ADR018. Checking the click on Render needs a disposable Render cron job (at least $1 a month), the user's call; until then t001 records the click as bundle-only.
- **Render parity task included:** the dashboard's cron header and Deploys page change. No REST, GraphQL or MCP surface changes.

## Unblock (2026-11-24)

- **Affected:** t007 step 2, then t011.
- **Source:** verified evidence (`/loopx w3` dispatch, gate check).
  - `git merge-base --is-ancestor ad7f6e2b36 9fdf7e0a1171` holds. The newest `chore(deploy): pin platform images to …` on origin/main is now `4b533337ab`, which pins `7b046987bcd8` (deploy run 45545485172, green), and it carries `ad7f6e2b36` too.
  - On the production app cluster the `dashboard` Deployment and its one pod run that pin's digest (`bex-dashboard@sha256:0ae406bbdc60…`, pod started 2026-11-25 04:45 UTC).
- **Gate:** the `pin must carry` gate is cleared; nothing else gates the walk. The QA user's sign-in comes from `scripts/qa-login.sh`.
- **Next:** t007 step 2 as § Live walk › Production describes, recorded there; then t011 closes the milestone if every point holds.

## Park (2026-11-25)

- **Affected:** t007 step 2, then t011.
- **Source:** verified evidence: the production walk above.
- **Finding:** every point held except two: React #418 on every cron job page once it had a successful run, and the header's loading frame 30–42 px short of the ready band at 390. Both are this milestone's (t004's facts line, t006's frame) and were fixed and tested in `a43d36790f`, with a local browser check at five widths.
- **Gate:** production must run `a43d36790f` (`pin must carry`); nothing is needed from the user.
- **Next:** t007 step 2's re-check as § Live walk › Production lists it, recorded there; then t011 closes the milestone if it holds.
