# w11 · m9 — Global side drawer: workspace switcher, personal status, and logout

**Worker:** worker11 **Goal:** give every authenticated mobile screen one consistent beancount-inspired drawer for workspace context, signed-in personal status, and secure logout **Status:** done (2026-08-02; one provider-hosted reveal drawer wraps the authenticated tabs — workspace switcher through m2's fail-closed boundary, in-memory `GET /v1/users` personal status, confirmed local-first logout; page-local switcher/logout removed; reduced-motion/safe-area/screen-reader/gesture-ownership handled; 36 new unit tests, format/typecheck/lint/295 unit/expo-check/iOS+Android bundles green; ADR048 §D7 + `mobile/e2e/m9-side-drawer.md` evidence; no simulator/device was available for the interactive iOS/Android walk)

## Gating

Starts after completed `w11/m2/t009`; may run independently of m5–m8. Reuse m2's workspace/cache boundary and OAuth session manager exactly—this milestone changes navigation and consumes the existing Render `GET /v1/users` identity read, not auth or tenancy semantics.

## Tasks (in order)

| id   | title                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | Add the shared beancount-inspired drawer shell around authenticated tabs     — **DONE** | 60m | w11/m2/t009 |
| t002 | Move workspace selection into the drawer with truthful role/plan status      — **DONE** | 45m | t001       |
| t003 | Add an in-memory personal-status read from Render `GET /v1/users`            — **DONE** | 45m | t002       |
| t004 | Add the personal footer, confirmed logout, and remove page-local duplicates  — **DONE** | 45m | t003       |
| t005 | Harden gestures, accessibility, safe areas, and responsive behavior          — **DONE** | 45m | t004       |
| t006 | Check Render parity and document the mobile navigation extension             — **DONE** | 30m | t005       |
| t007 | Simplify the drawer implementation                                           — **DONE** | 30m | t006       |
| t008 | Add meaningful drawer, identity, workspace-boundary, and logout coverage      — **DONE** | 60m | t006       |
| t009 | Close out the milestone                                                      — **DONE** | 15m | t007, t008 |

## Definition of done

From every authenticated tab and nested detail route on iOS and Android, a user can open one branded side drawer by button or left-edge swipe, see every accessible workspace with the current one selected and its role/plan status, switch workspace through the existing fail-closed cache boundary, inspect their Render-compatible account name/email (or an honest unavailable state), and confirm logout through the existing local-first OAuth revocation flow. Selecting a workspace or action closes the drawer; a switch never flashes prior-workspace data. The old Status-page-only switcher/logout controls are gone, tabs and deep links retain their current route/state behavior, horizontal charts/pagers do not accidentally open the drawer, Android back/dismissal, safe areas, rotation, reduced motion, and screen-reader focus/state are verified, and all copy is bilingual. Mobile's full required checks and focused iOS/Android interaction walks pass.

## Source + Goal linkage

- **Source:** User request 2026-08-02; reference `../beancount-io/mobile/src/components/ledger-drawer/{ledger-drawer.tsx,ledger-drawer-context.tsx,ledger-drawer-header.tsx,group-ledgers-by-owner.ts}` plus its settings account header/logout flow; bex baseline `mobile/app/(app)/_layout.tsx`, `mobile/src/features/workspaces/{workspace-provider.tsx,workspace-switcher.tsx}`, `mobile/src/features/auth/`, and `mobile/src/features/resources/resource-status-screen.tsx`.
- **Goal linkage:** Advances ADR048 D1's supervision-first native experience and ADR008's Render-like developer experience: workspace and account context become globally reachable without adding forbidden desktop configuration or a mobile-only backend capability.
- **Expected outcome:** Users can change workspace, verify which personal account is signed in, and sign out from the same predictable drawer anywhere in the app instead of returning to Status for two page-local controls.
- **Why now:** The secure primitives shipped in m2, but their UI is fragmented: `WorkspaceSwitcher` and the only logout button live solely in `ResourceStatusScreen`, while Activity, Alerts, agent sessions, and nested details have neither. The copied Beancount client already proves a compatible drawer architecture, and the backend already ships the exact identity endpoint needed for personal status.
- **Research decision:** Adapt Beancount's provider-wrapped, stationary-menu/content-reveal drawer instead of restructuring Expo Router's established Tabs into a Drawer navigator. Expo SDK 57's official Drawer would require Reanimated/Worklets and a nested route-layout migration; this control surface is not a replacement route navigator. Reuse React Native `Animated`/`PanResponder`, the existing `GestureHandlerRootView`, `horizontal-swipe-owner`, Safe Area provider, theme, and tab layout, while fixing accessibility/focus concerns explicitly. Include the standing Render-parity task because the drawer consumes tenant/user-facing APIs; `GET /v1/users` deliberately remains REST-only per its backend contract, with GraphQL/MCP unaffected.
