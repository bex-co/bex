# m139 access recovery verification

Verified 2026-09-07 on the local implementation. These are client integration and simulator checks; physical push delivery, signing, release, and live-agent qualification remain in their existing w11 tasks.

## Acceptance evidence

| Requirement                                                  | Evidence                                                                                                                                                                                                                                                                                                                                                      |
| ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Fresh affirmative permission before protected work           | Capability policy enforces the 30-second receipt window, workspace identity, recognized grants, and clock rollback. Mounted expiry tests verify dispatch refuses stale grants even before React renders the expiry. Ordinary 25-second polling preserves fresh mounted logs and confirmation state; a delayed response expires both.                          |
| Membership and fresh access before foreground/reconnect work | Real workspace/capability providers and Apollo link chain verify single-flight membership → fresh capability requests, concurrent reconnect/foreground, startup membership failure recovery, and no protected request before verification. Removed selection reaches the explicit verified chooser or empty state.                                            |
| Unavailable versus denied                                    | Mounted checker outage and unavailable-grant scenarios preserve navigation and announce no downgrade. Confirmed denial clears access and announces the change. English and Chinese native screenshots/announcement capture verify the presentation.                                                                                                           |
| Boundary cancellation and cleared protected state            | Mounted LogViewer and SafeActionPanel checks verify closed streams, cleared buffered lines/modal intent, no late callback rendering, and no automatic replay. An actual Apollo request is aborted on identity reset; its late result cannot enter the cache. A newer reset supersedes an upgrade still waiting for reset handlers.                            |
| Exact confirmation context                                   | Mounted tests invoke obsolete confirmation callbacks after target replacement, workspace switch, and identity reset; none dispatch. Double confirmation sends once. Ambiguous delivery retains accepted-but-unverified feedback. Credential-wait tests prevent history/stream dispatch after access loss or cancellation; closed XHR ignores queued progress. |
| Native navigation                                            | Android native tab taps, foreground recovery, and hardware Back verify that confirmed loss removes Sessions history with `Tabs.Protected`. iOS native tab/session taps, capability downgrade, Back, upgrade, and manual recovery verify restricted content stays cleared and native tabs follow confirmed grants.                                             |
| Shared contracts                                             | Parity trace below; backend, dashboard, scopes, server ownership, and resource-action contracts are unchanged.                                                                                                                                                                                                                                                |
| Quality and verification                                     | Three simplify reviewers completed reuse, quality, and efficiency reviews. Applied shared environment/message/freshness helpers, fixture reuse, unchanged-selection persistence, and poll preservation. Final counts: 335 unit tests plus ten mounted integration scenarios.                                                                                  |

## Native evidence

A development build ran on iPhone 17 Pro and Android 36 ARM64. A temporary in-memory fixture replaced the Apollo terminal transport while retaining production boundary/access links, providers, screens, and native navigators. It supplied synthetic workspace/resource/session data and rejected every resource mutation. No server resource mutation was sent.

- [Android Sessions before revocation](artifacts/m139/android-sessions-before.png) → [Status after revocation and hardware Back](artifacts/m139/android-status-after-revocation-back.png). Before the fix, Back returned to the hidden Sessions route with a denied card. `Tabs.Protected` removes that history; the repeated case ended on Status with `canGoBack() === false`.
- [Android restart confirmation](artifacts/m139/android-confirmation-before.png) → [confirmation cleared on recovery/downgrade](artifacts/m139/android-confirmation-revoked.png). Opened with a native tap, changed operation access to denied, pressed Home and resumed. The modal disappeared, Status rendered, and the mutation count stayed zero through subsequent polling.
- [iOS Status after revocation and Back](artifacts/m139/ios-status-after-revocation-back.png). Sessions and its protected detail content were removed after confirmed loss.
- [iOS Chinese unavailable state](artifacts/m139/ios-unavailable-zh.png) retains established tabs while clearing protected Status content and offering neutral retry copy. After native Retry, captured order was membership → fresh capabilities → permitted Status reads. [Confirmed Viewer result](artifacts/m139/ios-viewer-zh.png) removes Sessions and announces `你在此工作区的访问权限已更改。`; no mutation was sent and `canGoBack()` was false.
- [iOS direct identity replacement](artifacts/m139/ios-identity-final.png) has correct safe-area geometry. The initial fixture included an extra navigation replacement during identity reset and showed an overlapping header. Repeating the actual reset → identity publication sequence without that extra fixture navigation rendered correctly, as did signed-out/sign-in restoration and subsequent permission changes. No product layout workaround was added.

An Android development static-flag warning occurred during initial fixture setup with the old hidden-tab implementation. The inspected Metro log contains that single occurrence; the later protected-tab and confirmation/recovery runs did not reproduce it. Simulator checks do not claim physical screen-reader delivery or provider notification qualification; the native accessibility API invocation and translated payload were captured.

## Checks

- `yarn format:check`: passed.
- `yarn typecheck`: passed.
- `yarn lint` (ESLint and knip): passed.
- `yarn test:unit`: 335 unit tests and ten mounted integration tests passed.
- `yarn expo:check`: passed.
- `yarn bundle:ios` and `yarn bundle:android`: passed after the final production change.
- Android development APK: built, installed, and exercised.
- `git diff --check` and repository Markdown formatting: passed.

The mounted suite is `scripts/access-recovery-tests.js`; policy, boundary, and REST-log regressions live beside their implementation. Native fixture scripts stayed outside the repository and never persisted OAuth credentials. Backend code is unchanged, so no backend test gate is newly affected.

## Contract parity review

Reviewed ADR006's shared Core and named-workspace contract, ADR018's Native mobile / Role-aware dashboard controls notes and Workspace members & roles row, ADR087's access recovery rules, and the dashboard/mobile contributor guides.

| Surface        | Current contract and m139 effect                                                                                                                                                                                                                                                            |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Shared backend | `members/service.go:346` resolves membership and probes effective relations. `fresh=true` rechecks `can_view` and bypasses the positive-check cache on that replica; it is not a membership version or an instantaneous global revocation guarantee. No backend changes.                    |
| REST           | `GET /v1/viewer/capabilities?ownerId=…&fresh=true` calls the same service (`members/rest.go:134`). Log history/subscription retain their existing routes, ownership, bearer, and server error semantics. m139 blocks dispatch locally when its access/session context is no longer current. |
| GraphQL        | `viewerCapabilities(ownerId:, fresh:)` delegates to the same service (`members/graphql.go:169`). Mobile continues using generated documents and the four existing tri-state grants. Receipt time and boundary generation are client memory only, with no schema additions.                  |
| MCP            | `get_viewer_capabilities` passes the named workspace and `fresh` to the same service (`members/mcp.go:145`). This existing bex extension and its errors are unchanged.                                                                                                                      |
| Dashboard      | `features/capabilities/hooks/use-capabilities.ts` retains its explicitly documented permissive-while-unknown controls. Native intentionally requires affirmative fresh access under ADR087; this is a presentation/recovery policy difference, not a different server permission model.     |

The existing ledger explicitly preserves Billing's general supervision reads and requires `can_create` for Contributor rollback in bex. m139 preserves both decisions. It does not claim a newly completed Render matrix cell, widen OAuth scopes, add mobile settings/destructive actions, or change resource preconditions. Resource action projections remain owned by m141; notification persistence and launch reconciliation remain owned by m140; GraphQL token-refresh recovery remains 071. No new API drift was found in this change, so no additional parity item was created. The existing ledger evidence is sufficient because no server contract or Render capability changes.
