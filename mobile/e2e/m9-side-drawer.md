# m9 global side drawer device qualification

Run this release gate against a disposable, non-production workspace on one physical iOS device and one physical Android device. Source-level completion (unit tests, typecheck, lint, Expo export, both platform bundles) does not claim this checklist has been executed on hardware with production tenant credentials.

## Setup

- Sign in on both platforms as an identity whose Render `GET /v1/users` returns a real name and email.
- Belong to at least two workspaces with distinct plans/roles so the switcher shows more than one row and truthful status.
- Keep the dashboard workspace list open as independent server evidence for names/plans/roles, and the OAuth session view for logout revocation.

## Open, reach, and dismiss

- From Status, Activity, Sessions, and Alerts, open the drawer with the header menu button; from a nested service/database/key-value/session detail route, open it with a left-edge swipe. Confirm the same single drawer instance appears everywhere and tabs/deep links keep their route and scroll state.
- Close via: tapping the visible content sliver (backdrop), a leftward swipe, the header button target again after reopening, and Android hardware back. Confirm back closes the drawer before it would navigate away, and no invisible catcher keeps eating taps after an interrupted close.
- Rapidly repeat open/close and interrupt the animation mid-slide; confirm it always settles fully open or fully closed with the backdrop state matching.

## Workspace switching

- Confirm every accessible workspace appears once, in API order, with name and localized role · plan, and exactly one checkmark on the current workspace.
- Tap the already-selected workspace: the drawer closes with no reload.
- Switch to another workspace: confirm the previous workspace's resource/usage data never flashes — the switching state shows, the boundary resets, then the new workspace renders — and the drawer closes only after the switch settles. Verify the selection persists across app relaunch for the same login session.
- Pull-to-refresh the list; force an offline state and confirm the offline note plus a reachable retry, with no other workspace's cached tenant data shown.

## Personal status, settings, and logout

- Confirm the footer shows the signed-in name with email beneath (or email-only / an honest "Signed in" when the server returns a partial identity), pinned below the scrolling workspace list, with a chevron indicating it opens Settings.
- Force the current-user read offline/unavailable and confirm the footer shows the distinct offline/unavailable status with an explicit retry (a separate row, not nested in the Settings target), while workspace supervision stays usable.
- Tap the identity row: confirm the full-screen Settings sheet opens with the signed-in identity, a Color scheme selector (System/Light/Dark), a Language selector (English/中文), Log out, and the app version. Close it and confirm the drawer is still open behind.
- Change the color scheme and language: confirm both apply immediately and survive an app relaunch (persisted on device only; never in the auth store). Color scheme "System" tracks the OS setting.
- Tap Log out in Settings: confirm a native confirmation appears. Cancel it — nothing happens. Confirm it — exactly one sign-out runs, the sheet closes, and the auth guard returns to sign-in. Double-tap Log out and the confirmation control; confirm at most one sign-out sequence and no second dialog.
- Log out while offline: confirm local credentials and identity-bound state are cleared even though remote revocation cannot complete (no false failure surfaced).

## Accessibility, safe areas, and responsiveness

- With a screen reader on, open the drawer: confirm one named menu surface, the selected workspace announced as selected, focus moved into the menu, obscured app content not traversable, and focus returning logically on close.
- Verify safe-area insets (notch/home indicator), rotation (width recomputes and stays capped), large Dynamic Type / font scaling (no clipped or untappable rows), light and dark themes, and reduced-motion (open/close is instant, no slide).

## Gesture ownership and negative scope

- On a screen with a horizontal chart/pager, swipe horizontally inside it: confirm the chart/pager keeps the gesture and the drawer does not open. Confirm vertical scrolls and tab presses are never captured as a drawer drag, and the drawer remains fully usable by button alone.
- Verify the drawer adds no route or control for service creation, desktop/service configuration, billing, deletion, or any other destructive surface. The only surfaces are workspace switching, the identity read, and the personal Settings sheet (device-local color scheme, language, and logout) — Settings is a Modal, not a new `app/` route. Confirm the old Status-page workspace picker and logout icon are gone.
- Verify dashboard workspace names/plans/roles and the account name/email match what the phone shows for the same caller.
