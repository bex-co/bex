# w4 · m198 — Include continued pre-deploy output after cancellation

**Worker:** worker4 **Goal:** a canceled deploy's default log view includes its own pre-deploy step's later completion/error output while retaining the correct deploy cancellation time. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resolve the matching pre-deploy end fact | 35m | — |
| t002 | Include the finished step in the canceled deploy's default log window | 25m | w4/m198/t001 |
| t003 | Audit shared consumers and preserve window controls | 20m | w4/m198/t001, w4/m198/t002 |
| t004 | Render parity and API/UI contract verification | 20m | w4/m198/t003 |
| t005 | Simplify changed code | 15m | w4/m198/t004 |
| t006 | Test coverage for canceled-step diagnostic windows | 30m | w4/m198/t004, w4/m198/t005 |
| t007 | Closeout with the live success and error replays | 15m | w4/m198/t006 |

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
