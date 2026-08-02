# m5 native push qualification

Run this matrix before distributing push-enabled bex mobile builds. Simulator builds and source-level tests are useful smoke evidence, but they do not replace authenticated delivery evidence from physical iOS and Android devices.

## Evidence hygiene

- Use a disposable, non-production workspace and test identity.
- Record device model, OS version, build commit, environment, timestamp, and pass/fail for every executed row.
- Never capture or paste access/refresh tokens, authorization codes, Expo/APNs/FCM tokens, provider endpoints containing credentials, EAS credentials, or notification bodies containing tenant secrets.
- Verify registration through the secret-free `notificationDeviceSubscriptions` response and sanitized server delivery/audit state. Do not print the registration mutation variables.
- Screenshots may show generic fixture names and opaque resource ids only.

## Configuration gate

1. Log in to the intended Expo account with `eas login` and verify the account with `eas whoami`.
2. Link or create the first-party EAS project and place its public project id in `EXPO_PUBLIC_EAS_PROJECT_ID`. Do not put any EAS access token or push credential in an `EXPO_PUBLIC_*` variable.
3. Confirm `pushNotificationsAvailable` is `true` in the target bex environment. Repeat one pass with transport disabled and prove the app shows **unavailable** without presenting an OS permission prompt or requesting an Expo token.
4. Build signed development/release binaries for `co.bex.mobile`. Expo Go is not valid push evidence.

## Physical-device matrix

Execute every row on one supported physical iOS device and one supported physical Android device.

| Scenario                  | iOS evidence                                                                                                                                                                                | Android evidence                                                                              |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Fresh install and auth    | System-browser PKCE sign-in returns to the app; Alerts is auth-gated before sign-in                                                                                                         | Same                                                                                          |
| Gesture-only permission   | No prompt at launch; prompt appears only after tapping **Enable**                                                                                                                           | Same; `bex-alerts` channel exists before token registration                                   |
| Denied permission         | Deny the prompt; app remains usable and reports denied without registration                                                                                                                 | Same                                                                                          |
| Revoked permission        | Enable, then revoke in system settings and foreground/relaunch; app reports revoked                                                                                                         | Same                                                                                          |
| Server transport disabled | App reports server unavailable; no OS prompt, token request, or register mutation occurs                                                                                                    | Same                                                                                          |
| Registration and repair   | Secret-free device row appears for the stable installation id; reconnect repairs registration                                                                                               | Same                                                                                          |
| Token rotation            | Rotate the native push token using platform-supported test procedure; current installation is re-registered once without exposing either token                                              | Same                                                                                          |
| Foreground receipt        | One alert appears in the local inbox, deduped by notification id; badge matches unread count                                                                                                | Same                                                                                          |
| Background receipt        | Notification is shown; opening it marks the local item read and reconciles the badge                                                                                                        | Same                                                                                          |
| Terminated/cold start     | Tap a notification from a terminated app; after auth restore it opens only the validated target                                                                                             | Same                                                                                          |
| Deep-link allowlist       | Valid `srv-`, `dpg-`, `red-`, and `ags-` relative routes reach their existing authorized detail query                                                                                       | Same                                                                                          |
| Malicious/stale link      | Absolute URLs, query/fragment/traversal, unknown routes/fields/events, malformed ids, and missing/unauthorized resources fail safely                                                        | Same                                                                                          |
| Logout online             | Protected UI disappears immediately; inbox and badge clear; current installation unregisters                                                                                                | Same                                                                                          |
| Logout offline            | Protected UI/inbox/badge clear immediately without waiting for network; stale server registration is not treated as local success and is pruned/repaired by the documented server lifecycle | Same                                                                                          |
| Accessibility/layout      | VoiceOver, large text, light/dark mode, rotation, and notification settings/inbox are usable                                                                                                | TalkBack, font scaling, light/dark mode, rotation, and notification settings/inbox are usable |

Also exercise one deploy success/failure, service failure, cron failure, quiet-hours deferral, collapse/dedupe, and a delivery retry/prune path against sanitized fixtures. Correlate each visible notification to durable server evidence without recording payload credentials.

## 2026-08-02 simulator smoke evidence

- Executed: `npx expo run:ios --device 'iPhone 17 Pro' --no-bundler` against the already-running workspace Metro server on port 8081.
- Result: native iOS build succeeded with 0 errors, installed as `co.bex.mobile`, and launched on an iPhone 17 Pro simulator running iOS 26.5.
- Visually inspected: the signed-out screen rendered without a React error overlay and exposed no protected tenant or Alerts content. The Alerts tab correctly remained unreachable behind authentication; no test identity was fabricated.
- Screenshot: [signed-out simulator smoke](./artifacts/m5-ios-sign-in.png). The retained Debug-development-client frame includes Expo's generic LogBox warning toast; it is not presented as release-build or push-delivery evidence.
- Not executed: EAS login/account verification, EAS project linking, a populated `EXPO_PUBLIC_EAS_PROJECT_ID`, authenticated sign-in, Alerts UI behind auth, OS permission flow, Expo token acquisition, APNs/FCM delivery, background/terminated delivery, or any physical iOS/Android row above.

## Release decision

Any failed physical-device row blocks push-enabled distribution. Simulator/export evidence must remain labeled as simulator/export evidence and must never be promoted to a physical-device pass.
