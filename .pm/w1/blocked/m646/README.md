# w1 · m646 — A rollback's Confirm Rollback page lists what the rollback changes, as Render's does

**Worker:** worker1 **Goal:** before a rollback, the dashboard compares the live deploy's configuration with the target deploy's, as Render's `rollbackComparison` does. It shows each changed command, health check path and instance count as live → target values, names each changed env var key, secret file and env group, warns about the plan, and never shows an env var value or a secret file's contents. **Status:** blocked (t001–t004 and t006–t008 done, shipped in `6e614db0b0`; t005's production walk on 2026-11-26 verified DoD 1–3 and found DoD 4 short at 390 px, fixed in `f0eaf57377`; t005 re-checks DoD 4 on a pin carrying it, then t009)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's rollback comparison](done/t001.md) — **DONE** | 30m | — |
| t002 | [GraphQL read: the live and target releases' configuration compared](done/t002.md) — **DONE** | 2h | t001 |
| t003 | [The Confirm Rollback page in Render's words](done/t003.md) — **DONE** | 1h45m | t002 |
| t004 | [Cron jobs, image-backed services and static sites](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [Production walk on a disposable service](t005.md) | 30m | t004 |
| t006 | [Render parity](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 20m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 45m | t006 |
| t009 | [Closeout](t009.md) | 10m | t008 |

## Definition of done

1. On production, take a disposable web service in `bex-canary` whose env var keys, one secret file and health check path changed after an earlier deploy. Rollback on that deploy opens its Confirm Rollback page (`…/deploys/<id>/rollback`), which lists each change in Render's rows and sentences, and no env var value or secret file content appears on the page or in the GraphQL response.
2. Rollback on a deploy with nothing changed since says "No configuration changes since this deploy."
3. A target whose release record was reclaimed or predates snapshots, and a live deploy without a record, both show Render's "Unable to load configuration for the current live deploy. Rolling back to this deploy could cause unanticipated configuration changes." rather than an empty diff. (Amended 2026-11-26: Render shows that one sentence when either snapshot is missing and has no separate unavailable state, so the separate state this item first asked for would have diverged.)
4. The comparison's loading frame matches its ready frame, and the route's pending skeleton matches the ready page, at 1440 and 390 px.

## Verified against Render (2026-11-26)

The orchestrator re-checked Render in a headed browser, signed in to a Render Pro workspace (a rollback confirmation was opened on a web service and cancelled), and against Render's public dashboard bundle. That corrected this item's first text, and the tasks follow the corrections ([render-artifacts/rollback-comparison.md](../../../../docs/render-artifacts/rollback-comparison.md)):

- **Not a modal.** Render's confirmation is a page section on the deploy page's `/rollback` route: "Confirm Rollback", the comparison, the notices, then "Rollback to this deploy" and Cancel. The deploy list's Rollback is a link to it.
- **Values are shown** for the Start, Pre-Deploy and Docker Command, Health Check Path and Instances rows (live → target). Only env var values and secret file contents are hidden.
- **Env vars** are a sorted KEY / VALUE table with `••••••••••••` values, not sentences.
- Render also has an **Instances** row, static-site wording for env groups, and the static and Auto-Deploy notices.
- A **missing snapshot on either side** shows Render's one "Unable to load configuration…" sentence (DoD 3 amended).

## Unblock (2026-11-26)

- **Affects:** t005, then t009.
- **Source:** verified evidence. Production's newest pin, `chore(deploy): pin platform images to 11695fb886cb` (`ec6211f2ca`), carries `6e614db0b0` (`git merge-base --is-ancestor 6e614db0b0 11695fb886cb` exits 0), so the gate `pin must carry 6e614db0b0` holds.
- **Next:** a worker walks t005 on production in `bex-canary` as the QA user, the 1440/390 px loading-vs-ready check of the Rollback page included, then closes with t009. No user action.

## Production walk (t005, 2026-11-26)

On production pin `11695fb886cb`, as the QA user. The disposable web service `qa-20261126-m646-rb` (disposable) in `bex-canary` ran `traefik/whoami:v1.10.3` with env vars `QA_ALPHA` and `QA_BETA`, secret file `qa-secret.txt` and health check path `/`. A save-only env patch then updated `QA_ALPHA`, deleted `QA_BETA`, added `QA_GAMMA` and changed the file's contents, and the health check path became `/health`. That deploy went live, then a manual deploy with no change followed. The service was deleted afterwards.

- **DoD 1 holds.** Rollback on the first deploy links to `…/deploys/<id>/rollback`. Its Confirm Rollback page lists Health Check Path (`- /health`, `+ /`), the Environment Variables table (`QA_ALPHA` on both lines, `QA_BETA` on `-`, `QA_GAMMA` on `+`, every value `••••••••••••`) and "Secret file `qa-secret.txt` was changed after this deploy. The rollback will revert to the original contents." No env var value or file content appears in the page's HTML or in the `RollbackComparison` response, which carries key and file names only. Cancel returns to the deploy page and dispatches nothing. At 390 px the page has no horizontal scroll.
- **DoD 2 holds.** Rollback on the second deploy, after the unchanged redeploy, says "No configuration changes since this deploy."
- **DoD 3 holds.** A 2026-08 deploy of a service in the `bex` workspace, whose release predates configuration records (opened and cancelled), shows "Unable to load configuration for the current live deploy. Rolling back to this deploy could cause unanticipated configuration changes." That service has Auto-Deploy on, and the page also shows Render's Auto-Deploy notice. The disposable service has it off and shows none.
- **DoD 4: the comparison's loading frame matches its ready frame.** With the `RollbackComparison` request held, the spinner frame is 148 px, as the empty and unavailable states are at both widths. The card is 231 px at 1440 and 251 px at 390, and the buttons do not move when the answer lands. A comparison with rows grows below the frame, as Render's does.
- **DoD 4: the route's pending skeleton did not match at 390 px.** With the route chunk held, the skeleton's Confirm Rollback card header is 81 px at both widths. The ready one is 81 px at 1440 but 101 px at 390, because the lead wraps onto a second line, so the comparison and buttons dropped 20 px when the page landed. Fixed in `f0eaf57377`: the skeleton now lays out the ready title, lead and button labels invisibly under its bars (`ReservedText`, `ReservedButton`).
- **Not a defect:** above the card, the deploy header skeleton reserves the fullest header (`deploy-page-skeletons.tsx`, shared with the deploy page). It has the id line and seven facts, so a header with fewer facts settles upward when the page lands: 27 px for the image deploy at 1440, 32 px at 390, and 55 px for the 2026-08 git deploy at 1440.

## Render parity (t006)

| Surface | Render | bex (w1/m646) | Verdict |
| --- | --- | --- | --- |
| Rollback control | link to `/<type>/<srv>/deploys/<dep>/rollback` | link to `/services/<srv>/deploys/<dep>/rollback` (a static site's under `/static`); Render's `/web/…/rollback` reaches it through the alias routes | matched |
| Page | deploy header without Rollback, then "Confirm Rollback" in place of the log; a deploy it cannot roll back to redirects to the deploy page | same | matched |
| Rows | Start, Pre-Deploy, Docker Command, Health Check Path, Instances (`-` live, `+` target, `none`), Environment Variables table, Environment Group, Secret Files, Plan | same, in Render's order and sentences | matched |
| Static site | env var, group and file rows in static wording | no build env record: Render's "Unable to load configuration…" sentence | partial: [w1/m660](../../m660/README.md) (promoted from w1/156), recorded in ADR018 |
| Cron job | its deploy page has no rollback route | bex's cron Rollback opens the same page | filed: [w1/157](../../157.md) |
| States | spinner in a `min-h-[9.25rem]` frame; "No configuration changes since this deploy."; "Unable to load configuration…" for an error or either missing snapshot | same | matched |
| Notices | static: redirects, rewrites and headers; Auto-Deploy warning when on | same | matched |
| Buttons | "Rollback to this deploy", Cancel back to the deploy, error under the buttons | same; bex keeps its permission recheck and protected-environment phrase | matched |
| Read | `rollbackComparison` with `serverPlans` | GraphQL `rollbackComparison(serviceId, deployId)` with Render's snapshot fields and `currentPlanLower` for the plan warning | matched (bex field for the plans) |
| REST, MCP | no comparison; rollback unchanged | no comparison; `POST /v1/services/{id}/rollback` and `rollback_deploy` unchanged | matched |

ADR018's Rollback row (`docs/ADR018/deploys.md`) records the page, the read and both known limits. The bex sentences the dialog had and Render's page lacks (the tag re-pull, "saved settings stay unchanged") are gone.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4` (2026-11-25), item 3. On the user's request it was checked in a headed browser the same day and filed to w1.
  - **Render's read** is in Render's public dashboard bundle (`server-*.js`):
    - the query is `rollbackComparison(serverId, deployId, ownerID) { currentLiveDeploy { serviceConfigSnapshot } deploy(id) { serviceConfigSnapshot } serverPlans }`;
    - the `ServiceConfigSnapshot` fields are `dockerCommand`, `runCommand`, `preDeployCommand`, `healthCheckPath`, `instances`, `plan`, `envGroups`, `deletedEnvGroupCount` and `envVarChanges`;
    - `envVarChanges` holds `createdVarKeys`, `deletedVarKeys`, `updatedVarKeys`, `createdFileKeys`, `deletedFileKeys` and `updatedFileKeys`.
  - **Render's dialog** is in `DeployContainer-*.js`:
    - title and lead: "Confirm Rollback" and "Rolling back reverts certain configuration changes.";
    - rows: Start Command, Pre-Deploy Command, Docker Command, Health Check Path, Environment Variables, Secret Files;
    - secret files: "Secret file … was added after this deploy. The rollback will ignore it.", "…will restore the file." and "…will revert to the original contents.";
    - env groups: "The rollback will unlink environment group …";
    - plan: "Your current plan has lower CPU and memory allocations than the plan originally used for this deploy…";
    - empty and error states: "No configuration changes since this deploy." and "Unable to load configuration for the current live deploy…";
    - button: "Rollback to this deploy".
  - **No feature flag:** neither chunk imports Render's `featureflags` hooks, so every Render workspace has this dialog.
  - **bex before this milestone:** checked on production on 2026-11-25, on bex's own `metrics-grafana`; the dialog was opened and cancelled. It says "Roll back to this deploy? The service will run this deploy's image (built from …) and retained configuration. Saved settings stay unchanged and apply with the next standard deploy. Auto-deploy will be turned off. Older deploys without retained configuration restore only the image." It shows no comparison (`dashboard/src/features/deploys/components/deploy-actions.tsx`, `services.eventsRollbackConfirm*` in `dashboard/src/features/services/locales/en.ts`).
  - **The data exists:** [ADR004 § Per-release configuration snapshots (w1/m152)](../../../../docs/ADR004-app-deployment.md#per-release-configuration-snapshots-w1m152) keeps a record (`ReleaseRecordName`, `<app>-podtemplate-rN`) and per-source env and file copies for 20 generations. w4/m145's `undeployedChanges` already compares saved sources with the active release's record.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first). This extends ADR018's Rollback row (`docs/ADR018/deploys.md`).
- **Expected outcome:** someone rolling back sees what reverts before confirming, as on Render, instead of discovering it from the next deploy's behavior.
- **Why now:** rollback restores retained configuration since w5/m107, so it does change configuration. The dialog is the only place to say what changes, and today it does not.
- **Render parity included:** GraphQL and dashboard change. Render has no REST or MCP comparison, and the parity task confirms that REST and MCP rollback are unchanged.
