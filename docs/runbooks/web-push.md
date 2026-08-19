# Runbook — Browser web push (VAPID)

**Owner:** notifications · **Source:** [ADR052](../ADR052-notifications.md) D3c · **Status:** implemented (w2/m76)

This runbook enables **browser** Web Push for the dashboard. It is independent of native Expo push ([mobile-push.md](mobile-push.md)). There are **no Apple, Google, Expo, or app-store prerequisites**: bex talks to the browser vendor's push service with a VAPID keypair you mint locally.

With all three `BEX_WEBPUSH_*` variables unset, bex-api constructs no VAPID sender, makes no push-service network calls, and the dashboard reports that browser push is not configured. Native Expo availability is unchanged.

## 1. Preconditions

- A control-plane database (`BEX_CP_DB_URI`); subscriptions and deliveries are durable there.
- Dashboard served over **HTTPS** (or `localhost`). Service workers and `PushManager` require a secure context.
- A signed-in workspace member in a Chromium or Firefox profile you can use for verification.

Do not put the VAPID **private** key in Git, issues, shell history, or any dashboard/mobile binary. The public key is handed to browsers as `applicationServerKey` and is not a secret.

## 2. Install or rotate the VAPID keypair

Preview and install `bex-system/bex-webpush` from a history-disabled shell (the script never prints the private key):

```bash
DRY_RUN=1 scripts/webpush-secret.sh
scripts/webpush-secret.sh
```

Override the contact URI if you do not want the installer default `mailto:webpush@bex.local`:

```text
BEX_WEBPUSH_SUBSCRIBER=mailto:ops@example.com
```

`BEX_WEBPUSH_SUBSCRIBER` must be a `mailto:` address or an `https:` origin (RFC 8292 `sub`).

The installer creates or updates the Secret through a mode-0600 temporary env file and waits for the bex-api rollout. The checked-in Deployment uses three `optional: true` Secret references, so an absent Secret is the honest disabled state.

For rotation:

1. Run `scripts/webpush-secret.sh` (it mints a new pair when the env keys are empty, or re-applies the pair already in `.env`).
2. Wait for every bex-api replica to become Ready.
3. Members must **re-enable** browser notifications: a public-key change invalidates existing `PushManager` subscriptions. Dead endpoints return 404/410 and are pruned.

## 3. Member enablement

On `/notifications`:

1. Save the shared event / quiet-hours policy under **Mobile push** (there is no second policy for the browser).
2. Under **Browser notifications**, choose **Enable in this browser**.
3. Grant the permission prompt.

Unsupported browsers, a denied permission, and a server without VAPID keys each show an explicit state instead of a silent toggle.

## 4. Delivery operations

One source event creates one logical member notification plus at most one durable delivery per active Expo installation **and** per active browser subscription. Web push encrypts the closed JSON envelope (`title`, `body`, `data`) with RFC 8291 `aes128gcm` and authenticates with an RFC 8292 VAPID JWT. Outcomes:

- HTTP 201/202: mark the delivery delivered (no Expo-style receipt poll).
- 404/410: revoke **that** browser subscription and stop sending to its endpoint.
- 429 / 5xx / 408: bounded retry; the subscription stays.
- 400/413: terminal payload failure; do not prune.

Payloads contain only short status copy and an allowlisted **relative** route. The service worker (`/push-sw.js`) ignores absolute or protocol-relative click targets.

## 5. Verification

1. Enable browser notifications as above.
2. Induce a `deploy_failed` on a service the member would be notified about (same recipe as deploy-notification email).
3. Confirm an OS notification appears and that clicking it opens the in-app route.
4. In DevTools → Application → Notifications / Push, unsubscribe the browser (or revoke the site permission) and induce another event: the worker should log a prune (`410` / `404`) and the subscription list should drop that browser.

To prove the disabled path, remove `bex-system/bex-webpush` (or unset the three env vars on a host-run bex-api), restart, and confirm `GET /v1/notification-settings/push/availability` returns `"webPushAvailable": false` with no `vapidPublicKey`.

## 6. Browser caveats

| Browser | What we verified | Caveat |
| --- | --- | --- |
| Chromium / Firefox desktop | Code + loopback HTTP classification tests; live OS-notification walk is operator evidence on a HTTPS dashboard | Requires a secure context and a granted permission. |
| iOS Safari | **Vendor-documented only** (not live-tested in this milestone) | `PushManager` exists only after the site is **added to the Home Screen** (installed PWA). A tab in Safari is not enough. Full PWA installability (manifest/offline) is separate work. |
| Corporate TLS intercept | Documented | Some proxies block vendor push endpoints (`*.push.services.mozilla.com`, FCM / WNS endpoints). Delivery then retries as transient; it is not a bex application bug. |

## 7. Troubleshooting

- **Permission denied** — the dashboard says so; recover in the browser site settings, then enable again.
- **Toggle missing / “server has not enabled”** — Secret absent or partial `BEX_WEBPUSH_*` (partial set fails bex-api startup).
- **Notification click does nothing** — the envelope `route` must be a same-origin path starting with `/` and not `//`.
- **Repeated 410** — the browser dropped the subscription (key rotation, site data cleared); the member re-subscribes.
