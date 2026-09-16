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

### Credential inventory (audited 2026-09-11)

Public identifiers only; every secret lives in EAS or the cluster Secret.

| Credential | Where it lives | State |
| --- | --- | --- |
| EAS project `@puncsky/bex-mobile` | `app.json` `extra.eas.projectId` + all three `eas.json` profiles | `dba70c4b-4aae-4bf9-a461-a19bcae69b3a` |
| Apple Team | provisioning profile in the shipped `.ipa` | `PTLM7BZQMM` (Stargately, Inc.) |
| iOS push entitlement | App ID capability → `aps-environment` in the profile | `production` — present |
| APNs auth key (`.p8`) | EAS iOS credentials | key `K988RCCWG7` — present |
| FCM v1 service account | EAS Android credentials | `firebase-adminsdk-fbsvc@mobile-bex.iam.gserviceaccount.com` — present |
| Android upload keystore | EAS Android build credentials | SHA-256 `38:3B:…:DC:71` |
| Expo access token | `bex-system/bex-push` (prod) | installed, valid |
| Expo **enhanced push security** | Expo account/project setting | **off** — token is not yet enforced |
| Play App Signing key | Play Console → Setup → App integrity | **not yet recorded** (see below) |
| Association repo vars | GitHub repo **Variables** (not secrets) | **both set 2026-09-16** — associations ship configured from the next dashboard build (see below) |

Read back the EAS-side rows at any time with `eas credentials -p ios` / `-p android` (interactive).

### App association files

`dashboard/public/.well-known/apple-app-site-association` and `assetlinks.json` are **generated build artifacts, not hand-maintained files.** `dashboard/package.json`'s `build` script runs `node scripts/generate-mobile-associations.mjs` before every `vite build`, so anything edited into them by hand is overwritten on the next build and never reaches production. Editing them is always the wrong move.

They are produced from two GitHub Actions **repository variables**, already wired through `.github/workflows/deploy.yml` → `dashboard/Dockerfile` build args:

| Variable | Value to set |
| --- | --- |
| `BEX_MOBILE_APPLE_TEAM_ID` | `PTLM7BZQMM` |
| `BEX_MOBILE_ANDROID_SHA256_CERT_FINGERPRINTS` | comma-separated SHA-256 fingerprints |

The generator refuses a one-platform association: set both or neither. Unset emits valid, empty, honestly-disabled documents. Both were set on 2026-09-16, so the next dashboard build publishes configured associations. To change them:

```sh
gh variable set BEX_MOBILE_APPLE_TEAM_ID --body PTLM7BZQMM
gh variable set BEX_MOBILE_ANDROID_SHA256_CERT_FINGERPRINTS --body '<upload-sha256>,<play-sha256>'
```

Preview the exact artifacts without a deploy:

```sh
cd dashboard && BEX_MOBILE_APPLE_TEAM_ID=… BEX_MOBILE_ANDROID_SHA256_CERT_FINGERPRINTS=… \
  BEX_MOBILE_ASSOCIATION_OUTPUT_DIR=/tmp/assoc node scripts/generate-mobile-associations.mjs
```

The documents claim exactly one path, `/invite`, matching `app.json`'s Android intent filter. The OAuth callback is deliberately **not** claimed — see [ADR012](../ADR012-auth.md): the mobile redirect is a private-use custom scheme, and claiming `/oauth2redirect` points iOS at a route the dashboard does not serve. A regression test enforces this.

The fingerprints variable takes a **list**, which is what the Play re-signing problem needs. The EAS upload keystore fingerprint alone only covers internal-distribution APKs; Google re-signs Play uploads, so a Play-delivered build will not verify against it. **Before any Play-track install is used for qualification**, add the Play App Signing SHA-256 from Play Console → Setup → App integrity → App signing key certificate alongside it.

Nitro serves both documents as `application/json` — `dashboard/server/plugins/mobile-association-content-type.ts` overrides the `text/plain` that Nitro's static handler otherwise assigns to Apple's intentionally extensionless filename. After deploying, verify with Apple's CDN (`https://app-site-association.cdn-apple.com/a/v1/dashboard.bex.co`) and Google's [Statement List Generator and Tester](https://developers.google.com/digital-asset-links/tools/generator), then reinstall the app — both platforms fetch the statement at install time.

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
