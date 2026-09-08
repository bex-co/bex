# w6 · m140 — Repair notification workspace and launch handling

**Worker:** worker6 **Goal:** make mobile alerts open the correct authorized destination in the selected workspace and remove locally retained content when destination access is lost **Status:** done

**Size:** 3h implementation + 2h closing work; 8 tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Carry explicit workspace context through notification operations — **DONE** | 45m | `w6/m137/t008` |
| t002 | Defer cold-start notification navigation until the context is ready — **DONE** | 45m | `w6/m140/t001`, `w6/m139/t002` |
| t003 | Reconcile notification items and badges after destination access changes — **DONE** | 45m | `w6/m140/t001`, `w6/m139/t003` |
| t004 | Guard asynchronous notification work with the current boundary — **DONE** | 45m | `w6/m140/t002`, `w6/m140/t003` |
| t005 | Render parity — **DONE** | 30m | `w6/m140/t004` |
| t006 | Simplify — **DONE** | 20m | `w6/m140/t005` |
| t007 | Test coverage — **DONE** | 60m | `w6/m140/t005` |
| t008 | Closeout — **DONE** | 10m | `w6/m140/t006`, `w6/m140/t007` |

## Definition of done

- [x] Device registration/list/unregistration, inbox/count, and mark-read use the explicitly selected and server-validated workspace. Existing callers that omit the optional context retain documented default-workspace behavior.
- [x] With two authorized workspaces and a non-default selection, items, badges, token binding, and navigation agree with that selection. Unknown/foreign context fails closed through shared Core membership checks.
- [x] A valid initial tap survives auth/workspace/access restoration and opens the permitted destination exactly once. Invalid, stale-session, and denied destinations remain generic; do not silently switch workspaces from a notification.
- [x] Access changes remove affected locally cached notification content and recalculate badges before it can render or open. Successful reconciliation handles removals; transport errors or a limited page are not mistaken for authoritative global absence.
- [x] Late receipts, mark-read completions, registration results, or inbox responses cannot mutate another workspace/session/generation; logout and switches clear the appropriate local state.
- [x] Automated provider/transport/store integration scenarios and iOS/Android interaction evidence cover cold/warm taps, switching, revocation, and stale callbacks; all changed backend and required mobile checks pass. Physical provider delivery and OS-signing qualification remain explicitly tracked in w11/m5/t007, not claimed by synthetic tests.

## Source + Goal linkage

- **Source:** Mobile source review and $pm-brainstorm on 2026-09-07 at e609e9322, proposal 3; user handoff to $pm for w6. Follow-up to [w11/m5](../../../w11/m5/README.md) client implementation and [w6/m137](../m137/README.md) destination filtering, under [ADR087](../../../../docs/ADR087-mobile-role-views.md) Notifications follow destination access.
- **Goal linkage:** [ADR048](../../../../docs/ADR048-mobile.md) push → evidence → response loop and [ADR008](../../../../docs/ADR008-vision.md) dependable API-driven operations.
- **Expected outcome:** A cold-start alert opens once after authentication and workspace/access validation; non-default workspaces receive their own inbox/registration behavior; revoked destination content disappears from local items and badges.
- **Why now:** The backend now excludes unauthorized destinations, but the native inbox only merges old rows. Notification operations omit selected workspace context, and startup clears its binding then consumes the initial tap before asynchronous registration restores it. These are implementation gaps beyond the existing physical-device qualification gate.
- **Render parity:** Included because this changes a tenant-facing native/API surface. ADR018's Notifications (deploy/failure alerts) row is complete; its Native push extension note documents the intentional absence of installation-bound device registration from MCP. Compare workspace/error semantics on supported surfaces without adding device-token tools or claiming Render push parity.

## Review evidence and scope

- `mobile/src/features/notifications/api/notifications.graphql` omits selected workspace context for device registration, inbox/count, and mark-read; backend notification resolvers use the authenticated request's default workspace when no override is supplied.
- `mobile/src/features/notifications/notifications-provider.tsx:202` clears the active binding, starts inspectAndRepair asynchronously, and then clears/handles getLastNotificationResponse while that binding is absent.
- `mobile/src/features/notifications/inbox-store.ts:122` seeds mergeRemote from every existing local row. An isolated in-memory probe with a stored agent_pr_ready item and an empty newly authorized server inbox retained the item and unread badge 1.
- Notification callbacks check the login session after awaits but need the current workspace and access generation too.
- Expo's [notification routing documentation](https://docs.expo.dev/versions/latest/sdk/notifications/#handle-push-notifications-with-navigation) exposes initial-response handling separately from subsequent response listeners.

Reuse the closed notification envelope, shared backend destination policy, existing personal opt-in/out, and m139's access generation. No new event vocabulary, automatic cross-workspace navigation, lock-screen resource details, provider integration, store-release automation, or device-registration MCP tools.

## Completion evidence — 2026-09-07

[Contract, implementation checks and native interaction evidence](../../../../mobile/e2e/m140-notifications.md) records the observable results for all six DoD items, 343 unit tests, 18 mounted scenarios, backend REST/GraphQL tests and both platform exports. Initial-response, revocation and switch screenshots cover iOS 26.5 and Android 16 synthetic runtimes. Physical provider delivery and signing remain w11/m5/t007; this closeout does not complete that gate.

Simplify's three independent reviews were applied and checked. The optional workspace slice was shipped in f42012713; the remaining implementation and closeout are verified working-tree changes.
