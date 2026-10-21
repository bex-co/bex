# Finding — an env-group link on an Auto-Deploy-off service ships a release that no deploy records

Live `/qa-find-bugs w4` loop run s3g6, 2026-10-21 UTC, `https://dashboard.bex.co`, `muse.env` through the login helper, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), production pin `9cace6b6e25d`. All fixtures were owned by this run and deleted in-run: the two groups and the site read 404 by id, and the session was revoked.

**Severity:** major. The owner turned Auto-Deploy off, and the API answers that the service is pending (`pendingServiceIds`), yet the site is rebuilt and republished anyway. No deploy row, no `build_*` or `deploy_ended` event and no revision note records it. A manual deploy minted next is stranded at `created` and then recorded as `canceled` even though its content is what the site serves.

## Fixtures

- Static site `qa-20261021-s3g6-static` (`srv-dbc5dvu5u6js73eso3q0`).
  - Created by REST `POST /v1/services` from `https://github.com/bex-co/bex`, branch `main`, `rootDir: examples/static-site`, `publishPath: .`, `autoDeploy: "no"`.
  - Build command: `printf "grp=%s\n" "$QA_GRP" > v.txt`.
  - The served `/v.txt` therefore shows which group value each build saw.
- Group `qa-20261021-s3g6-grp` (`evg-dbc5dvpv5vjs73b9hbpg`), `QA_GRP=one`.
  - Created 0.5 s after the site, with `serviceIds: [site]`.
  - Answer: `201` with `"pendingServiceIds":["srv-dbc5dvu5u6js73eso3q0"]`.
- Group `qa-20261021-s3g6-grp2` (`evg-dbc5mde5u6js73eso420`), `QA_G2=x`, created later with `serviceIds: [site]`.
  - Answer: `201` with the same `pendingServiceIds`.

## Timeline (REST deploys list + events, served `/v.txt` via `curl`)

