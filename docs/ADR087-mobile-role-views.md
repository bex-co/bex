# ADR087 — Mobile views by workspace role

**Status:** Proposed · 2026-09-06.  
**Audience:** Product, design, mobile, backend, and QA.  
**Related:** [ADR048 — Mobile](ADR048-mobile.md), [ADR024 — Members](ADR024-members.md), [ADR012 — Auth](ADR012-auth.md), [ADR047 — Agent sessions](ADR047-cloud-coding-agent-sessions.md), and [ADR052 — Notifications](ADR052-notifications.md).

## Decision in one paragraph

Ship one mobile app whose content and actions follow the caller's effective permissions in the **selected workspace**. Keep the five existing roles: Admin, Developer, Contributor, Viewer, and Billing. Use server capabilities to decide access; use role names to explain it. Viewers and billing members receive read-only supervision; contributors also receive logs, supported lifecycle controls, and agent supervision; developers and admins can additionally roll back deployments and delegate agent work. Admin privileges do not expand mobile into a configuration or administration product. Hide unauthorized content and avoid requesting it; disable an authorized action when a temporary condition prevents it. The API remains the authority for every read and action.

This ADR specifies proposed product behavior and implementation requirements. It does not claim these view gates, capability extensions, or notification corrections have shipped.

## Context and research

The customer question is: **“What can I see and do in this workspace from my phone?”** A role label alone cannot answer it. The same person can administer one workspace and observe another; the native token can be narrower than the person's desktop session; a resource's state can prevent an otherwise permitted action.

The product objective is to shorten the path from an alert to an informed, permitted response without presenting dead-end controls or leaking restricted information through previews, counts, notifications, or old screen state.

### What the current repository establishes

Research inspected the checkout at `39353418a9f0f0e4681f533b3bad04cd22115f8a`, including existing working-tree mobile navigation changes, on 2026-09-06. These are static code findings, not a production penetration test or a claim about deployed binaries.

| Finding | Evidence and implication |
| --- | --- |
| Roles are workspace-scoped and are not one numeric ladder. | The [OpenFGA model](../deploy/gitops/authz/model.fga) grants `can_view` to all five roles; `can_view_logs` and `can_operate` to Contributor/Developer/Admin; `can_create` to Developer/Admin; `can_manage_billing` to Billing/Admin. Organization-admin inheritance can exceed what a membership label suggests. |
| Mobile has a narrower credential grant than desktop. | [Native OAuth scopes](../mobile/src/features/auth/expo-oauth-transport.ts) are `openid offline_access bex.read bex.write`, without `bex.sensitive`. The [Core scope map](../lego/backend/internal/core/scope.go) intersects OAuth scope with authorization even for first-party public clients. Logs require `bex.read`; agent reads use `can_operate` and therefore also require `bex.write`. |
| The shared capability query already exists, but is coarse. | [members.Service.Capabilities](../lego/backend/internal/members/service.go) and its [GraphQL contract](../lego/backend/internal/members/graphql.go) expose `viewerCapabilities(ownerId)`, eight booleans, and a best-effort role. They do not expose per-resource decisions, action reasons, freshness, or protection preconditions. [Core.Can](../lego/backend/internal/core/base.go) maps checker errors to `false`; false does not establish that the member's role caused the refusal. |
| Mobile currently emphasizes resource state over access. | [Service actions](../mobile/src/features/services/service-actions-card.tsx) construct controls from lifecycle/deploy state; [session listing](../mobile/src/features/agent-sessions/sessions-list-screen.tsx) skips its query only when no workspace is selected. The [workspace provider](../mobile/src/features/workspaces/workspace-provider.tsx) carries a role, but there is no shared mobile equivalent of the dashboard capability hook. |
| Contributor rollback is forbidden in bex. | [deploys.Service.Rollback](../lego/backend/internal/deploys/service.go) requires `can_create`; bare Trigger and Cancel require `can_operate`. The [executable-selection regression policy](../lego/backend/internal/api/m68_executable_selection_test.go) treats rollback, arbitrary commits/images, commands, and build-source changes as choosing executable content. |
| “Read-only” does not mean every read is available. | [Agent List/Get/Transcript](../lego/backend/internal/agentsessions/service.go) require `can_operate`, so Viewer/Billing cannot read sessions. [databaseProcesses](../lego/backend/internal/api/scope_matrix_overrides.go) requires the sensitive OAuth scope even when a client omits SQL text. The [mobile insights card](../mobile/src/features/postgres/postgres-insights-card.tsx) currently requests it alongside ordinary telemetry. |
| Billing currently has general supervision reads in bex. | [Service metrics](../lego/backend/internal/metrics/service.go), [events](../lego/backend/internal/events/service.go), [deploy history](../lego/backend/internal/deploys/service.go), and [usage](../lego/backend/internal/usage/service.go) use `can_view`. The “names only” shorthand in ADR024 is narrower than this implementation. This ADR retains current bex authorization; it does not silently introduce a new financial-only role. |
| Notification access can disagree with destination access. | [Agent push projection](../lego/backend/internal/notifications/push_worker.go) uses fan-out without `EligibleRoles`; the [delivery policy](../lego/backend/internal/notifications/delivery_policy.go) treats nil eligibility as all roles. The [stored inbox](../lego/backend/internal/notifications/inbox.go) checks workspace/subject and `can_view`, without checking the destination's relation. Thus the static path permits agent metadata for a Viewer/Billing recipient whose session read is refused. |

