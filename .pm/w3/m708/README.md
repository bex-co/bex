# w3 · m708 — Changing a workspace's plan is Render's page at `/billing/update-plan`, with its review and a typed downgrade confirmation

**Worker:** worker3 **Goal:** every upgrade link and Change plan button opens Render's full-page plan chooser at `/billing/update-plan`; a downgrade goes through Render's preview (`/billing/update-plan-preview`) and confirmation (`/billing/update-plan-confirm`, typed `sudo downgrade workspace <name>`), and an unmet precondition shows Render's titled blocker with its fix link; the change-plan dialog on Workspace Settings is retired. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's three plan pages, and settle whether Choose custom domains applies to bex's metered domains | 40m | — |
| [t002](t002.md) | `/billing/update-plan` is a page: current plan, Select a plan cards with Enterprise as Get in touch, Payment Method, billing notes, Cancel | 1h45m | t001 |
| [t003](t003.md) | An unmet downgrade precondition shows Render's titled blocker with its fix link | 1h | t002 |
| [t004](t004.md) | `/billing/update-plan-preview` compares the plans and reviews the new limits | 1h15m | t002 |
| [t005](t005.md) | `/billing/update-plan-confirm` lists what changes and asks for `sudo downgrade workspace <name>`; the dialog is retired | 1h25m | t003, t004 |
| [t006](t006.md) | Walk an upgrade and a downgrade in a real browser at 1440 and 390, locally and on production | 45m | t005 |
| [t007](t007.md) | Render parity | 30m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 8h30m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-26 without sign-in: `BillingUpdatePlan-BMYzZkCW.js`, `BillingUpdatePlanPreview-b8G1f0fw.js`, `BillingUpdatePlanConfirmation-DZzLF7e6.js`, `TeamPlanPicker-BE6cuQLr.js`; routes `/billing/update-plan`, `/billing/update-plan-preview`, `/billing/update-plan-confirm`; builders `/billing/update-plan?plan=hobby|professional|organization` and `?plan=professional&next=${window.location.pathname}`.
- **Plan page:** heading Change plan; "Your workspace is on the … plan."; **Select a plan** cards (`TeamPlanPicker`, Enterprise as **Get in touch**, not selectable); "Learn more about or compare plans by visiting our" pricing page; **Payment Method** ("This workspace plan requires a card for paid services and compute.", "Add a valid card to upgrade to a paid plan."); How billing works; a summary error "There are some errors in your selection. Please review and fix them to continue." over **Plan selection errors**; Select plan.
- **Blockers** (title, fix link): "Too many users in team." / Update team members; "Too many services in workspace." / Update services; "Too many workspaces." and "Too many Hobby workspaces." / Update workspaces; "Too many custom domains"; "Cannot downgrade while using unavailable roles" ("To downgrade your workspace, first change affected team members and pending invites to Developer or Admin.") / Update team members. HIPAA's blocker is out of scope (no HIPAA in bex).
- **Preview:** the plan comparison card, **Review new limits**, and **Choose custom domains** when the target keeps fewer domains than the workspace has.
- **Confirmation:** Review changes; **What's changing?**; New plan summary; "Review your new limits."; "Changes will be applied to the next billing period." where they apply; a typed confirmation `sudo downgrade workspace <name>` (the name trimmed, whitespace runs replaced) before **Confirm downgrade**; "Error updating plan." on failure; analytics "Team Plan Changed".
- **Not in scope here:** Render's pricing-update opt-in banner on the same page ("We’re updating our pricing on …"), which belongs to the 2027 pricing work already tracked by `dashboard-operations.md`'s flag rows.

## bex today

Verified 2026-11-26 on main `76dfb1e0e2`.

- `routes/billing_.$first.$.tsx` sends `/billing/update-plan` to `/workspace/settings?plan=change`, which opens `features/workspaces/components/change-plan-dialog.tsx` (`plan-picker.tsx`, `hooks/use-change-workspace-plan.ts`); every other `/billing/*` goes to `/billing`.
- The dialog lists downgrade preconditions from bex-api (`secondary_workspaces`, `member_roles`, `invite_roles`, `member_limit`, `inbound_ip_rules`). `lego/backend/internal/workspaces/service.go` `ChangePlan` enforces `requirePlanEntitlement` (Enterprise needs `EnterpriseEntitlement`), `RequirePlanBilling` for paid plans and `guardPlanCapacity`; none of that changes.
- `docs/render-artifacts/workspace-plan-change.md` (2026-07-12) lists the dedicated page, the disabled same-plan submit and Enterprise's Get in touch as "Deliberate simplification", because "bex has no billing app to house a full page". bex has had a Billing page since w5/m70. ADR018's "Workspace lifecycle" row (`docs/ADR018/identity-workspaces-account.md`) records no sign-off.
- Custom domains: ADR030 meters domains above a plan's included count; Render's new plans show `limitType SOFT` with a per-domain price, its legacy Hobby `HARD` 2 (`billing-confirm-plan-utils`).

## Definition of done

1. `/billing/update-plan` (and `/w/<tea-id>/billing/update-plan`, and every in-app upgrade link) opens the plan page with Render's sections; `?plan=` preselects a card and `?next=` is where Cancel and success return; Enterprise reads Get in touch and cannot be selected.
2. Choosing a paid plan without a payment method shows Render's message and keeps Select plan disabled; bex-api's `RequirePlanBilling` still refuses it server-side.
3. Each unmet downgrade precondition shows Render's titled blocker and its fix link; Select plan stays disabled until none remain.
4. A downgrade goes to the preview (comparison, new limits, Choose custom domains if t001 finds it applies) and then the confirmation, which requires the exact `sudo downgrade workspace <name>` before Confirm downgrade; an upgrade confirms without the phrase, as Render's does.
5. Success shows Render's result and returns to `next`; a failure shows "Error updating plan." with the page kept.
6. The dialog and `?plan=change` are gone, with old links landing on the page; `workspace-plan-change.md` and ADR018's row say what bex does, the "Deliberate simplification" rows removed.
7. Walked locally and, once a production pin carries the milestone, on production as the QA user in a disposable workspace (created and deleted for the walk), upgrade and downgrade, at 1440 and 390.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), item 5, verified against Render's chunks the same day.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Every upgrade link in bex already points at Render's `/billing/update-plan`; only the page behind it differs, and the July simplification has no ADR018 sign-off.
- **Expected outcome:** changing plans looks and guards like Render: a full page, blockers that say how to fix them, and a typed confirmation before a downgrade takes features away.
- **Why now:** the July premise (no billing page to house it) no longer holds. w6/m627 (billing email, parked) also touches the plan flow's server side, but this milestone changes only the dashboard and keeps `ChangePlan` as it is.
- **Render parity task:** included (t007): the milestone changes a dashboard flow and its URLs.
