# w4 · m198 — Include continued pre-deploy output after cancellation

**Worker:** worker4 **Goal:** a canceled deploy's default log view includes its own pre-deploy step's later completion/error output while retaining the correct deploy cancellation time. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resolve the matching pre-deploy end fact — **DONE** | 35m | — |
| t002 | Include the finished step in the canceled deploy's default log window — **DONE** | 25m | w4/m198/t001 |
| t003 | Audit shared consumers and preserve window controls — **DONE** | 20m | w4/m198/t001, w4/m198/t002 |
| t004 | Render parity and API/UI contract verification — **DONE** | 20m | w4/m198/t003 |
| t005 | Simplify changed code — **DONE** | 15m | w4/m198/t004 |
| t006 | Test coverage for canceled-step diagnostic windows — **DONE** | 30m | w4/m198/t004, w4/m198/t005 |
| t007 | Closeout with the live success and error replays — **DONE** | 15m | w4/m198/t006 |

## Definition of done

Replay the observed journeys on an owned, disposable Free image web service. The exact requests, responses, commands and timing controls are in [finding.md](finding.md).

- Create the BusyBox HTTP marker fixture, verify its first successful release serves HTTP 200, and stage the other BusyBox version. Save the 45-second successful Pre-Deploy Command, cancel its carrying deploy while the step runs, and let the step finish. The deploy remains canceled and its step says succeeded. Fresh desktop 1440×1000 and mobile 390×844 detail pages with the default Deploy window show the final completion marker once, including when searched.
- Repeat the 35-second exit-23 command and cancel while it runs. After its step ends, the deploy stays canceled and its step says failed. The default All logs search shows the final error marker once; Last 15 minutes and returning to Deploy window also show that same row. Deploy finishedAt and duration still describe cancellation, rather than being extended to migration completion.
- Re-run the uncanceled exit-23 control: pre_deploy_failed, actionable exit-code reason, retained service Running, and its marker present in All/Build/Application buckets. Query the exact canceled-step probes: final output lies after cancellation and inside the matching pre_deploy_ended bound. The implementation uses that deploy's fact, with no event or output borrowed from another deploy.
- Public samples through failed/canceled steps keep HTTP 200 on the original image, as observed. Clear Pre-Deploy Command and wait for recovery: the staged image is now served, the new deploy is Live, and both pending-change flags clear. This does not replace w4/m181's separate EndpointSlice/90-second closeout.
- Delete only the fixture, verify REST 404 and restoration of all six starting resource ID inventories, revoke only the replay session, and verify whoami 401/zero browser cookies. Close this milestone after the observable live result, rather than when code merely exists.

Compatibility classes not exercised live in the source pass—historical event paging, true stopped/superseded steps, suspend and first release, paid worker/private, cron/static/datastore siblings, legacy aliases, Chinese and Render UI—belong to t003/t004/t006 verification. They are not additional observed failures.

## Source + Goal linkage

- **Source:** functional qa-find-bugs infinite loop, 2026-10-10 local / 2026-10-11 UTC, pass a147, normal signed-in QA user from muse.env. [finding.md](finding.md) contains durable complete API captures and verified local evidence. Owned service and session fully cleaned up.
- **Goal linkage:** ADR008 reliable hosting; ADR004 deliberate migration completion after Cancel; ADR006 distinct deploy and step lifecycle timestamps; ADR010 diagnostic logs; ADR018 deployment parity.
- **Expected outcome:** the default canceled-deploy log range includes its own continued pre-deploy step's complete observed output. It retains accurate cancellation times, serving-release preservation, cursor semantics, source buckets and user-selected ranges.
- **Why now:** w5/085 correctly records post-cancel step outcomes, but the log consumer still cuts off at the earlier deploy cancellation. Both success and error output disappear despite correct retained data and final verdicts. The wider-range workaround proves this is a window integration gap.
- **Render parity included:** a dashboard diagnostic surface changes. Prefer consuming the existing events API with no new schema; verify REST/GraphQL/MCP filters and aliases stay consistent. Render's exact authenticated post-cancel behavior was not observed, and bex's continued-step policy stays deliberate.
- **Estimate:** seven tasks, ~2h 40m (160m), including the standing closing tasks. This schedules the fix; it does not implement it.

## Closeout

Shipped `7d23ecb2c` (deploy.yml run 38137784517 green); live replay 2026-10-11 UTC on owned Free web fixture `srv-db5o89m02cuc73ca6kug` (`qa-20261011-m198-predeploy`).

- **Fix:** `useCanceledStepEnd` (dashboard/src/features/deploys/hooks/) resolves the deploy's own `pre_deploy_ended` via typed, cursor-paged `serviceEvents` over a frozen `[createdAt, updatedAt+60s]` interval matched on `details.deployId`; the default window ends at max(finishedAt, fact+1s). Loading state while checking; explicit notice (+ retry on read error) when the fact is missing/unreadable. `useDeploy` keeps polling a canceled deploy whose step still reads `canceled` for 12 min after finishedAt (10-min Job deadline + slack). No schema/API change; ADR004 records the policy.
- **Success replay:** `dep-db5o8oe02cuc73ca6l0g` canceled 12:43:32.16Z; completion marker 12:44:03.78Z; fact 12:44:06Z; deploy stays `canceled`, step `succeeded`, finishedAt unchanged. An already-open page picked up the late settlement; fresh 1440×1000 and 390×844 loads with search show the marker once, no "No matching logs".
- **Error replay:** `dep-db5o9td13j8s73csrcm0` canceled 12:45:52.70Z; error marker 12:46:18.15Z; fact 12:46:20Z; step `failed`, finishedAt unchanged. Default search, `?r=15m`, back to Deploy window (URL and Log-options menu, desktop + mobile) each show the marker once. Another deploy's fact (12:44:06Z) is ignored by construction (unit + integration tests).
- **Controls:** uncanceled exit-23 `dep-db5oaut13j8s73csrcp0` → `pre_deploy_failed`, reason "exited with code 23", service Running, marker once in All/Build/Application. 11 public samples during the failed/canceled steps: HTTP 200 BusyBox 1.37.0. Clearing the command → `dep-db5obit13j8s73csrcqg` Live, HTTP 200 BusyBox 1.36.1, `undeployedChanges`/`undeployedChangesApplying` false. Does not close w4/m181.
- **Audit (t003/t004):** one `DeployLogPanel` mount, one `DeployDetailPage` with two route mounts (services/static), one `useDeploy` caller; static/cron have no step → resolver `none`. REST/GraphQL/MCP adapters unchanged. Unverified: suspend/first-release cancel, true superseded steps live, paid worker/private, alias URLs, Chinese UI, Render signed-in UI.
- **Tests:** real-Apollo resolver suite (paging past a full page, foreign fact, same-second, error/retry, deploy switch) and page suite over a window-filtering fake server (fails on the old finishedAt bound); dashboard lint + 4546 tests green.
- **Cleanup:** fixture deleted, REST 404; databases/KV/env groups/blueprints/projects match baseline (services differ only by another worker's a186 fixture deleted by its owner); own session revoked, whoami 401, cookies 0, about:blank.

