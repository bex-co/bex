# Runbook — Mobile push setup, rotation, and verification

**Owner:** mobile workstream · **Source:** [ADR048](../ADR048-mobile.md) D2 · **Status:** implemented; physical release qualification pending

This runbook enables Expo Push Service delivery for the bex mobile companion. Push is optional: with `BEX_PUSH_PROVIDER` unset, bex-api constructs no push transport and makes no Expo network calls. Supervision, email notifications, and device-subscription cleanup continue to work.

## 1. Preconditions

- An EAS project bound to the `co.bex.mobile` iOS bundle and Android application IDs.
- `EXPO_PUBLIC_EAS_PROJECT_ID` set to that public project ID at mobile build time.
- APNs and FCM v1 credentials provisioned for the EAS project.
- Expo enhanced push security enabled and a dedicated access token minted.
- A control-plane database (`BEX_CP_DB_URI`); subscriptions, inbox entries, delivery attempts, and receipts are durable there.
- Disposable signed-in test members and physical iOS and Android devices for release qualification.

Follow Expo's [push setup](https://docs.expo.dev/push-notifications/push-notifications-setup/) for the platform credentials. Do not put the Expo access token in `EXPO_PUBLIC_*`, `app.json`, Git, an issue, shell history, or a mobile binary. The EAS project ID is public configuration; the access token is a server credential.

## 2. Install or rotate the server credential

Put the dedicated token in the repo-local, gitignored `.env`, or export it from a secret manager into a history-disabled shell:

```text
BEX_PUSH_PROVIDER=expo
BEX_EXPO_PUSH_ACCESS_TOKEN=<out-of-band Expo access token>
```

Preview and install it into the cluster selected by the current kubeconfig:

```bash
DRY_RUN=1 scripts/push-secret.sh
scripts/push-secret.sh
```

The installer creates or updates `bex-system/bex-push` through a mode-0600 temporary env file, never prints the token, and waits for the bex-api rollout. The checked-in Deployment uses optional Secret references, so an absent Secret is the honest disabled state.

For rotation:

1. Mint a new dedicated Expo access token without revoking the old token.
2. Run `scripts/push-secret.sh` with the new token.
3. Wait for every bex-api replica to become Ready.
4. Trigger one disposable notification and prove its Expo receipt succeeds.
5. Revoke the old token in Expo.

Do not configure `BEX_EXPO_PUSH_URL` in production. It exists only for a loopback/TLS fake-provider test; an HTTP non-loopback value is rejected at startup.

## 3. Build-time mobile configuration

Copy `mobile/.env.template` to a gitignored mobile env file and set:

```text
EXPO_PUBLIC_EAS_PROJECT_ID=<public EAS project UUID>
```

Remote notifications require a native development or release build after adding `expo-notifications`; Expo Go is not release evidence. On Android, the client creates the `bex-alerts` channel before requesting a token. The permission prompt is shown only after the user explicitly enables notifications in the app.

Denial is normal. A denied device remains usable, shows the disabled state, and can open OS settings later. Explicit logout clears local inbox/badge state first and attempts to revoke only that installation's subscription without making local logout depend on the network.

## 4. Delivery and receipt operations

One source event creates one logical member notification and at most one durable delivery per active installation. Provider sends use a stable notification ID, collapse ID, and Android tag. This prevents logical replay duplicates and lets the operating system replace a retry where supported; it does not turn Expo/APNs/FCM into an exactly-once transport.

Expo ticket success means only that Expo accepted the request. The worker persists the ticket and checks its [push receipt](https://docs.expo.dev/push-notifications/sending-notifications/) after the provider's recommended delay. Outcomes are handled as follows:

- success: mark the durable delivery delivered;
- pending/missing receipt: poll again within the bounded receipt window;
- throttling or provider outage: bounded exponential retry;
- `DeviceNotRegistered`: revoke the exact device subscription and stop sending to its token;
- malformed payload or invalid credentials: terminal failure and operator-visible metric, without logging the provider message or token.

Payloads contain only the fixed schema version, logical notification ID, closed event type, short status copy, and an allowlisted relative route containing an opaque resource ID. They never contain environment values, prompts, repository content, logs, credentials, provider endpoints, or member email addresses.

## 5. Release qualification

Use disposable devices/subscriptions and redact every captured identifier. On both a physical iOS device and a physical Android device, prove:

1. Permission not-determined, denied, granted, and later-revoked states.
2. Token registration, rotation, app reinstall/account switch, and explicit logout cleanup.
3. Foreground, background, and terminated delivery.
4. Exactly one visible alert for one durable `deploy_failed` event and one durable `server_failed` event under normal provider behavior.
5. Quiet-hours deferral, timezone change, a DST boundary, and critical bex-schedule bypass without claiming the iOS Critical Alerts entitlement.
6. Inbox deduplication, unread count, badge reconciliation, and read state.
7. A tap opens the exact authorized service route; absolute URLs, queries, fragments, traversal, unknown routes, extra payload keys, stale IDs, and unauthorized resources fail closed.
8. A transient provider failure retries without a storm; `DeviceNotRegistered` prunes the disposable token.
9. Cleanup removes every disposable subscription, notification, delivery, and captured token artifact.

Simulator UI and deep-link tests are useful but do not satisfy this release gate. Record redacted device/OS/build identifiers, source event ID, logical notification ID, timestamps, receipt class, visible-count result, and cleanup result in `mobile/e2e/`; never record a push token or Expo credential.

## 6. Disable and rollback

To stop new push network traffic, remove `BEX_PUSH_PROVIDER` and `BEX_EXPO_PUSH_ACCESS_TOKEN` from `bex-system/bex-push` or delete that Secret, then restart bex-api. Existing subscriptions and inbox history remain available for repair and retention; the worker does not send while the transport is disabled. Re-enabling with a valid token resumes durable pending work subject to its retry/retention bounds.
