# w4 · m578 — In a protected environment a Developer sees every per-resource Suspend and Resume disabled with Render's reason, never a 403 after the click

**Worker:** worker4 **Goal:** the per-resource lifecycle projections (`serverActions`, `databaseActions`, `keyValueActions`) answer suspend and resume by the same rule the verbs enforce: in a protected environment only a workspace Admin may run them, as on Render. Every per-resource Suspend and Resume in the dashboard (the service row menu, the service header and Settings card, the database header menu and Info actions, the Key Value Info actions) is then disabled with Render's _This action is limited to admins._ for a Developer there, and enabled for an Admin. The bulk toolbar already gates this way (w4/m550). **Status:** blocked (pin must carry `ec45a8ebec`; t001–t003 and t005–t007 done 2026-11-22; t004 passed on dev-4, and its production half and t008 follow the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The three lifecycle projections deny suspend and resume to a non-Admin on a protected member, by the verbs' own rule — **DONE** | 50m | — |
| [t002](done/t002.md) | The service row menu, header and Settings card read Render's admin-only reason — **DONE** | 30m | t001 |
| [t003](done/t003.md) | Database and Key Value Suspend and Resume gate on their projections — **DONE** | 1h | t001 |
| [t004](t004.md) | Live check on dev-4 as an Admin, a Developer and a Viewer at 1440 and 390 | 30m | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
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
- **Render:** [render.com/docs/projects](https://render.com/docs/projects) reserves suspending and resuming services in a protected environment to Admins. Its dashboard disables a refused Suspend or Resume with _This action is limited to admins._ ([capture](../../../../docs/render-artifacts/service-list-bulk-actions.md)). bex-api has enforced the rule since w8/187 ([ADR032](../../../../docs/ADR032-environments.md)).
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). The capability projections exist so that a client never offers a verb the server refuses ([ADR087](../../../../docs/ADR087-mobile-role-views.md)). The mobile app reads the same three projections and hides a denied action, so it is fixed by t001 with no client change.
- **Expected outcome:** a Developer in a protected environment never clicks Suspend or Resume and reads a 403. The control says why before the click, as the bulk toolbar and Render already do.
- **Why now:** w4/m550 shipped the bulk path's correct gating on 2026-11-22, so the same page now gates one way in the toolbar and the other way in each row's menu. The fix is a decision projection plus copy, with no new verb.
- **Render parity:** included (t005). The change faces tenants on GraphQL and in the dashboard.

## Progress (2026-11-22)

- **Shipped (t001–t003, t005–t007):** the three projections decide suspend and resume by the verbs' rule (`core.RestrictedDecision`), with the reason `protected_environment_admin_required`. The dashboard shows it as Render's _This action is limited to admins._ on the service row menu, the service header's Resume and Settings card, a database's header menu and Info actions, and a Key Value's Info actions. The datastore controls read `databaseActions` and `keyValueActions` and re-read them before dispatch. ADR018 and ADR032 describe it.
- **Follow-ups filed (t005):** [w4/724](../../724.md) Delete, [w4/725](../../725.md) maintenance mode on, [w4/726](../../726.md) Shell: each reads as allowed to a Developer in a protected environment and the server refuses it.

## Live check (2026-11-22)

**dev-4 (t004).** bex-api built from this change on `33ab37bfb0`, and the dashboard from this tree on :50040. Authz was enforced by an isolated OpenFGA seeded with `deploy/gitops/authz/model.json` (`DEV_OPENFGA_URL`). Before the UI pass, a Developer's REST suspend of the protected service, Postgres and Key Value, and a Viewer's resume, all answered 403. Three throwaway Kratos identities were used: an Admin on a Scale workspace, and a Developer and a Viewer it invited. The fixtures were one project with `staging` and a protected `production`. Each environment held a running and a suspended web service, a Postgres and a Key Value. Production's datastores were on the smallest paid plans, because a workspace holds one free Postgres and one free Key Value. Headless Chromium drove each role at 1440 and 390, and both widths gave the same results:

| Surface | Admin, protected | Developer, protected | Viewer, both | Admin and Developer, staging |
| --- | --- | --- | --- | --- |
| Service row menu Suspend / Resume | enabled; suspended and resumed | disabled, _This action is limited to admins._, nothing sent; Restart enabled | disabled, "Your role can’t perform this action. Ask a workspace admin.", nothing sent | enabled; both cycled |
| Service header Resume, Settings card | enabled; cycled | disabled, admin-only copy, nothing sent | disabled, role copy | enabled; cycled |
| Database header menu and Info actions | enabled; cycled | disabled, admin-only copy, nothing sent; Restart enabled and ran | disabled, role copy | enabled; cycled |
| Key Value Info actions | enabled; cycled, with Render's `sudo suspend key value <name>` | disabled, admin-only copy, nothing sent | disabled, role copy | enabled; cycled |

- **GraphQL, protected.** The Admin read suspend `allowed` with `protected_confirmation_required` on running resources and with `suspended` on the suspended one. Resume was `allowed` with `not_suspended` on running resources and empty on the suspended one. The Developer read `denied`/`protected_environment_admin_required` for suspend and resume on all four resources, and restart `allowed`. The Viewer read `denied`/`insufficient_permission` throughout. In staging the Developer read `allowed`.
- **Fresh check before dispatch.** The Developer opened Suspend on a staging Postgres, Key Value and service, and the Admin then protected staging over REST. Each confirm showed _This action is limited to admins._ and sent nothing.
- **After a state change.** The control that replaces the one used became usable 0.2–1.7 s after the reply on datastores. On services it took 2.6–3.7 s, the lifecycle hook's 2.5 s convergence poll. One 20 s outlier did not repeat; the host was under load.
- **Fixed in this change:** after a successful Suspend from the service row menu, the still-open dialog read "Unavailable while the service is suspended. Resume it first." for 2–3 s (4 of 4 runs). The re-asked projection answered for the new state while the hook waited for convergence. A dispatched action's dialog no longer shows a gate reason. A regression test covers it.
- **Not this change, noted:** a free web service that has idled to sleep reads "Suspended" in the project list while REST says `not_suspended`. A Viewer's database Restart tooltip says "this service".
- **Cleanup:** the fixtures, project, workspaces and identities were deleted, the OpenFGA container was removed, and bex-api was restarted with its defaults.

**Production.** This waits for a pin that carries the change, `ec45a8ebec`. As the QA user (an Admin) in `bex-canary`, suspend and resume one service, one Postgres and one Key Value in a protected environment of a disposable project, at 1440 and 390, then close with t008. Production has one QA identity, so the Developer and Viewer lines ran on dev-4 only.
