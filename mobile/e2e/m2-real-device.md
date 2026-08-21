# m2 physical-device release qualification

Run this checklist before distributing bex mobile through TestFlight, a Play testing track, or a production app store. Source merge and CI exports do not count as physical-device evidence.

## Preconditions

- One supported physical iPhone/iPad and one supported physical Android device.
- A phone-reachable HTTPS Hydra/dashboard/bex-api environment.
- `scripts/auth-bootstrap-client.sh` has provisioned the public `bex-mobile` client with the exact `co.bex.mobile:/oauth2redirect` private-use redirect (accepted-risk custom scheme; see ADR012).
- A test identity with MFA and membership in two workspaces containing visibly different fixtures.
- A development or release build using the official `co.bex.mobile` bundle/package id; Expo Go is not valid because it does not own the production redirect.

## Evidence matrix

Record device model, OS version, build commit, environment, timestamp, and pass/fail for every row on both platforms. Screenshots must not contain tokens, authorization codes, recovery codes, or other credentials.

1. **System-browser sign-in:** launch signed out, confirm the OS browser—not a WebView—opens Hydra/Kratos, complete password + MFA, and return through the exact native callback.
2. **Cold restore:** terminate the app, relaunch it, and confirm protected UI stays behind the loading gate while refresh rotates; no prior-workspace content flashes.
3. **Workspace isolation:** switch between the two workspaces while a request is in flight and confirm the old workspace disappears during switching and never reappears in late responses.
4. **Offline expiry:** expire/revoke the access token, take the device offline, relaunch, and confirm the app exposes no protected/cached tenant content; reconnect and retry successfully.
5. **Logout:** start logout with a slow/offline network, confirm protected UI disappears immediately, then verify the old access token returns 401 and the old refresh chain returns `invalid_grant` once connectivity returns.
6. **Invalid callback:** open a malformed `co.bex.mobile:/oauth2redirect/...` and an unclaimed `bex://` callback; confirm neither authenticates and no resource screen opens.
7. **iOS reinstall:** uninstall/reinstall, relaunch with any surviving Keychain item, and confirm refresh must succeed before protected UI appears.
8. **Android backup/restore:** restore/reinstall with platform backup enabled and confirm SecureStore credentials are absent/unusable and sign-in is required.
9. **Accessibility and layout:** exercise sign-in, workspace picker, errors, and deep links with VoiceOver/TalkBack, large text, light/dark mode, rotation, and tablet/large-screen layout.

## Release decision

A failure blocks distribution. File the failure as a w11 inbox note or milestone task with device/OS/build details and sanitized reproduction evidence; do not weaken the checklist or substitute simulator/export evidence.
