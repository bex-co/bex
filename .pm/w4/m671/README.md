# w4 · m671 — bex checks a running image service's image and tells the workspace when it disappears, as Render does

**Worker:** worker4 **Goal:** when the image a running image-backed service serves is deleted from its registry, or its credential stops working, the workspace hears about it (event, webhook, email) while the instances keep serving, before a restart, a scale-up or a rollback fails on it. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design note: what is checked, how often, and when a failure counts | 30m | w4/m669/t001 |
| [t002](t002.md) | The operator checks each running image service's served image and records `ImageAvailable` with its cause | 1h45m | t001 |
| [t003](t003.md) | bex-api records one `image_pull_failed` per outage with no deploy, says the instances still serve, and sends one email | 1h | t002 |
| [t004](t004.md) | dev-4 and production: an image that expires from `ttl.sh` raises the event and the email, and a recovery clears it | 45m | t003 |
| [t005](t005.md) | Render parity | 25m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 5h20m**.

## Definition of done

- Every running (not suspended) image-backed service's served image is checked on t001's interval with the service's own credential, by a manifest `HEAD` that downloads no layer.
- An image that is gone (not found) or whose credential is refused, on t001's consecutive-check rule, sets `ImageAvailable=False` on the App with Render's cause. A rate-limited or unreachable registry never does.
- That transition records exactly one `image_pull_failed` with no deploy, whose `details.message` names the cause and says the running instances are unaffected. It reaches REST, GraphQL, MCP, webhooks and the dashboard Events feed, and sends one Image Pull Failed email worded for a running service. It is not repeated while the outage lasts, and a recovery clears the condition so a later outage reports again.
- The check never changes the running Deployment, and never runs for a service whose image is in bex's own registry.
- On production, a fixture service deployed from a `ttl.sh` image raises the event and the email after the image expires, and the service keeps serving.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: deploying a prebuilt image, as [render.com/docs/deploying-an-image](https://render.com/docs/deploying-an-image) describes), item 4. Re-checked by hand at `origin/main` `28b4f18bbe` before filing: no open, blocked or done item on any workstream owns it.
- **Render:** "Render periodically checks image availability for active services. If an image is unavailable, Render can notify you even if your running services are unaffected. However, any action that requires a fresh image pull might fail until the image is reachable again." The Image Pull Failed event's details require only `message` and `imageURL`, not a deploy (`lego/backend/internal/api/openapi/render-public-api-1.json`).
- **bex today:**
  - Nothing checks a running service's external image; bex learns it is gone only when a pull fails.
  - When a pull of the live release does fail with no deploy open, w8/m156 t005 already records `image_pull_failed` with no deploy (`buildServedReleasePullFact`, `lego/backend/internal/store/reconciler.go:1871-1891`). This milestone reuses that fact shape for a failure found before any pull.
  - The email's lead says "could not pull its Docker image, so the new version did not start" (`lego/backend/internal/notifications/alert_email_worker.go:462-464`), which is wrong for a running service.
  - w8/blocked/m156 keeps bex's own registry from deleting a served build; this milestone covers external registries, which bex does not control.
- **Recommendation, not a user decision:** the interval (proposed: every 6 hours) and the rule "two consecutive not-found or refused answers" are bex's choice, set in t001; Render does not publish its own.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), and reliability: a service should not learn at 3 a.m., on a node replacement, that its image was deleted weeks ago.
- **Expected outcome:** a deleted image or an expired credential is reported while the service still runs, with time to fix it.
- **Why now:** bex replaces nodes and scales automatically, and each of those pulls the image afresh, so a silently deleted image turns into an outage at a random time. w4/m669 supplies the causes this check reports (t001 depends on w4/m669 t001).
- **Render parity:** included: events on REST, GraphQL, MCP and webhooks, the email and the dashboard change.