**Resolve existing documentation conflicts explicitly.** The current [mobile scope instructions](../mobile/CLAUDE.md) and [scope-policy test](../mobile/src/__tests__/mobile-scope-policy.test.ts) exclude all environment-variable viewing and editing, including keys. They take precedence over ADR048 D3's historical one-variable exception. Likewise, the current native scope list above supersedes ADR048 D7's historical `openid offline_access` description. This ADR does not restore environment cards or request a sensitive scope.

### External evidence and product judgment

All external sources below were checked on 2026-09-06. Dates are publication/update dates when the source displays one; undated documentation is identified as such.

| Primary source | Finding | Decision for bex |
| --- | --- | --- |
| Render, [Workspaces, Members, and Roles](https://render.com/docs/team-members), undated | Five roles; Viewer lacks logs; Billing lacks observability; Contributor can deploy and roll back. Roles can differ between workspaces. | Retain familiar names, but document the two material bex differences: Billing's existing general reads and Contributor's inability to roll back. This is not exact Render permission parity. |
| Render, [Projects and Environments](https://render.com/docs/projects#protected-environments), undated | Protected environments restrict particular non-admin actions; protection is not a blanket prohibition on all operations. Blueprint-mediated changes have an explicit caveat. | Inspect bex's actual preconditions. Do not copy a competitor's permission matrix or infer restrictions from an environment named “Production.” |
| GitHub, [GitHub Mobile](https://docs.github.com/en/get-started/using-github/github-mobile) and [Configuring notifications](https://docs.github.com/en/subscriptions-and-notifications/get-started/configuring-notifications#managing-your-notification-settings-with-github-mobile), undated | Mobile supports triage/review and explicit account switching; notification controls include event preferences and working hours. | Preserve supervision, visible workspace context, and relevant notifications. These documents do not establish GitHub's internal role-gating or revocation implementation. |
| Apple, [Tab bars](https://developer.apple.com/design/human-interface-guidelines/tab-bars), updated 2026-06-08 | Apple recommends retaining tab buttons when their content is unavailable and explaining empty sections. | Keep navigation stable during loading, empty results, and outages. The role-dependent Sessions tab below is a deliberate product tradeoff, not an Apple recommendation. |
| Apple, [Menus](https://developer.apple.com/design/human-interface-guidelines/menus), updated 2026-06-08; [Accessibility](https://developer.apple.com/design/human-interface-guidelines/accessibility), updated 2025-06-09 | Unavailable commands can be dimmed; state needs more than color and must be understandable with assistive technology. | Use visible reasons for temporary action blocks. Omitting unauthorized content is our product rule; these guidelines do not mandate a universal hide-versus-disable policy. |
| OWASP, [Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html), undated | Default-deny behavior, object-specific checks on every request, and server enforcement are central requirements. | A hidden button, route guard, role badge, or notification link never grants access. |
| Expo, [Protected routes](https://docs.expo.dev/router/advanced/protected/), updated 2026-05-23 | Guards prevent client navigation and can remove inaccessible history entries; they do not replace server access control. | Guard deep links and mounted screens as well as navigation entries. Validate the actual iOS native-tabs and Android navigator behavior used by this checkout. |
| OpenFGA, [Query Consistency Modes](https://openfga.dev/docs/interacting/consistency), undated | Query caching can delay observation of relationship changes; higher-consistency queries skip that cache. | State the consistency boundary and test revocation. A client refetch is not proof of instantaneous server-side revocation. |

Apple's text and update dates were verified through its official documentation JSON for [tabs](https://developer.apple.com/tutorials/data/design/human-interface-guidelines/tab-bars.json), [menus](https://developer.apple.com/tutorials/data/design/human-interface-guidelines/menus.json), and [accessibility](https://developer.apple.com/tutorials/data/design/human-interface-guidelines/accessibility.json).

The evidence supports the permission model and navigation tradeoffs. It does not establish which layout users prefer: the role journeys and usability targets below are product hypotheses to validate.

## Role experiences

Each role lands on **Status**, with the selected workspace and translated role available in the existing drawer. There is no global “admin mode,” role impersonation, or role picker. A missing role label means “Role unavailable”; it never falls back to Admin or overrides an affirmative server capability.

| Role | Job on the phone | Experience and boundary |
| --- | --- | --- |
| **Viewer** | Determine whether the system is healthy and understand what changed. | Status, metrics, deploy history, events, non-sensitive datastore evidence, operational usage, and eligible notifications. No logs, resource controls, or session content. An empty workspace has an explanatory state with no service-creation CTA. |
| **Contributor** | Diagnose and operate existing workloads; supervise ongoing agent work. | Viewer reads plus logs, supported restart/suspend/resume, bare redeploy, deploy cancellation, cron run/cancel, and session status/cancellation. No rollback, composer, steering, live prompt submission, or session resume. “View session” must not imply “can delegate.” |
| **Developer** | Recover workloads and delegate bounded coding tasks. | Contributor experience plus rollback to an eligible deployment and agent composition. Later steering/live turns require the same create permission and their separately qualified mobile implementation. Configuration, credentials, and destructive administration remain desktop work. |
| **Admin** | Coordinate recovery and delegation across a workspace. | The same native operational experience as Developer. Membership changes, workspace settings, payment management, and permission bypasses do not appear merely because the user is an admin. |
| **Billing** | Understand operational consumption and know where billing work belongs. | The read-only supervision experience currently authorized by bex, including the existing month-to-date usage glance. No logs, resource controls, or sessions. Explain that invoices/payment management belong to the dashboard; do not fabricate native billing totals or a billing tab. |

Usage remains the existing **meter totals plus collection coverage**, not estimated charges, invoices, balances, or a claim that partial telemetry is a complete bill. Its [minimal mobile query](../mobile/src/features/usage/api/usage.graphql) is the contract for every role.

Where existing mobile scope permits a dashboard handoff, display billing-management guidance only with confirmed `canManageBilling`. The browser authenticates independently, and the user must be able to verify the destination workspace. Never pass native credentials in a URL or silently assume the browser selected the same workspace. This ADR adds no native billing workflow or new handoff protocol.

## Navigation and content matrix

### Top-level navigation

The reference order is **Status → Activity → Sessions → Notifications**.

| Destination   | Viewer | Contributor | Developer | Admin | Billing |
| ------------- | ------ | ----------- | --------- | ----- | ------- |
| Status        | Show   | Show        | Show      | Show  | Show    |
| Activity      | Show   | Show        | Show      | Show  | Show    |
| Sessions      | Hide   | Show        | Show      | Show  | Hide    |
| Notifications | Show   | Show        | Show      | Show  | Show    |

These are expected results for healthy grants under the current native scopes, not a role switch statement. Status/Activity require confirmed `canView`; Sessions requires confirmed `canOperate`; Notifications requires caller/workspace inbox access and the item filtering below.

**Why hide Sessions:** Viewer/Billing cannot read even a session list under the current backend contract. A permanent unavailable destination would consume a large part of a three-to-four-tab interface without supporting their work. This deliberately departs from Apple's general tab-stability guidance. Keep the relative order and route identity of all surviving tabs; change the set only after a resolved workspace/access transition. Never remove a tab because its list is empty, a feature is temporarily unavailable, a request is loading, or the network failed.

An authorized but unconfigured Sessions destination explains setup requirements. Only a caller allowed to create sees the composer or its setup guidance. A contributor sees “No sessions yet” without a create CTA or model-key/GitHub configuration controls.

If Sessions becomes inaccessible while open, clear its content, remove its history entries, return to Status, and announce the access change. A transport error must not masquerade as a confirmed role change.

### Detail sections and actions

**Show** means both render and request the permitted data. **Allow** still requires a supported resource, current state, successful preconditions, and confirmation where applicable. **Hide** means no control, preview, count, or associated query.

| Content/action | Viewer | Contributor | Developer | Admin | Billing | Governing rule |
| --- | --- | --- | --- | --- | --- | --- |
| Service status, deploy history, events, ordinary metrics | Show | Show | Show | Show | Show | `canView`; authorize the target |
| Postgres/KV overview and ordinary capacity/connection-count metrics | Show | Show | Show | Show | Show | `canView`; no connection credentials |
| Postgres sizes and table-scan statistics | Show | Show | Show | Show | Show | Existing non-sensitive reads |
| SQL process list and top queries | Hide | Hide | Hide | Hide | Hide | Current APIs require sensitive scope; omitting SQL fields does not lower the gate |
| Service/build logs where supported | Hide | Show | Show | Show | Hide | `canViewLogs`; gate HTTP query and SSE connection |
| Bare redeploy and cancel deploy | Hide | Allow | Allow | Allow | Hide | `canOperate`; no commit/image override |
| Rollback to an eligible previous deploy | Hide | Hide | Allow | Allow | Hide | `canCreate`; exact target still validated |
| Service/Postgres restart where supported | Hide | Allow | Allow | Allow | Hide | `canOperate`; no invented KV restart |
| Service/Postgres/KV suspend/resume | Hide | Allow | Allow | Allow | Hide | `canOperate`; protection/billing/state preconditions |
| Cron history | Show | Show | Show | Show | Show | `canView`; values-free run metadata |
| Cron run-now/cancel active run | Hide | Allow | Allow | Allow | Hide | `canOperate`; no schedule/command editor |
| Session list/detail and eligible PR link | Hide | Show | Show | Show | Hide | Actual session read gate is `canOperate` |
| Cancel an eligible active session | Hide | Allow | Allow | Allow | Hide | `canOperate`; preserve mobile phase restrictions |
| Create an agent task | Hide | Hide | Allow | Allow | Hide | `canCreate` plus feature/GitHub/model readiness |
| Personal notification opt-in/out and mark-read | Allow | Allow | Allow | Allow | Allow | Own installation/inbox only; not a resource-operation grant |
| Invite acceptance | Allow | Allow | Allow | Allow | Allow | Existing authenticated token-redemption flow; refresh membership after success |
| Environment keys/values, secrets, connection strings, API/SSH keys | Hide | Hide | Hide | Hide | Hide | Outside mobile scope even if backend role permits them |
| Service creation, settings, topology, admin, delete/PITR/failover, Web Shell | Hide | Hide | Hide | Hide | Hide | Outside mobile scope for every role |

The action gates are established by [deploys](../lego/backend/internal/deploys/service.go), [apps](../lego/backend/internal/apps/service.go), [Postgres lifecycle](../lego/backend/internal/postgres/lifecycle.go), [Key Value](../lego/backend/internal/keyvalue/service.go), and [agent sessions](../lego/backend/internal/agentsessions/service.go). The row for SQL processes corrects an existing mobile request mismatch; it does not weaken the API.

**Future session features remain distinct.** Transcript replay/read tickets require `canOperate`; steering, live prompt-turn tickets, and resume/rehydration require `canCreate` (Resume also has an outer operate check). Apply those gates when the corresponding ADR048 milestone ships. Do not add those features, archive/delete, or a generic approval/bypass mode as part of view gating. A PR link is an external GitHub action; GitHub independently determines repository/review access.

## Show, hide, disable, and explain

Apply these rules in order. Permission denial is different from a feature that the user may access but cannot use now.

| Situation | Presentation | Data and interaction behavior |
| --- | --- | --- |
| Outside mobile scope | Omit the native feature for every role | No route, field selection, background request, or mutation adapter |
| Confirmed absence of access | Hide the section/action and close any mounted instance | No restricted fetch, polling, SSE, prefetch, count, or cached preview; do not show an empty “Actions” card |
| Permission unresolved, expired, or unavailable | Neutral checking/unavailable state | Never infer permission from a role or stale affirmative result; no new protected work |
| Authorized action, temporarily blocked | Keep it disabled with a nearby reason | Examples: offline, suspended service, operation pending, no eligible rollback target, billing precondition |
| Authorized read, legitimately empty | Show the correct empty state | “No deployments yet” is valid only after a permitted successful read |
| Authorized feature, setup missing | Explain setup in the existing destination | Show only guidance/actions permitted for that caller; no native secret/configuration editor |
| Direct link cannot be opened | Generic access/unavailable state with Back to Status | Do not echo an unverified resource name or distinguish a foreign ID from a nonexistent one |

Resource-type-specific actions can be absent when the operation does not exist at all. Temporary unavailability of a supported operation should have an explanation. Capability-dependent pending layouts preserve the ready layout's geometry once the visible regions are known; before that, use a neutral shell without restricted labels or counts.

Do not scatter “upgrade” or “ask an admin” placeholders wherever an item is omitted. The drawer's role explanation provides general discoverability. Show request-access guidance only after an explicit denied navigation/action with a reliable permission reason. There is no access-request submission API in this ADR.

Suggested copy is product text, not raw server errors. Implementation must use `useTranslations()` and supply both locales:

| Key / situation | English | Chinese |
| --- | --- | --- |
| `access.checking` | Checking access… | 正在检查访问权限… |
| `access.unavailable` | We couldn't check your access. Try again. | 无法检查你的访问权限，请重试。 |
| `access.changed` | Your access in this workspace has changed. | 你在此工作区的访问权限已更改。 |
| `access.cannotOpen` | This item isn't available with your current access. | 你当前的访问权限无法查看此内容。 |
| `access.requestHelp` | Ask a workspace admin to review your access. | 请联系工作区管理员确认你的访问权限。 |
| `access.offlineAction` | Connect to the internet to continue. | 请连接网络后继续。 |
| `access.nativeLimit` | This feature is available in the dashboard. | 此功能可在网页控制台中使用。 |
| `access.roleUnknown` | Role unavailable | 暂时无法获取角色信息 |

Reasons remain visible and screen-reader accessible; they cannot depend on hovering, color, or a lock icon alone. After a permission-driven redirect, move focus to the destination heading and announce the change. Use existing theme tokens, scalable text, reduced-motion behavior, and `useWindowDimensions`.

## Authorization and client contract

### One policy authority, several presentation gates

For an action, the product rule is:

```text
show = in mobile scope AND confirmed permission for this workspace/target
enable = show AND feature available AND resource eligible AND online
         AND required confirmation complete AND no operation already pending
```

The server computes permission from current identity, granted OAuth scopes, workspace membership/inheritance, and target ownership. Feature readiness, state, billing prerequisites, and confirmation are additional conditions, not new roles. Frontend lifecycle predicates may explain readiness; they never manufacture authorization.

Reuse the backend's shared Core checks and the existing capabilities feature. Do not build a second role-to-permission engine in mobile, derive roles from boolean combinations, use the dashboard's **permissive-while-unknown** [capability hook](../dashboard/src/features/capabilities/hooks/use-capabilities.ts), or edit generated GraphQL by hand.

### Required contract work

The existing `viewerCapabilities(ownerId)` is the starting point. The following are **proposed additive contract requirements**, not fields available today:

1. **Explicit evaluation outcome.** Each relevant grant must distinguish allowed, denied, and unavailable/unknown. Carry bounded, non-sensitive reason codes when known, distinguishing missing OAuth scope, insufficient permission, and authorization-service failure. A failed checker must not be converted into a confident “your role forbids this” message. Legacy false remains restrictive but reason-unknown.
2. **Explicit context.** The client supplies the selected workspace. Resource decisions additionally bind the exact resource kind/id and reject a target outside that context. Identity comes from authentication, never a client-supplied subject. Do not silently substitute the caller's default workspace.
3. **Target/action detail where needed.** Extend the shared capability projection to express existing verbs and their preconditions, using the same helpers as their execution paths. Workspace `canOperate` alone cannot promise that a specific resource can be suspended; `canCreate` does not imply every desktop creation workflow belongs on mobile. Probes must be read-only and have no billing, provisioning, or mutation side effects.
4. **Freshness semantics.** Document whether a response used cached checks and support fresh evaluation for recovery after an access change. Client receipt time is “last checked,” not proof of the underlying membership version. Do not invent a security guarantee from an arbitrary timestamp.
5. **Compatibility.** Keep existing callers compatible and the backend's REST/GraphQL/MCP semantics aligned wherever the capability contract is exposed. Mobile selects only the required fields through codegen. Unknown action IDs/reasons fail closed and use generic localized copy.

Preparatory implementation may use the existing booleans to suppress work safely with generic copy. That alone does not satisfy the launch criteria: reason-specific UX, reliable role-change detection, and complete resource-action gating require the corresponding contract gaps to be closed.

### Queries and resource preconditions

Gate query mounting as well as components. Split optional restricted reads from ordinary supervision so a refusal cannot blank an otherwise readable service screen. For example, remove the mobile process-list request; keep capacity, sizes, and table-scan cards available. Hide Logs for Viewer/Billing before mounting either its historical request or live stream. Gate session composer readiness/repository queries on create access; a “ready” integration is not a permission grant.

A resource load must prove access to the exact target in the selected workspace before its title, previews, and actions appear. Never obtain a list broadly and filter unauthorized rows only after receipt.

bex's current protected-environment behavior is an **extra confirmation on selected operations**, rather than Render's blanket non-admin restriction for those operations. See [service protection](../lego/backend/internal/apps/protection.go) and [datastore confirmation](../lego/backend/internal/core/confirmation.go). Do not implement `protected && !admin` as an invented policy. Present the precise action/target and required confirmation through the existing safe-action flow; never silently fill a confirmation obtained from an error and retry.

Before dispatch, freeze the subject/session boundary, workspace, resource, verb, and target revision/deploy/run. Recheck permission freshness and eligibility. Any boundary change dismisses the confirmation. Existing server checks still run when the mutation arrives; a capability snapshot is not a bearer grant. Preserve single-flight behavior, bounded requests, and authoritative reconciliation after ambiguous outcomes. Never queue resource mutations for automatic execution after reconnect.

## Access changes, offline state, and deep links

Extend the existing [DataBoundary](../mobile/src/common/apollo/data-boundary.ts) lifecycle to include access changes inside the same workspace, not just workspace switches/logout. Key permission state and related work by identity/session, workspace, and boundary generation; add resource identity for target decisions. Keep authorization snapshots in memory.

| Transition | Required behavior |
| --- | --- |
| Cold start / refreshed login | Restore authentication and verified workspace membership, then obtain capabilities before mounting protected queries/actions. A persisted workspace ID is a preference, not evidence of membership. |
| Workspace A → B | Freeze interactions; abort requests/streams; clear cached protected data, confirmations, drafts, inbox, and old navigation history; resolve B and its capabilities; then render. A late A response cannot publish in B. |
| Foreground / reconnect / return from dashboard | Refresh membership and access before restoring protected work. Do not replay an old action. |
| Same-workspace downgrade | Invalidate the access generation; clear affected data and pending controls immediately when detected; refresh capabilities; remove inaccessible history. |
| Upgrade | Enable newly permitted views only after affirmative current capabilities. No background submission of a previously refused action. |
| Membership removal | Clear that workspace's data and selection; return to the verified workspace chooser. If none remain, retain the existing empty state and logout path. |
| Auth failure | Follow the reviewed session-manager refresh/sign-out contract. A resource-level 403 is not, by itself, a reason to log the person out. |
| Unknown/foreign/deleted deep link | Validate the existing route/ID allowlist, authentication, workspace binding, and destination permission. Present generic unavailable copy if any requirement fails. |
| Offline | Disable all resource actions and new protected navigation. Same-boundary, previously authorized non-sensitive snapshots may remain visibly stale; clear logs, transcripts, prompts, and transient sensitive content. Cold start and workspace switching do not reconstruct access from persisted content. |

While foregrounded, re-evaluate access at least every 30 seconds and on the transitions above. Treat a snapshot older than that interval as insufficient to enable actions or open restricted content; an in-flight refresh may preserve the established shell and labeled non-sensitive observations. Failed refreshes show an unavailable state, not a role-change announcement. This is a proposed client freshness policy, not a measured end-to-end revocation guarantee.

The backend currently has a [30-second positive-check cache](../lego/backend/internal/authz/authz.go), configured in [Core HTTP wiring](../lego/backend/internal/core/http.go). Revocation evicts positives on the processing replica; other replicas can retain them until expiry. Gates using write relations such as `can_operate` and `can_create` use [fresh checks](../lego/backend/internal/core/base.go), including session reads that use `can_operate`; not every HTTP mutation is classified that way. Membership-to-OpenFGA reconciliation and [stream revalidation](../lego/backend/internal/core/revalidate.go) add separate timing boundaries.

Therefore a refresh cannot promise immediate removal everywhere. Measure the complete path from membership change through reconciliation, API replicas, and client refresh; assert prompt local clearing **after detection**. Offline devices and already delivered content cannot be remotely made to forget bytes they received.

Notification links must also pass the existing [subject/workspace/session binding](../mobile/src/features/notifications/deep-link.ts). Do not auto-switch workspaces or execute a mutation from an incoming envelope. Invitation links retain their separately reviewed redemption flow; an invitation is not workspace membership until acceptance succeeds and membership is refreshed.

## Notifications follow destination access

A notification preference expresses interest, not permission. The same item must not be inaccessible in Sessions but visible as a repository name, PR URL, preview, or unread count in Notifications.

| Event family | Expected recipients under current grants | Content/action boundary |
| --- | --- | --- |
| Service/deploy/cron supervision | All five roles with target read access and matching preferences | Non-sensitive status summary; logs and actions are separately gated after opening |
| Agent completion, failure, PR ready | Contributor/Developer/Admin with session read access | No Viewer/Billing title, body, repository/PR metadata, badge contribution, or inbox row |
| Future agent decision request | Recipient must have the relevant decision/turn permission | A Contributor must not be asked to approve or submit work that requires create access; a separately designed read-only status event can use read eligibility |
| Usage/billing alerts | Only when an implemented event and authorized mobile destination exist | The current mobile usage glance does not create a billing notification workflow |

Implement one destination-eligibility policy in the notification backend and reuse it when projecting recipients, immediately before deferred/retried delivery, and when returning stored inbox items and unread counts. Check current resource access rather than merely copying the old recipient role into an event. A role downgrade must remove unauthorized historic items from the API's visible projection and badge count, even if durable records are retained internally. Authorization outages defer or suppress disclosure; they do not broaden recipients.

Close the static agent fan-out/inbox mismatch identified above before claiming this policy complete. Client filtering is useful defense in depth but cannot fix server disclosure or an OS-rendered notification.

Keep lock-screen text generic and free of repository names, prompts, log excerpts, credentials, and resource details; fetch the authorized detail after opening. Preserve the existing closed routing envelope. Revalidate at open even after a successful send-time check. Delivery providers and already delivered OS notifications cannot be reliably recalled, so minimize their content from the outset.

Personal device opt-in/out and mark-read remain available to read-only members. Workspace-wide event/webhook configuration stays on the dashboard. Respect the existing event preferences, urgency, and working/quiet-hours policy rather than deriving subscriptions solely from a role.

## Alternatives and consequences

| Alternative | Assessment |
| --- | --- |
| Hardcode screens against role names | Simple initially, but wrong for OAuth scope, inherited grants, resource context, and role/capability reconciliation. Rejected. |
| Show everything and handle 403 after tap | Produces dead ends and can fetch restricted previews before rendering. Rejected. |
| Keep every role's tabs identical with a permanently restricted Sessions page | Closest to Apple's general recommendation and easier for cross-workspace muscle memory. Rejected for this release because Viewer/Billing cannot complete any session job; revisit if usability evidence shows context-dependent tabs confuse users. |
| Clone Render's exact matrix | Familiar, but would change bex Billing reads, Contributor rollback, and protection behavior. Requires a separate platform authorization decision. |
| Request sensitive OAuth access to make existing cards work | Broadens the native credential and conflicts with mobile scope. Omit the SQL-process/secret surfaces instead. |
| Build separate admin, engineering, and finance apps | Duplicates navigation and state boundaries while the core supervision journeys overlap. Rejected. |

This decision adds backend projection and notification work beyond a cosmetic hide/show pass. It reduces misleading actions and unnecessary requests, but requires careful navigation transitions and explicit unavailable states. Admin and Developer intentionally look similar on mobile; Billing's native value is operational context, with financial management left to the dashboard.

## Delivery, acceptance, and measurement

### Delivery sequence

1. **Backend contract and policy:** verify the action matrix, add reliable capability outcomes and required target preconditions, and close notification eligibility/inbox gaps. Preserve existing authorization; do not alter roles or widen native scopes.
2. **Mobile integration:** add a fail-closed capability provider, conditional navigation, query/action gates, same-workspace access invalidation, bilingual copy, and minimal telemetry. Regenerate GraphQL and extend the exact scope allowlist for the reviewed capability operations only.
3. **Qualification:** run the role/context tests and signed-device navigation/notification cases below. Ship the view policy only after its mandatory gates pass. Later transcript/steering features remain on their own milestones.

These are implementation stages, not new PM-board entries or delivery-date commitments. This document adds no runtime implementation.

### Acceptance criteria

| Scenario | Required evidence |
| --- | --- |
| All five roles, ordinary resources | Render and network assertions match both matrices. Hidden regions issue zero associated requests, including through prefetch and restored navigation. |
| Viewer/Billing session and log links | No session/log metadata is rendered or returned through a broader notification path. Direct backend requests are refused. |
| Contributor | Bare deploy, cancel, supported lifecycle, and cron actions work; rollback, arbitrary executable selection, composer, steering, and resume are refused and absent. |
| Native Admin/Developer | Expected mobile actions work, but no env keys/values, SQL-process request, connection credentials, secret scope, admin screens, or destructive controls appear. |
| Protected or otherwise blocked resource | Exact backend precondition determines readiness. No inferred admin bypass, silent confirmation, or optimistic success. |
| Different roles in A and B; account switch | No data, badge, confirmation, request completion, or back-stack entry crosses the boundary. Role captions never grant access. |
| Same-workspace role change during open detail, stream, or confirmation | Affected content clears on detection, streams stop, pending submissions are canceled, and the new capability result governs subsequent work. |
| Authorization checker outage, null snapshot, missing OAuth scope, unknown enum | Fail closed; reason stays accurate; no “you were demoted” inference, infinite retry, or permissive fallback. |
| Read-only versus unavailable | Legitimate empty states differ from forbidden, partial telemetry, stale data, and network errors. Permitted non-sensitive cards survive an unrelated restricted-read refusal. |
| Push queued before downgrade; historical inbox after downgrade | Delivery, inbox pages, counts, and open-time access agree with current target eligibility. Generic OS content discloses no restricted metadata. |
| Invite accepted / expired / another identity | The reviewed invite contract remains unchanged; a successful acceptance refreshes membership and capabilities without treating the invited role as a prior grant. |
| iOS and Android, narrow widths, large text, English/Chinese, screen readers | Stable tab order, correct focus/history after access changes, complete translated reasons, and accessible disabled controls. |

Use backend authorization/notification tests, mobile query/render integration tests, and physical-device evidence for OS behavior. Scope-policy tests remain mandatory but do not substitute for behavioral authorization tests. Run the repository's required checks for the implementation's affected modules; for mobile this includes formatting, types, lint, unit tests, Expo checks, and both platform exports.

### Success measures

These are proposed launch targets, not measured results:

- **Correctness:** zero unauthorized content/request/badge disclosures and zero wrong-workspace submissions in the acceptance matrix. Any failure blocks rollout.
- **Discoverability:** in moderated tests covering all five roles, at least 90% of participants identify their workspace and whether they can perform the assigned task within 10 seconds. Include users who switch between different roles.
- **Task completion:** at least 90% complete their role-appropriate supervision task without tapping a permission-denied control. Measure time to evidence separately from time to action.
- **Production diagnostics:** track actionable controls refused for permission after being shown enabled, capability-unavailable rates, and access-change propagation time. Separate races from persistent mapping bugs; collect a baseline before setting latency/SLO claims.

Diagnostics use bounded action/screen identifiers and reason codes, with only approved coarse role categories if needed. Never record credentials, invitation tokens, prompts, log contents, repository names, raw API errors, or notification bodies.

### Remaining decisions

No unanswered item changes the default matrices above. Product should validate the Sessions-tab tradeoff and whether Billing users find the operational-only mobile experience useful. If Billing must become names-only, or agent transcript access must extend to Viewers, propose a separate backend permission change with matching API and notification behavior. A redacted SQL-process endpoint, richer finance mobile features, and native agent approvals likewise need separate scope decisions.
