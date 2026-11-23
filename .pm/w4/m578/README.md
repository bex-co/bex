# w4 · m578 — In a protected environment a Developer sees every per-resource Suspend and Resume disabled with Render's reason, never a 403 after the click

**Worker:** worker4 **Goal:** the per-resource lifecycle projections (`serverActions`, `databaseActions`, `keyValueActions`) answer suspend and resume by the same rule the verbs enforce: in a protected environment only a workspace Admin may run them, as on Render. Every per-resource Suspend and Resume in the dashboard (the service row menu, the service header and Settings card, the database header menu and Info actions, the Key Value Info actions) is then disabled with Render's _This action is limited to admins._ for a Developer there, and enabled for an Admin. The bulk toolbar already gates this way (w4/m550). **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The three lifecycle projections deny suspend and resume to a non-Admin on a protected member, by the verbs' own rule | 50m | — |
| [t002](t002.md) | The service row menu, header and Settings card read Render's admin-only reason | 30m | t001 |
| [t003](t003.md) | Database and Key Value Suspend and Resume gate on their projections | 1h | t001 |
| [t004](t004.md) | Live check on dev-4 as an Admin, a Developer and a Viewer at 1440 and 390 | 30m | t002, t003 |
| [t005](t005.md) | Render parity | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 4h05m**.

## Definition of done

On dev-4 with OpenFGA enforced (w4/m550 t005's recipe), then on production once the pin carries the change, as far as its one QA identity (an Admin) allows:

- In a protected environment, a Developer sees Suspend and Resume disabled with _This action is limited to admins._ on a service's row menu, its header and its Settings card, on a database's header menu and Info actions, and on a Key Value's Info actions. Clicking sends no lifecycle call.
- An Admin there suspends and resumes each of them. In an unprotected environment the Developer does too. A Viewer reads the role's view-only reason everywhere.
- `serverActions`, `databaseActions` and `keyValueActions` report `suspend` and `resume` as `denied` for a non-Admin on a protected member. A backend test per package asserts that the projection and the verb agree for an Admin and a Developer, protected and not.
- REST and MCP are unchanged: the verbs already refuse a non-Admin there.

## Source + Goal linkage

- **Source:** inbox note w4/713 ([`source-713.md`](source-713.md)), filed by w4/m550 t004 on 2026-11-21. Promoted by `/loopx w4` on 2026-11-22. The fix touches three backend projections, the service surfaces' copy, and three datastore components that read no projection yet. That is about 2h40m of implementation over three tasks, past the inbox's hour.
- **Premise checked 2026-11-22 on `ba7e6c069e`:**
  - **Services.** `apps.ActionCapabilities` (`lego/backend/internal/apps/actioncaps.go:41-91`) decides suspend and resume on `can_operate` alone. For a protected member, `suspendPrecondition` (:103-107) adds `protected_confirmation_required`. `setSuspended` (`apps/service.go:6132-6150`) calls `requireUnprotected(ctx, a, "suspend"|"resume")` (`apps/protection.go:130-142`), which asks a fresh workspace `can_manage` there. `TestActionCapabilities_SuspendPreconditionMatchesGuard` (`apps/actioncaps_test.go:47-90`) pins both halves: the projection says allowed and the verb answers `ErrForbidden` to a non-Admin.
  - **Datastores.** `postgres.ActionCapabilities` and `keyvalue.ActionCapabilities` (`postgres/actioncaps.go:33-63`, `keyvalue/actioncaps.go:32-61`) have the same gap through `core.EnvironmentProtectionPrecondition`. Their verbs refuse through `core.RequireEnvironmentAdmin` (`postgres/protection.go:28-32`, `keyvalue/protection.go:55-59`; `postgres/lifecycle.go:55-58`, `keyvalue/service.go:869-872`).
  - **Dashboard.** `gateAction` (`dashboard/src/features/capabilities/lib/resource-actions.ts`) presents `protected_confirmation_required` as ready. `service-row-actions.tsx`, `suspend-service-card.tsx` and `resume-service-button.tsx` gate on `serverActions`, so they enable both actions for a Developer. `database-row-actions.tsx`, `database-danger-actions.tsx` and `key-value-danger-actions.tsx` gate on the workspace `can_operate` only. The dashboard calls neither `databaseActions` nor `keyValueActions`, and a datastore view carries no environment. `key-value-row-actions.tsx` offers no Suspend or Resume.
  - **What w4/m550 covers:** only the bulk toolbar. `bulkLifecycleReason` (`dashboard/src/features/projects/lib/bulk-lifecycle.ts`) reads the workspace grant, with `can_manage` in a protected environment. It took that route because of this same mismatch (m550 README, "Deviation, gating source"). No per-resource surface was changed.
- **Render:** [render.com/docs/projects](https://render.com/docs/projects) reserves suspending and resuming services in a protected environment to Admins. Its dashboard disables a refused Suspend or Resume with _This action is limited to admins._ ([capture](../../../docs/render-artifacts/service-list-bulk-actions.md)). bex-api has enforced the rule since w8/187 ([ADR032](../../../docs/ADR032-environments.md)).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). The capability projections exist so that a client never offers a verb the server refuses ([ADR087](../../../docs/ADR087-mobile-role-views.md)). The mobile app reads the same three projections and hides a denied action, so it is fixed by t001 with no client change.
- **Expected outcome:** a Developer in a protected environment never clicks Suspend or Resume and reads a 403. The control says why before the click, as the bulk toolbar and Render already do.
- **Why now:** w4/m550 shipped the bulk path's correct gating on 2026-11-22, so the same page now gates one way in the toolbar and the other way in each row's menu. The fix is a decision projection plus copy, with no new verb.
- **Render parity:** included (t005). The change faces tenants on GraphQL and in the dashboard.
