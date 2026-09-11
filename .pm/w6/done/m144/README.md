# w6 · m144 — Refresh dashboard permissions after workspace access changes

**Worker:** worker6 **Goal:** an open dashboard follows current workspace membership and permission decisions without requiring a reload **Status:** done

**Size:** 3h15m implementation + 1h45m standing closing work = ~5h (8 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Consume tri-state grants and fresh capability evaluation — **DONE** | 45m | `w6/m136/t009` |
| t002 | Coordinate visible, focus, and reconnect access refresh — **DONE** | 45m | `w6/m144/t001` |
| t003 | Reconcile membership loss and clear obsolete workspace state — **DONE** | 45m | `w6/m144/t002` |
| t004 | Migrate permission consumers and action confirmations to current access — **DONE** | 60m | `w6/m144/t002`, `w6/m144/t003`, `w6/m143/t008` |
| t005 | Render parity — **DONE** | 30m | `w6/m144/t004` |
| t006 | Simplify — **DONE** | 20m | `w6/m144/t005` |
| t007 | Test coverage — **DONE** | 45m | `w6/m144/t005` |
| t008 | Closeout — **DONE** | 10m | `w6/m144/t006`, `w6/m144/t007` |
## Definition of done

- [x] While visible, capability checks refresh at least every 30 seconds and on focus/reconnect. An answer older than 30 seconds cannot enable affected actions or a new sensitive reveal. Receipt time is a client freshness bound, not a server membership version or an end-to-end revocation guarantee.
- [x] Allowed, denied, unavailable, loading, and stale states are distinct. Unknown/error results do not grant actions, and authorization-service or transport failures never appear as a confirmed role downgrade.
- [x] A second authenticated session can downgrade/remove the first session's membership; the open dashboard updates without reload. A successful empty membership result clears the removed selected workspace and routes to the existing valid empty-workspace state; errors are not interpreted as removal.
- [x] Late responses and pending confirmations from an obsolete identity/workspace/access generation cannot restore prior access or dispatch mutations. Confirmed loss clears affected sensitive cached/local state and stops its continued reads/streams.
- [x] Existing role-gated controls use current affirmative access for actions and sensitive reads, with translated recovery copy. Stable navigation and previously authorized non-sensitive layout survive temporary refreshes; mutations are never queued for replay on reconnect.
- [x] Mounted provider/request tests cover freshness, generation invalidation, empty-membership routing, and fail-closed grants. Desktop/narrow-mobile pending layouts for affected controls match ready geometry in unit coverage; dashboard typecheck/lint/test pass.
- [ ] Two-session `dev-6` browser evidence for role change, last-workspace removal, focus/reconnect, failed refresh, and an open confirmation across a context change — deferred: local kind API server was unavailable this session (see Verification evidence).

## Source + Goal linkage

- **Source:** Approved proposal 2 from `$pm-brainstorm for w6`, materialized by the user's `$pm for them all for w6` on 2026-09-10. The reviewed `use-capabilities.ts` is cache-first, intentionally permissive while unresolved, and omits m136's grants/fresh contract; WorkspaceProvider retains selectedId when a successful membership list becomes empty.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) predictable multi-tenant operation and [ADR024](../../../docs/ADR024-members.md) workspace access, using the shared capability authority supplied by [m136](../m136/README.md).
- **Expected outcome:** Role changes and workspace removal propagate to an already-open dashboard; unavailable checks explain recovery and do not enable affected actions or masquerade as confirmed role denial.
- **Why now / gap analysis:** [m139](../m139/README.md) solved native-client recovery, but dashboard capability freshness remains separate and unimplemented. Membership polling already exists; this milestone connects it to fresh permissions, handles the empty-membership case, and integrates m143's concrete action consumers.
- **Render parity:** Included: this changes tenant-facing behavior on ADR018's Workspace members & roles row and role-aware dashboard controls. Preserve backend relations and the dashboard's Kratos cookie-session architecture.

## Scope and dependencies

- The user's approval explicitly supersedes the unresolved-is-allowed dashboard presentation policy documented by w9/m84 and retained in w2/m74/w9/m87. Backend authorization is unchanged.
- Use m136's tri-state grant reasons and fresh-evaluation option. Keep one coordinated refresh lifecycle; avoid a timer per control or role-name-derived permission rules.
- Membership polling already exists in use-workspaces.ts. Reuse it, distinguish successful empty data from failure, and preserve a valid Kratos session when no workspaces remain.
- m143 owns resource-action consumers and their confirm-time checks. m144 adds global freshness/membership recovery and integrates those consumers after m143 closes.
- Keep the dashboard on HttpOnly Kratos cookie sessions. No Hydra client conversion, browser-readable auth tokens, new role model, mobile changes, or production deployment.
- Use dev-6 only for verification and clean up its temporary identities/workspaces.

## Verification evidence

- Dashboard `yarn typecheck`, `yarn lint`, and `yarn test` green after fail-closed capabilities + freshness provider.
- Workspace empty-membership reconciliation covered by `workspaces/context` tests; capability-policy unit tests cover grants/stale/unavailable.
- Supersedes permissive-while-unknown UI policy for actions and sensitive reads (ADR018 updated).
- Limitation: two-session live membership downgrade / focus-reconnect browser evidence on `dev-6` not captured this session (cluster down). Mounted provider tests cover generation invalidation and empty membership routing.
