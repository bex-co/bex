# w6 · m139 — Complete mobile access-change recovery

**Worker:** worker6 **Goal:** keep protected mobile content and operational controls consistent with the caller's current workspace access after refresh, reconnect, and membership changes **Status:** done

**Size:** 3h implementation + 2h closing work; 8 tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Separate stable navigation from permission freshness — **DONE** | 45m | `w6/m138/t010` |
| t002 | Refresh membership and access before foreground recovery — **DONE** | 45m | `w6/m139/t001` |
| t003 | Invalidate protected screen state and navigation on access changes — **DONE** | 45m | `w6/m139/t001` |
| t004 | Recheck access and connectivity before confirmed actions — **DONE** | 45m | `w6/m139/t002`, `w6/m139/t003` |
| t005 | Render parity — **DONE** | 30m | `w6/m139/t004` |
| t006 | Simplify — **DONE** | 20m | `w6/m139/t005` |
| t007 | Test coverage — **DONE** | 60m | `w6/m139/t005` |
| t008 | Closeout — **DONE** | 10m | `w6/m139/t006`, `w6/m139/t007` |

## Definition of done

- [x] An authorization snapshot older than the ADR087 client freshness window (30 seconds) cannot enable a mutation or a new protected read. Stable navigation may remain while a new check is pending; client receipt time is not represented as a server membership version.
- [x] Foreground/reconnect refreshes membership and access before restoring protected work. A removed selected workspace is cleared and the verified workspace chooser or existing empty-workspace/logout state is shown.
- [x] Transport and authorization-service failures render neutral unavailable states, never access.changed. Only confirmed permission loss triggers a downgrade announcement.
- [x] On detected identity/workspace/access changes, affected requests and streams stop, protected component state and pending confirmations clear, and inaccessible navigation history cannot restore old content.
- [x] Dispatch rechecks the current session/workspace/access generation, connectivity, and target; no resource mutation is queued for automatic replay after reconnect.
- [x] English/Chinese copy, screen-reader announcements, iOS native tabs, and Android navigation behave consistently. Mounted-provider and navigation/request tests demonstrate the transitions; both platform interaction checks and all required mobile checks pass.

## Source + Goal linkage

- **Source:** Mobile source review and $pm-brainstorm on 2026-09-07 at e609e9322, proposal 2; user handoff to $pm for w6. Follow-up to [w6/m138](../m138/README.md) and its t005 acceptance criteria, under [ADR087](../../../../docs/ADR087-mobile-role-views.md) sections Authorization and client contract / Access changes, offline state, and deep links.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) API-first operations and [ADR048](../../../../docs/ADR048-mobile.md) supervision: the phone must offer actions only with current affirmative access.
- **Expected outcome:** Foreground/reconnect restores verified membership and permissions before protected work; failed checks produce unavailable copy, and confirmed access loss clears affected content and pending operations.
- **Why now:** m138 shipped tab/query/action hide/show but its provider retains affirmative grants after failed refreshes without an expiry, uses a timer instead of a foreground access barrier, and treats an unavailable grant as a downgrade. These are concrete gaps in the completed implementation, not a new role model.
- **Render parity:** Included because this changes a tenant-facing native/API surface. ADR018's Workspace members & roles row and Role-aware dashboard controls note are already complete. Preserve bex's documented Contributor/Billing behavior and the native scope boundary; this is a native access-consumption correction, not a new Render capability.

## Review evidence and scope

- `mobile/src/features/capabilities/capabilities-provider.tsx:62` polls without a foreground/reconnect access barrier; lines 95–101 retain the last resolved grant set on a failed refresh.
- `mobile/src/features/capabilities/capability-policy.ts:101` counts allowed → unavailable as a downgrade, leading the provider to announce access.changed.
- `mobile/src/common/apollo/data-boundary.ts` aborts request leases and registered handlers; provider, navigation, and component-local state must participate in the access generation.
- `mobile/src/features/workspaces/workspace-provider.tsx` bootstraps membership but has no foreground membership-removal recovery.
- Review baseline: mobile typecheck and 330 unit tests passed. No physical-device proof was collected; pure policy tests do not establish mounted-provider request behavior.

Keep m138's role-aware navigation and the shared backend authorization authority. Resource precondition consumption belongs to m141; notification store/tap cleanup belongs to m140. Do not add mobile settings, environment-variable reads, destructive actions, wider OAuth scopes, or a new authorization engine.

## Completion evidence

Completed locally 2026-09-07. [Verification record](../../../../mobile/e2e/m139-access-recovery.md) maps the DoD to mounted-provider/request/confirmation tests, native Android/iOS interactions, parity and simplify reviews, and all required checks. 335 unit tests and ten mounted tests pass; both native exports pass. Physical push delivery/signing/live-agent gates remain in w11. No commit or push was requested.
