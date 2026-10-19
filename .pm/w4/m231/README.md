# w4 · m231 — Make push event choices honest about the urgency filter

**Worker:** worker4 **Goal:** a push event a member selects is either deliverable under the urgency that applies to it, or visibly marked as filtered before and after saving. Each event's urgency comes from the backend's single projection, not a client copy. **Status:** in progress (t001–t006 done 2026-10-19; t007 closeout waits on the deploy and live replay)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Publish each push event's urgency on the settings contract — **DONE** | 40m | — |
| [t002](done/t002.md) | Show urgency per event and flag selections the effective minimum drops — **DONE** | 50m | w4/m231/t001 |
| [t003](done/t003.md) | Audit policy consumers and place the adjacent classes — **DONE** | 20m | w4/m231/t001, w4/m231/t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 10m | w4/m231/t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 10m | w4/m231/t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | w4/m231/t004 |
| [t007](t007.md) | Closeout | 20m | w4/m231/t005, w4/m231/t006 |

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