| UTC | What happened | Recorded as |
| --- | --- | --- |
| 06:09:03 | Site created. Deploy A `dep-dbc5dvu5u6js73eso3qg` (bexTrigger `create`) minted; group 1 linked while A is `queued` | A `queued` |
| 06:11:55–06:13:42 | A builds and goes live | A `live`; `build_started`, `build_ended`, `deploy_ended succeeded` |
| 06:14 | `GET /v.txt` → `grp=` (A's build ran before the link reached it) | — |
| 06:13:59 | Manual deploy B `dep-dbc5g9u5u6js73eso3s0`: `POST /v1/services/{id}/deploys {}` → `201 status:"created"` | B `created` |
| 06:14:17–06:14:29 | BuildKit lines for a build running `printf "grp=%s\n" "$QA_GRP"` appear under B's deploy page | — |
| 06:20:36 | `GET /v.txt` → **`grp=one`**; service reads `revision: rev-2`, `phase: Running` | B still `created`, `startedAt: null`, `updatedAt` = creation time |
| 06:22:30 | Manual deploy C `dep-dbc5k9hv5vjs73b9hbvg` → `201 status:"queued"` | — |
| 06:22:43 | — | B **`canceled`**, `finishedAt 06:22:43`, `"Superseded by dep-dbc5k9hv5vjs73b9hbvg"`, `startedAt` still null |
| 06:24:25 | C live | C `live` |
| 06:25:02–06:26:42 | Control manual deploy D `dep-dbc5lfhv5vjs73b9hc0g` minted while nothing else is open | `created → build_in_progress → live` (normal) |
| ~06:26:55 | `PUT /v1/env-groups/<grp1>/env-vars/QA_GRP {"value":"two"}` → 200; group 2 created and linked → `pendingServiceIds:[site]` | — |
| 06:27:02 / 06:28:41 | — | events `server_failed`, then `server_available` (no deploy id) |
| 06:28:48 | `GET /v.txt` → **`grp=two`** | **no deploy row**: the list still tops out at D `live`; `autoDeploy: "no"` |

The deploy list after the run is `D live`, `C deactivated`, `B canceled`, `A deactivated`. Nothing explains the 06:28 republish. Nothing says B's output served from about 06:14 to 06:24.

## Root cause

**1. The gated link edits the release identity.**

- `rollLinked` (`lego/backend/internal/envgroups/service.go:1131-1147`) is used for an Auto-Deploy-gated service (`autoDeployGated`, `:1125-1129`: `Spec.Repo != "" && !Spec.AutoDeploy`).
- In that case it does a bare `rollout.PatchAppLocked` that runs `mutate(a)`, which adds the group's Secret to `spec.envFromSecrets` / `filesFromSecrets`.
- Its own comment promises that "the pod keeps serving its current release and picks the values up on the owner's next deploy".
- But the operator's release identity includes those lists:
  - `EnvFromSecrets` and `FilesFromSecrets` in `releaseIdentityInput` (`lego/operator/internal/controller/release_identity.go:207-208`).
  - The native artifact identity too (`:174-175`, `:185`).
- So the patch changes the fingerprint, and the operator adopts a new release.
- `requestedReleaseGeneration` (`release_identity.go:672-676`) takes `app.Generation` because the stamped annotation is not newer. Nothing minted a deploy row for that generation.

**2. The rowless release strands the next real deploy.**

- B was minted at `previous+1` (`deploys/service.go:284-290`, `:796`, `:831`). Its row generation is one past the rowless release the operator is building.
- `recordDeploy` (`lego/backend/internal/store/reconciler.go:844-845`) advances a row only when its generation matches `status.releaseGeneration`.
- `supersededDeployStatus` (`reconciler.go:1647-1648`) returns "settled, no status" while the release is behind the row.
- So B sits at `created`, with no timestamps and no lifecycle facts, until a later deploy supersedes it (`:1610-1620`) or the 18-minute gate (`defaultDeployGateTimeout`, `:120`) closes it `canceled`.
- The B-page BuildKit lines are the rowless build shown by B's open log window (`logs/progress.go:384-388` filters only by `FinishedAt`). They are not B's own build.

The second link (06:26:55) reproduced the first half on an established site with no deploy open: a republish with no row at all.

## Control cases

- **D** (no link in flight) went `created → build_in_progress → live` with full events. Manual deploys are fine on their own.
- **A**, the site's first deploy, recorded correctly. The link happened while it was queued, and its build did not see the group value (`grp=`).

## Follow-up probe — a web service, link and unlink (run w7k2, 2026-10-21 UTC)

Fixture: a Docker web service `qa-20261021-w7k2-web` (`srv-dbc6199v5vjs73b9hc30`).

- Created from `https://github.com/bex-co/bex`, `rootDir: examples/hello-go`, `autoDeploy: "no"`, Free, own env `MESSAGE=own`.
- The app logs `hello-go listening on …` once per process start.
- Deploy `dep-dbc6199v5vjs73b9hc3g` went live at 06:54:55, at `rev-1`.

What happened:

- **Link.** At 06:55:14, `POST /v1/env-groups {name:qa-20261021-w7k2-grp, envVars:[QA_X=1], serviceIds:[web]}` → `201`, `pendingServiceIds:[web]`.
  - At 06:55:18 a new instance `…-41rjqnbsl932rehmnf3b` logged `hello-go listening`.
  - By 06:55:30 the service read `rev-2`.
  - The deploy list still held one row. Events show nothing after `deploy_ended` at 06:54:55.
- **Unlink.** At 06:58:53, `DELETE /v1/env-groups/<grp>/services/<web>` → `204`.
  - At 06:58:55 another instance start was logged.
  - The service read `rev-3`, and the deploy list still held one row.
- Group and service were deleted; both read 404 by id.

So the rowless release is not static-specific. It also covers unlink, which rolls the pods by design (`UnlinkService` "roll it") but records no deploy. Private, worker, cron and group delete (`detach`) remain unprobed.

## Unverified

- Whether the value-only `PUT` (`rollOne`, `service.go:1264-1267`, returns early for a gated service and touches no CR) contributed. It changes no spec field, so the 06:28 republish is attributed to group 2's link. Both writes landed within seconds.
- Private, worker and cron services with Auto-Deploy off. Web and unlink are confirmed in the follow-up probe above.
- Group delete (`detach`) on a gated service.
- Where the `server_failed` event at 06:27:02 came from on a static site during the rowless release.
- Render: "If you make changes to an environment group … Render kicks off a new deploy for every linked service that has autodeploys enabled". With Auto-Deploy off, nothing deploys until the owner deploys. This is quoted in `autoDeployGated`'s comment and was not re-fetched this run.

## Related

- `w8/m117` (blocked) is the same stranded-row mechanism (`supersededDeployStatus` behind-the-row), reached by canceling the newer of two deploys. This finding is a different trigger: a rowless release from a gated link.
- `w4/224` and `w4/m234` own the env-group "pending" reporting. They made the API say pending; this finding is that the operator does not honor it.
- `w1/092` was the original "Auto-Deploy off still ships" fix for group writes (restartedAt). Link writes still change the release identity.
