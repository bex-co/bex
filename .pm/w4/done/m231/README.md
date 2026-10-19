# w4 · m231 — Make push event choices honest about the urgency filter

**Worker:** worker4 **Goal:** a push event a member selects is either deliverable under the urgency that applies to it, or visibly marked as filtered before and after saving. Each event's urgency comes from the backend's single projection, not a client copy. **Status:** done (2026-10-19)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Publish each push event's urgency on the settings contract — **DONE** | 40m | — |
| [t002](done/t002.md) | Show urgency per event and flag selections the effective minimum drops — **DONE** | 50m | w4/m231/t001 |
| [t003](done/t003.md) | Audit policy consumers and place the adjacent classes — **DONE** | 20m | w4/m231/t001, w4/m231/t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 10m | w4/m231/t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 10m | w4/m231/t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | w4/m231/t004 |
| [t007](done/t007.md) | Closeout — **DONE** | 20m | w4/m231/t005, w4/m231/t006 |

Seven tasks, 190m total (3h10m). One **major** finding: a saved selection silently never delivers. This milestone schedules the work only.

## Definition of done

Replay [finding.md](finding.md) using the [recorded probes](evidence.json) as the QA user. Record that user's push settings first and restore them exactly at the end.

- **Urgency on the read.** `GET /v1/notification-settings/push` and GraphQL `pushNotificationSettings` (or a sibling catalog read named in t001) return the urgency of every selectable event. `deploy_started`, `deploy_succeeded`, `service_suspended` and `service_resumed` read **Routine**; the other 14 read what `push_worker.go` projects. Both surfaces return the same table, and a backend test fails if the projection and the published table disagree.
- **Global filter.** At fresh 1440 and 390 on `/notifications` with Minimum urgency **Important**, each event shows its urgency. Checking "Deploy succeeded" shows, at that event and in the save area, that it will not be pushed until Minimum urgency is Routine. Saving still persists exactly the chosen events and urgency (REST/GraphQL read-back), with no silent change to either. Lowering Minimum urgency to Routine clears the notice.
- **Per-service override.** Adding an override with a Routine event under an inherited or explicit Important minimum shows the same notice for that service only.
- **Control.** A selection at or above the minimum (for example "Deploy failed" under Important) shows no notice. Validation errors keep their existing messages.
- **Checks pass.** Dashboard `yarn lint` and `yarn test` and backend `go test ./...` pass; the failing-direction UI test fails on the pre-fix panel.
- **Cleanup.** The QA user's push settings read back byte-identical to the recorded baseline, and only this run's session is revoked.

Live delivery to a device or browser is not part of this DoD (none is registered for the QA user). t006 asserts the drop/deliver decision through `delivery_policy` with the published urgencies.

## Source + Goal linkage

- **Source:** live functional `/qa-find-bugs w4`, pass e4a77c, 2026-10-19 UTC, `muse.env`, workspace `bex-canary`. See [finding.md](finding.md) and [evidence.json](evidence.json); the screenshot `.playwright-mcp/qa-push-urgency-e4a77c.png` is local and gitignored. Production was pinned to `5e75de920`; research is on main `2d55820d2`.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) truthful product state; [ADR052](../../../docs/ADR052-notifications.md) §(c) defines push policy as event filter **and** minimum urgency. This milestone makes that combination visible rather than changing it.
- **Expected outcome:** members who enable deploy-success or suspend/resume pushes either receive them or are told, before saving, why they won't.
- **Why now:** the default minimum (Important) makes exactly the obvious opt-ins inert: "Deploy started", "Deploy succeeded", "Service suspended" and "Service resumed". The panel promises "Only selected event types can produce a push notification" and confirms "Push settings saved". Browser push (w2/m76) means any dashboard user can now hit this, not only mobile users.
- **Render parity included:** push is a bex extension with no Render counterpart. The task checks REST/GraphQL/dashboard consistency of the new urgency field; MCP has no push-settings tool, and that absence is recorded rather than added.
- **Unverified:** actual device and browser delivery, and the mobile app (which edits no push events today).

## Hosted probe — qa pass 69310f (2026-10-19, not the t007 closeout)

A `/qa-find-bugs w4` pass replayed the Definition of done on production, pinned to `94a76a923136`, which contains `94a162d20`. It ran as the QA user in bex-canary. The push settings were recorded first and restored afterwards.

- **Baseline** (REST `GET /v1/notification-settings/push`):

  ```
  {"enabled":true,"events":["deploy_failed","server_failed","cron_failed"],"minimumUrgency":"important","timeZone":"UTC","workingHours":[],"quietHours":[],"maxDeferralSeconds":28800,"serviceOverrides":[]}
  ```

- **Urgency on the read ✅:** `GET /v1/notification-settings/push/event-urgencies` and GraphQL `pushEventUrgencies` return the same 18 rows.
  - `deploy_started`, `deploy_succeeded`, `service_suspended` and `service_resumed` are `routine`.
  - `server_failed`, `postgres_unavailable` and `key_value_unhealthy` are `critical`, and the other 11 are `important`.
- **Global filter, 1440 ✅:** every Event filters checkbox is named with its urgency (for example `Deploy failed Important`, `Deploy started Routine`), and the group copy reads "Only selected event types at or above the Minimum urgency can produce a push notification."
  - Checking **Deploy succeeded** under Important adds "Not pushed while Minimum urgency is Important." to that checkbox. The save area adds a `status`: "Some selected events won't be pushed — Deploy succeeded is Routine and won't be pushed while Minimum urgency is Important."
  - Save → toast "Push settings saved". REST read-back: events `deploy_succeeded, deploy_failed, server_failed, cron_failed`, `minimumUrgency: important`. GraphQL read-back is the same, so nothing was silently changed.
  - The notice survives a reload. Selecting Minimum urgency **Routine** (unsaved) clears both the event notice and the save-area status.
- **Per-service override ✅:** adding an override for the owned `qa-20261019-69310f-st` with "Override with an exact event list" and **Service suspended** under an inherited Important:
  - shows "Not pushed while Minimum urgency is Important." at that event, and "1 selected event won't be pushed for this service while its Minimum urgency is Important." in the override
  - adds the save-area item "qa-20261019-69310f-st: Service suspended is Routine and won't be pushed while this service's Minimum urgency is Important."
  - Setting that override's minimum to Routine clears only its notice; the global Deploy succeeded item stays. The override was not saved.
  - The explicit-Important variant was not probed.
- **Control ✅:** "Deploy failed Important", checked under Important, carries no notice.
- **Validation ✅:** saving with IANA timezone `Mars/Phobos` shows the `alert` "Enter a valid IANA timezone such as America/Los_Angeles.". No toast appears and nothing is saved (REST unchanged).
- **390 ✅:** on a fresh load at 390×844, the saved Deploy succeeded shows its notice (x 70, width 274, inside the viewport) and the save-area status, with no horizontal overflow. Screenshots `.playwright-mcp/qa-push-urgency-notice-69310f.png` (1440) and `qa-push-urgency-390-69310f.png` (local, gitignored).
- **Cleanup ✅:** REST `PATCH` with the baseline body, and the next `GET` was **byte-identical** to it; GraphQL agrees. The static fixture was deleted (404), and only that run's session was revoked.
- **Not observed:** the "Checks pass" bullet (CI), and live device or browser delivery (outside this DoD).
