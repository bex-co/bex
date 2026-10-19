# An overlapping deploy serves while recorded canceled; Restart then reverts to an older release

- **Severity:** major. Deploy history and `status.activeRevision` misreport what is serving for the whole length of the newer build, and a Restart in that window silently puts an **older** release back in service and drops the saved change.
- **Versions:** Bex checkout build `v0.3.3+d5c3373ed` (no `lego/cli` change since `bex-cli/v0.3.3`); upstream pin Render CLI v2.27.0 `a764810a7682`. Deployed platform pin `e6457568b47e`: no operator/backend change between it and `main` other than the m116 logs fix.
- **Context:** 2026-10-19 00:05–00:31 UTC, `https://api.bex.co/v1/`, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device-login token, non-TTY `-o json`. App CR status was read read-only through the `hetzner-prod` context, for the owned fixture only.
- **Fixture:** `qa-20261018-6f4ee6-logs` (`srv-dbalqkm804hs73aaec20`): Free native Python web service built from `render-examples/flask-hello-world@main`, build command `true`. Created and deleted by this run (detail and URL return 404 after delete).
- **Attribution:** platform (operator release tracking + backend deploy-row projection). Not the launcher and not upstream: the pinned CLI only sends `PATCH /v1/services/{id}` and `POST /v1/services/{id}/restart`, and reads `GET /v1/services/{id}/deploys`.

## Repro

Each version writes a marker file that the public URL serves, so the version actually serving can be read from outside the platform:

```bash
bex services create --name qa-<date>-<nonce>-logs --type web_service --runtime python \
  --repo https://github.com/render-examples/flask-hello-world --branch main \
  --build-command true --start-command 'python -u -m http.server $PORT' \
  --plan free --region frankfurt -o json --confirm
# wait for the first deploy to be live, then, back to back (about 1s apart):
for v in A B; do
  bex services update <srv-id> -o json --confirm \
    --start-command "sh -c 'echo $v > /tmp/v.txt; cd /tmp && exec python -u -m http.server \$PORT'"
done
# poll every 1-5s:
bex deploys list <srv-id> -o json
curl -s https://<slug>.onbex.co/v.txt
```

Expected, from Render's documented default **Wait** overlap policy ("Allow the in-progress deploy to finish, then proceed directly to the most recently triggered deploy", <https://render.com/docs/deploys>) and from bex's own ADR018 claim (w2/018: "an executing generation reaches `live` before the later generation deactivates it"):

- A becomes `live` once A serves, and the previous deploy becomes `deactivated` at that moment.
- B leaves `queued`, builds, becomes `live`, and A becomes `deactivated`.
- At every instant, the single `live` row is the release the URL serves.

Actual (reproduced three times; the CLI exits 0 every time):

| Repro | Superseded deploy (row) | Row canceled at | Its code served | Row shown `live` meanwhile |
| --- | --- | --- | --- | --- |
| 1 (A→B) | `dep-dbaltiu804hs73aaecg0` | 00:07:10 | 00:07:06 pod start, 00:07:23–00:08:32 all traffic | original `dep-dbalqkm804hs73aaec2g` until 00:08:36 |
| 2 (C→D) | `…aecng` | 00:12:57 | 00:12:53–00:14:23 | B's row until 00:14:28 |
| 3 (E→F) | `…aed6g` | 00:17:29 | 00:18:13–00:18:59 | D's row until 00:19:05 |

Repro 3 CR timeline (1s polling; `relG` = `status.releaseGeneration`, `actRev` = `status.activeRevision`; `v` = public `/v.txt`):

```text
00:15:57 F:queued  E:created            D:live | v=D | g=7 ph=Building relG=6 pend=7 actRev=rev-5 Ready=False/Building
00:17:22 F:queued  E:build_in_progress  D:live | v=  | g=7 ph=Building relG=7 pend=-  actRev=rev-5   <- E's build resolved; F adopted at once
00:17:29 F:build_in_progress E:canceled D:live | v=D | g=7 ph=Building relG=7 actRev=rev-5
00:18:13 F:build_in_progress E:canceled D:live | v=E | g=7 ph=Building relG=7 actRev=rev-5         <- E's pods serve; nothing records it
00:18:48 F:build_in_progress E:canceled D:live | v=E | g=7 ph=Deploying RolloutProgressing
00:18:59 F:build_in_progress E:canceled D:live | v=F | g=7 ph=Running relG=7 actRev=rev-7
00:19:05 F:live            E:canceled D:deactivated | v=F
```

`GET /v1/services/{id}/deploys` for repro 1: A `canceled`, created 00:05:31, started 00:05:40, finished 00:07:10. The original deploy is `deactivated`, with `updatedAt` 00:08:36 (the moment B went live). A never has a `live` interval.

### Restart in the window (repro 4)

