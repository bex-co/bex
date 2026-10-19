# Finding — push events a member selects are silently dropped by the default minimum urgency

Live functional `/qa-find-bugs w4`, pass e4a77c, 2026-10-19 UTC, `muse.env`, workspace `bex-canary`. Production was pinned to `5e75de920`; research is on main `2d55820d2`. The probes are in [evidence.json](evidence.json).

- **Severity:** major. A saved preference that the product reports as saved never takes effect, and nothing says so.
- **Repro:**
  1. `/notifications` → Mobile push (enabled, Minimum urgency **Important**, the default from `notifications/service.go:398-412`).
  2. Under "Event filters", whose copy reads "Only selected event types can produce a push notification.", check **Deploy succeeded** and click Save push settings.
- **Actual:** the toast says "Push settings saved" and no inline notice appears. GraphQL read-back is `events: [DEPLOY_SUCCEEDED, DEPLOY_FAILED, SERVER_FAILED, CRON_FAILED]`, `minimumUrgency: IMPORTANT` (2026-10-19T01:02:01Z). Settings were restored byte-identical afterwards.
- **Expected:** either the selection delivers, or the panel tells the member that this Routine event is filtered while Minimum urgency is Important.
- **Root cause:**
  - `lego/backend/internal/notifications/push_worker.go` projects `deploy_started`, `deploy_succeeded`, `service_suspended` and `service_resumed` at **Routine** (4 of 18 selectable events); every other event is Important or Critical.
  - `delivery_policy.go:288-289` evaluates the event filter first, then drops anything whose urgency ranks below the effective minimum (`DeliveryReasonBelowUrgency`).
  - The dashboard event list (`dashboard/src/features/notifications/lib/push-policy.ts:9-28`) carries no urgency, so `push-notification-settings-panel.tsx` cannot mark the conflict.
- **Fix (target behavior):** keep ADR052's two-dimension policy unchanged; make it visible.
  - The backend publishes each event's urgency on the settings contract, from the same table `push_worker` projects, so the two cannot drift.
  - The panel labels every event with its urgency. For any selected event below the effective minimum (the global one, or a service override's), it shows the drop at the event and in the save area.
  - Saving never silently changes the member's events or urgency.
- **Blast radius:**
  - One policy evaluator (`DeliveryPolicyEvaluator.Evaluate`, `delivery_policy.go:250`) is shared by Expo and web push (`w2/m76`, one policy model).
  - Editors: the global settings and per-service overrides in the same panel. The mobile app edits no push events (`mobile/src/features/notifications/preferences.ts` has no event or urgency fields).
  - Surfaces: REST `GET/PATCH /v1/notification-settings/push`, and GraphQL `pushNotificationSettings` / `updatePushNotificationSettings`. MCP has no push-settings tool.
- **Adjacent classes:**
  - The Critical bypass of quiet hours and deferral is unchanged.
  - Email preferences have no urgency and are unaffected.
  - Unknown or invalid urgencies keep `notifications.pushInvalidUrgency`.
  - An override with `minimumUrgency` Routine makes its Routine events deliverable, and the notice must clear for it.
- **Unverified:** no device or browser subscription exists for the QA user, so actual delivery was not observed. `push_worker` builds recipients only from active subscriptions (`:198`, `:249-285`), and `notificationInbox` returned `[]`. The drop is shown from code and the policy tests.
- **Dedupe:** the only board mention of minimum urgency is `w2/done/m76/done/t004.md` (web push shares the policy). No open item covers inert selections. `DO_NOT_DO.md` has nothing on push urgency.
- **Render:** n/a (bex extension).
