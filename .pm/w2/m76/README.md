# w2 · m76 — Web push (VAPID): browser push channel for dashboard and self-hosters

**Worker:** worker2 **Goal:** add browser web push (VAPID) as a third push transport beside native Expo push, riding the existing ADR052 durable inbox + per-member/per-event policy — so notifications actually reach users today (native push is release-gated indefinitely on Apple/Google credentials + physical devices) and self-hosters get push without distributing a binary. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                        | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | VAPID contract: keypair generation script, `BEX_WEBPUSH_*` env vars, `.env.example` sync, fail-closed gate (unset ⇒ channel absent, byte-identical)           | 30m | —          |
| t002 | Subscription store + migration: per-member browser subscriptions (endpoint, p256dh/auth keys), register/unregister API                                         | 45m | t001       |
| t003 | Delivery transport: webpush encryption beside the Expo transport in the push worker — same durable inbox, at-least-once, TTL, prune on 404/410                 | 60m | t002       |
| t004 | Policy reuse: web push honors the same per-member/per-event policy rows as native — no second policy model                                                     | 30m | t003       |
| t005 | Dashboard: service worker, permission flow, settings toggle (same-origin, Kratos cookie auth untouched — the DO_NOT_DO no-OAuth2-dashboard rule stands)        | 60m | t002       |
| t006 | Self-hoster docs: enabling web push without Apple/Google dependencies; iOS-Safari/PWA caveats verified and recorded                                            | 30m | t005       |
| t007 | Render parity (standing): subscription/settings surface consistent across exposed surfaces; no-push-at-Render divergence recorded                              | 30m | t004, t006 |
| t008 | Simplify (standing): run /simplify over the changed code                                                                                                       | 30m | t007       |
| t009 | Test coverage (standing): transport selection, policy enforcement, 404/410 pruning, env-unset byte-identical                                                   | 45m | t007       |
| t010 | Closeout (standing): verify DoD, mark done, move milestone to done/                                                                                            | 15m | t009       |

## Definition of done

With `BEX_WEBPUSH_*` set on the dev stack, a member enables browser notifications in dashboard settings, selects events with the existing per-event policy picker, and an induced deploy-failure event arrives as an OS notification in a real browser; a dead subscription (browser unsubscribed) is pruned after a 404/410 push response; with the env unset the channel is absent and behavior is byte-identical; self-hoster docs describe enablement with zero Apple/Google/app-store dependencies.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-08-18 (round 2, item 2); ADR052's gap register + ADR048 ("a responsive/installable PWA and web push remain valid complementary work, especially for self-hosters"); confirmed unbuilt (zero web-push/VAPID code in the repo). Vercel's mobile strategy (web push only) validates the shape.
- **Goal linkage:** notifications system (ADR052) + the open-source/self-host identity (ADR008/ADR058).
- **Expected outcome:** the ADR052 push channel delivers to real users for the first time (native remains release-gated in w11/m5); self-hosters get a store-free push path.
- **Why now:** the entire durable-inbox/policy machinery is live but no push reaches any user; web push has no store gate and de-risks nothing in w11.
- **Render parity included (narrow):** Render has no push notifications at all — the parity task records that divergence (never "fixes" it) and checks the subscription/settings surface is consistent across the surfaces it exposes.
- **Repo facts (checked 2026-08-18):** the push machinery to extend is `lego/backend/internal/notifications/` (`push_worker.go` with `classifyPushError` → prune; provider-neutral `push/transport.go`; Expo adapter + loopback fake; durable tables from migrations `0062_device_push_subscriptions` / `0063_push_deliveries`). `device_push_subscriptions` CHECK-constrains `provider='expo'` + `platform IN ('ios','android')`, so web-push subscriptions get a sibling table (next free migration ≥ 0088). Env precedent: `BEX_PUSH_PROVIDER` / `BEX_EXPO_PUSH_*` (`.env.example` ~347–366, wired `cmd/api/main.go` ~249; `scripts/push-secret.sh`). Dashboard surface to extend: `dashboard/src/routes/notifications.tsx` + `dashboard/src/features/notifications/` (push-notification-settings-panel, availability endpoint `GET /v1/notification-settings/push/availability`); `dashboard/public/` hosts the service worker. ADR018's native-push extension paragraph (~line 27) is the parity anchor t007 extends.