Same fixture, G then H back to back (00:20:46 / 00:20:49). Once the URL served G (00:22:34; G's row was already `canceled` at 00:22:30, H was building, `actRev=rev-7`=F), `bex restart <srv-id> --confirm` was run at 00:22:39 and exited 0 with `"<srv-id> restarted successfully"`:

```text
00:22:34 H:build_in_progress G:canceled F:live | v=G | relG=9 actRev=rev-7
00:22:39 >>> bex restart
00:22:46 R:queued H:canceled G:canceled        | v=G | g=10 ph=Deploying relG=10 actRev=rev-7
00:22:51 R:queued H:canceled                   | v=G | ph=Running actRev=rev-10
00:22:55                                       | v=F                       <- reverted to F
00:23:00 R:live H:canceled G:canceled          | v=F
00:31:21 spec startCommand = H's; URL serves F; no open deploy; ph=Running actRev=rev-10
```

Final deploy rows: `dep-dbam5ju804hs73aaedtg` live (trigger `api`); H `dep-dbam4oe804hs73aaedmg` `canceled`, finished 00:22:40, with an empty `failureReason`; G `dep-dbam4nmhvrac73cbstpg` `canceled`. The saved H configuration is not serving and nothing is pending. A user sees a successful Restart and a live service while the code they last saved never deploys, and the release that was actually serving (G) was replaced by an older one (F).

### Restart during a single in-flight deploy, no overlap (repro 5)

This repro isolates the Restart half from the overlap half. Fresh fixture `qa-20261019-085414-rst` (`srv-dbami4ehvrac73cbt0c0`, same shape). First, a control: `bex restart` while the **first** deploy was still `queued` was refused correctly with exit 1 and `409 conflict: service "<id>" has no live deploy to restart; deploy it first`. After the first deploy went live (served `first-…`), one `bex services update --start-command` (X) opened one deploy. With X `build_in_progress` and nothing else open, `bex restart --confirm` at 00:53:05 exited 0:

```text
00:52:55 X:build_in_progress first:live | v=first | g=2 ph=Building relG=2 actRev=rev-1
00:53:05 >>> bex restart  ("<srv-id> restarted successfully")
00:53:10 R:queued X:build_in_progress first:live | v=first | g=3 ph=Building relG=2
00:53:22 R:update_in_progress X:canceled first:live | v=first | g=3 ph=Deploying relG=3
00:53:53 R:live X:canceled first:deactivated | v=first | ph=Running actRev=rev-3
00:55:31 spec startCommand = X's; URL serves first; no open deploy
```

The events record `build_ended`/`deploy_ended` for X with `cancelReason: "Superseded by dep-dbamjsc8igjc73ahq060"` (the Restart), while X's deploy row has an empty `failureReason`. GraphQL `server(id){undeployedChanges}` reads `true`, so the dashboard discloses the saved/runtime difference; the CLI shows nothing. So, independent of the overlap defect, a Restart **cancels the user's in-flight deploy** (Render's Override policy) rather than waiting behind it (the Wait policy ADR018 claims for overlapping triggers; Render describes a restart as "a special form of manual deploy", <https://render.com/docs/deploys#restarting-a-service>). "Superseded" is also misleading here: a Restart deliberately reuses the running configuration, so it never carries the saved change it supersedes. This is t004's decision.

## Root cause (traced)

1. **The operator releases the run-to-completion pin as soon as the build resolves, not when the pinned release serves.** `lego/operator/internal/controller/release_identity.go:370` `buildRunning` is true only while `status.phase == Building`. In `prepareAppReleaseDecision` (`:400`), the coalescing branch at `:437` therefore pins only during the build. On the first reconcile after the build Job settles, the function falls through, computes the newer spec's `desired` identity, sets `ReleaseGeneration = requestedReleaseGeneration(app)` (`:439`), and starts the next build. Its own doc comment (`:424-436`) says the pending slot is "picked up on the next reconcile once this build resolves (phase leaves Building)".
2. **The pinned release still rolls out, but is never recorded as served.** Its Deployment template was already applied (`app_controller.go:2506`), so its pods replace the old ones. `markRunning` (`app_controller.go:3043`), the only writer of `status.activeRevision`, is reached from `reportKubernetesRunning` (`:2963`, call at `:2987`) only for the current release, which is now the newer generation still building. `activeRevision` therefore keeps naming the release before the one serving (`rev-5` while E serves; `rev-7` while G serves).
3. **The backend has no state in which it could observe the executing row live.** The CR never reports Ready at the executing generation. `lego/backend/internal/store/reconciler.go:1529` (`supersededDeployStatus`, `:1500`; comment at `:1520-1528`) sees `status.releaseGeneration > open.Generation` and closes the row `canceled`. Its comment calls this the "missed-observation fallback" and assumes that "under normal overlap handling the active row is observed live before the queued row is adopted". With the operator behavior in (1), that never happens.
4. **Restart trusts the stale revision.** `lego/backend/internal/deploys/rollback_config.go:202-232` `servingDeploy` (w4/m201) resolves `status.activeRevision` to its deploy row, which is still SQL-`live` (F), and restarts that release's configuration. The restart's release generation (10) then passes H's (9), so H's open row is closed `canceled` by the same `reconciler.go:1529` branch with no reason, and the operator abandons H's build.
5. **Why tests pass:** `lego/operator/internal/controller/build_supersede_envtest_test.go:99` asserts the current behavior: gen-3 is adopted right after gen-1's build, with no Running/`activeRevision` assertion for gen-1, and envtest has no kubelet to roll pods. The backend tests `deploys_test.go:146` (`TestLiveTransitionDeactivatesPriorLiveDeploy`) and `:177` (`TestCreateDeployKeepsActiveAndQueuesLatestOverlap`) feed the reconciler a CR that reports the executing generation Ready, a state the real operator never produces.

## Fix target

- The executing release that serves is recorded as served. Either:
  - the operator keeps the latest-pending slot pinned through the pinned release's rollout until it is Running with `activeRevision = rev-<its generation>` (or its rollout settles failed), then adopts the pending generation (Render's Wait policy); or
  - after adopting the pending generation, it still stamps `activeRevision` when the Deployment's pinned revision completes, and the backend closes the executing row `live` from that evidence instead of `canceled`.

  t001 chooses between these. Either way: the executing row reaches `live` and the previous row `deactivated` when its pods serve, then the executing row is `deactivated` when the newer row goes `live`.
- A `canceled` row's release never serves traffic. A row that served is never reported `canceled`.
- Restart during the window restarts the release actually serving, and never puts an older release back. Whether Restart may cancel an in-flight deploy is decided against Render (t004). If it does, the canceled row carries a `failureReason` naming the restart, and the undeployed saved configuration stays visibly pending; it must not be silently dropped.
- Neighbors unchanged: user Cancel (m117 territory), a genuine queued-row replacement (w2/018 cancels only the replaced queued row), build failure of the executing row (`build_failed`, w6/m100), prebuilt-image and direct-static-publish Apps (never pinned).

## Blast radius (to be confirmed by t005)

- Every repo-backed App type that pins (`buildRunning`: `Spec.Repo != ""`, no `Spec.Image`, not suspended, not direct static publish): native and Docker web, private, worker, cron, and source-built static services.
- **Observed control (sweep s7, 2026-10-19 01:10–01:13 UTC):** a source-built static site (`srv-dbampu823mpc73e3rokg`, `--publish-directory out`, two back-to-back `--build-command` updates SA→SB) stays truthful. SA's release was adopted away during its `Publishing` phase, never served (the URL went S0 → SB), and its row is honestly `canceled`. Static publish therefore does not share the serving window, though it passes through the same adoption point. t005 should explain why: the publish switch is atomic, while a Deployment rollout continues after adoption.
- **Observed control (sweep s8, 2026-10-19 01:29 UTC):** a prebuilt-image web service (`traefik/whoami`, two back-to-back `bex deploys create --image` v1.10.2 then v1.10.3). It is never pinned: the operator went straight to the newest generation, the intermediate release never rolled out, and its row is honestly `canceled`. The image path is unaffected.
- Every overlapping trigger: CLI/REST/GraphQL/MCP `services update` of release-identity fields, manual `deploys create`, Git pushes with auto-deploy, env var / env-group / secret-file changes that open a deploy, Blueprint syncs, deploy hook.
- `status.activeRevision` readers: Restart (`rollback_config.go:204`), `apps/service.go:1120` (`Revision`), the dashboard's current-revision display, and the operator's `successfulReleaseGeneration` / `releaseHasServed` (`release_identity.go:475`, `:544`), which decide what a cancel or failed rollout restores.

## Unverified

- Rollback (explicit) and suspend/resume during the window were not exercised.
- Whether a static or cron fixture shows the same window was not exercised (same `buildRunning` path by code).
- The one empty `/v.txt` sample at the adoption instant (repro 3, 00:17:22) is a single 3s-timeout sample; it is not claimed as downtime.
- Render's exact behavior for a Restart while a deploy is in progress is not documented on the deploys page.

## Dedupe

- `.pm` open and done: w2/018 (resolved 2026-08-16) claims this exact guarantee, so this is a regression of its premise. It is not covered by w4/m148 (Blueprint first deploy plus group link, different trigger), w4/m201 (Restart selection; its fix trusts the `activeRevision` that this bug leaves stale), or w8/m117 (canceling the newer of two open deploys strands the older; fixed in `84f83ab08`, blocked awaiting deploy; its `survivorInFlight` pins through rollout only on the cancel path). m117 touches the same overlap state machine; coordinate the two so one fix does not regress the other's DoD.
- `git log -S "func buildRunning"`: introduced by `5b8d7e30c` (w2/m72); no later change. No fix on `main` awaiting deploy.
