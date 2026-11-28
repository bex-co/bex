# Render workspace plan change — Render's three plan pages and what bex does

Two captures:

- **Render's dashboard bundle, read 2026-11-27 for w3/m708.** The page `index-D5V4ZgbE.js` routes the plan pages to lazy chunks named in `AsyncComponents-XDHzvoGb.js`: `BillingUpdatePlan-BMYzZkCW.js`, `BillingUpdatePlanPreview-b8G1f0fw.js`, `BillingUpdatePlanConfirmation-DZzLF7e6.js`, `TeamPlanPicker-BE6cuQLr.js`, `billing-confirm-plan-utils-DuDK6s98.js`, `PlanComparisonCard-Bjmk3cyK.js` and `useShowPlanUpdatedToast-zaRAPDKH.js`. The capture is bundle-only: no signed-in Render session was available, so no page was opened and nothing was submitted.
- **A live Render session, 2026-07-12 (w6/m12)**, driven up to but not through the submit, below under [Live capture](#live-capture-2026-07-12).

## What bex does (w3/m708)

- **Pages.** `/billing/update-plan` is Render's Change plan page, and `/billing/update-plan-confirm` its confirmation (`dashboard/src/routes/billing_.update-plan*.tsx`). `/w/<tea-id>/billing/update-plan`, `-confirm` and `-preview` select the workspace (`routes/w.$.tsx`) and open the same page with the query kept. They return to Billing when the URL names no `next`, as Render's `/w/<id>/billing/...` forms do. Every upgrade link and Change plan button in the dashboard opens the plan page: Billing's Plan card and Workspace Settings' Plan row (both return to Billing), and the plan gates with Render's `?plan=professional|organization&next=<page>` (environments, networking, inbound IP rules, webhooks, 2FA and Google login, the seat wall, agent sandbox capacity, a create a plan cap refused). The change-plan dialog on Workspace Settings is gone. `/workspace/settings?plan=change` lands on the plan page.
- **Choosing.** `?plan=` takes Render's aliases. `?next=` is kept only when it is a path on the dashboard. Enterprise is **Get in touch** (`mailto:hello@bex.co`, the address bex.co/pricing's "contact us" opens) and cannot be selected. The current plan's card says Current plan and cannot be chosen again.
- **Payment Method.** The paid-plan notice, "How billing works" (bex's billing docs), and Workspace billing with the card on file and its hosted Add / Update actions, which come back to the same URL. While the platform requires a payment method and none is on file, Select plan stays disabled for a paid plan. bex-api's `RequirePlanBilling` still refuses it, and its `PAYMENT_REQUIRED` reads "Add a valid card to upgrade to a paid plan."
- **Blockers.** GraphQL `workspacePlanChangeBlockers(id, plan)` (admin only, GraphQL-only like `changeWorkspacePlan`) answers with `ChangePlan`'s own refusals for the plan, all at once. Each refusal is read with the same helpers as `guardPlanCapacity` and the org downgrade preconditions: the people beyond the seats, the services beyond the cap, the caller's Hobby workspaces at the cap, roles the plan does not offer, an org's secondary workspaces and inbound IP rules. The page draws each with Render's title and fix link and keeps Select plan disabled until none is left. A fix link opens in a new tab, as Render's does. `lego/backend/internal/workspaces/plan_blockers_test.go` and the org downgrade E2E tests pin the blockers to the refusals.
- **Upgrade and downgrade.** An upgrade is applied from the plan page, as Render's is. A downgrade goes to the confirmation. The preview is never opened, because bex meters custom domains instead of capping them hard (below); a link to it goes on to the confirmation with the same plan.
- **Confirmation.** Change Plan for `<name>`, New plan summary (the current plan beside the new one with the highlights that differ, from bex's own plan facts, over the card on file), and for a downgrade Confirm downgrade with the typed `sudo downgrade workspace <name>` (Render's whitespace rule) before Confirm. An org primary going below Scale lists "The organization will be removed." first. A failure keeps the page with "An error occurred." over the reason, or "Error updating plan."
- **Success.** Render's toast ("Your workspace is now on the Pro plan.", "Welcome to the Scale plan."), then `next`, else what the opening link handed over, else the dashboard's home.
- **Unchanged.** `changeWorkspacePlan` and `ChangePlan` keep their contract and checks; an org primary's `ORGANIZATION_DOWNGRADE_BLOCKED` adds the seat limit to its `member_limit` precondition. REST and MCP still have no plan mutation, as Render's do not.

## Render's plan pages (bundle, 2026-11-27)

### Routes and URL grammar

- Routes: `/billing/update-plan`, `/billing/update-plan-confirm`, `/billing/update-plan-preview` and the same three under `/w/:ownerId/billing/`. Builders: `/billing/update-plan?plan=hobby|professional|organization`, and the plan gates' `/billing/update-plan?plan=professional&next=${window.location.pathname}`.
- `?plan=` (`billing-confirm-plan-utils` `y`): `hobby` is Hobby; `team`, `pro` and `professional` are Pro; `org`, `scale` and `organization` are Scale. Anything else selects nothing. Enterprise has no alias.
- `?next=` (`x`): kept when Render's path check reads it, with the page's own hash appended.
- Where Cancel, Back and success go: `next`, else the `afterPlanUpdate` path the navigation state carries, else `/w/<ownerId>/billing` for the `/w/` form and `/` for the bare one.

### Change plan (`BillingUpdatePlan`)

- Title "Change plan for `<workspace>`", a ghost Back above it, and a sticky footer with **Select plan** and **Cancel**.
- **Select a plan**: "Learn more about or compare plans by visiting our pricing page." over `TeamPlanPicker`. Each card has its title, a Current plan or New plan badge, description, price and features. The action is Select plan, or Plan selected on the chosen card. The current plan's card is read-only. Enterprise is **Get in touch** (`/contact?utm_int=product_upgrade`) and is never selectable.
- **Payment Method**: while a plan other than Hobby is selected, "This workspace plan requires a card for paid services and compute." with **How billing works**. Then **Workspace billing**: "Billing is unique to each workspace. Your card will be charged for fees associated with paid services or compute in this workspace." beside the card on file and its Add / Update card modal.
- Select plan is disabled while the selection breaks a plan rule, while a paid plan is selected with no card on file, or when the selection is the current plan. Render then scrolls to the bottom of the page.
- Submit: a Hobby target over a hard custom-domain limit goes to the preview; a downgrade goes to the confirmation; an upgrade calls `updateTeamProfile` on this page. A refusal shows "An error occurred. `<reason>`": "Add a valid card to upgrade to a paid plan." for `payment-required`, else "Server error during submission. Please try again".
- Under the sections: one warning per broken rule, then "There are some errors in your selection. Please review and fix them to continue."

### Plan rules (Render's blockers)

| Rule | Title | Message | Fix link |
| --- | --- | --- | --- |
| Hobby workspaces ≥ 5 | Too many Hobby workspaces. | You cannot have more than 5 Hobby workspaces. Please upgrade or delete another Hobby workspace. | Workspace limits (docs) |
| Services over the plan's cap | Too many services in workspace. | `<Plan>` workspaces can only have up to `<n>` services. | Update services (`/`) |
| An org with more than one workspace, target below Scale | Too many workspaces. | `<Plan>` doesn’t support more than one workspace. | Update workspaces (the org's workspaces) |
| Seats used over the plan's seats | Too many users in team. | `<Plan>` workspaces can only have 1 user. / up to `<n>` users. | Update team members |
| Billing, Viewer or Contributor members or invites, Scale to below | Cannot downgrade while using unavailable roles | `<Plan>` doesn’t support the `<roles>` role(s). To downgrade your workspace, first change affected team members and pending invites to Developer or Admin. | Update team members |
| A hard custom-domain limit exceeded (legacy Hobby) | Too many custom domains | You cannot have more than `<n>` custom domains across all services in a Hobby workspace. Please select the custom domains you want to keep in the next page. | Custom domain limits (docs) |
| Org SAML SSO, SCIM or domains | Cannot downgrade with SAML SSO & SCIM enabled | (lists what to remove) | Organization settings |
| HIPAA enabled | Cannot downgrade HIPAA-enabled workspace | … | Contact support |
| No Git credential and private repositories | Need Git credentials to change plan. | … | Add Git credential |

Every fix link opens in a new tab. bex has no rule for SAML/SCIM (a non-goal), HIPAA or Git credentials, and its custom domains never hit a hard limit. bex also refuses a downgrade that leaves inbound IP rules (ADR115 D6); Render warns of them on its confirmation ("IP allow lists must be removed before downgrading." / "Remove all IP allow list configurations from your workspace, environments, and services before changing your plan."). bex draws that one as a blocker in the same layout, with Render's words for it and a link to Workspace Settings' Inbound IP rules.

### Preview (`BillingUpdatePlanPreview`) and Choose custom domains

- Without `?plan=` it goes to `/billing/update-plan`. It draws only when the target plan caps custom domains (`customDomains.isUnlimited === false`), as "Change Plan for `<workspace>`": **Review impacted services** ("Changes will be applied after submitting", the plan comparison card), **Review new limits** ("With the `<Plan>` plan, `<workspace>` has limited access to some features.") and **Choose custom domains** ("Limit choose `<n>`/`<limit>`", a searchable table of toggles). Back returns to the plan page; Confirm opens the confirmation with `?plan=&customDomains=<ids>`.
- **When Render opens it**: only from a Hobby target whose custom-domain limit is `HARD` and exceeded, which is the legacy Hobby plan's limit of two (`C[TIER1].customDomains`). Render's new plans meter custom domains (`limitType SOFT`: 2, 15 and 25 included, then $0.25 a domain), so its own flow never opens the preview for a workspace on them.
- **bex**: its plans include 2, 15 and 25 custom domains and meter the ones above (ADR030 §16, `core.IncludedCustomDomains`). That is Render's soft limit, so the preview does not apply: bex's plan page never opens it. `/billing/update-plan-preview` goes on to the confirmation with the same plan, or to the plan page with no plan. Render would draw its preview for a URL typed by hand; that difference is w3/164.

### Confirmation (`BillingUpdatePlanConfirmation`)

- Without `?plan=` it goes to `/billing/update-plan`. Title "Change Plan for `<workspace>`" (capital P). Sticky footer: **Confirm** and **Back** (to `/billing/update-plan`, without the plan, with the navigation state).
- **New plan summary**: the plan comparison card (current plan and New Plan side by side, each with title, badge, description, price and HIGHLIGHTS, a trend arrow on the new side and crossed-out lines for what it lacks), then the card on file.
- **Confirm downgrade** (a downgrade only): "To initiate the changes to your plan, including:" over a list of what changes, then "Type `<phrase>` in the text box and click the confirm button." When nothing is listed: "To initiate the changes to your plan, type `<phrase>` in the text box and click the confirm button." The input is labelled "Sudo Command" and focused. The phrase is `sudo downgrade workspace ${name.trim().replace(/\s+/g, " ")}`, and what is typed is compared under the same rule. Confirm stays disabled until it matches. An upgrade reached here confirms without it.
- What Render lists: team members removed, projects removed, build pipeline minutes credited, bandwidth over the free allowance, autoscaling disabled, PITR retention reduced, preview environments disabled, registry credentials of others, 2FA and Google login enforcement disabled, webhooks deleted, dedicated static IPs to remove, metric streams removed, org SSO/SCIM removed, HIPAA, IP allow lists.
- A refusal: "An error occurred." over the reason, or "Error updating plan.". Success: analytics "Team Plan Changed", the plan-updated toast, then `next`, the state's `afterPlanUpdate`, or `/`.
- "Review changes", **What's changing?** / "Review your new limits." and "Changes will be applied to the next billing period." belong to the FY2027 pricing opt-in flow (a workspace offered the new plans that has not opted in). A workspace already on them, which is what every bex workspace is, never sees them. The opt-in banner and modal on the plan page are the 2027 pricing work tracked by `dashboard-operations.md`'s flag rows.

### Success toast (`useShowPlanUpdatedToast`)

Scale: "Welcome to the Scale plan." with its features. Pro: "Your workspace is now on the Pro plan." with the plan's description and "No seat fees". Hobby: "Your workspace is now on the new Hobby plan.". bex shows the first two as Render words them, with the plan's description. It drops "new" from Hobby's, since bex has no legacy Hobby beside it.

## Live capture, 2026-07-12

Captured live from `dashboard.render.com` in a real authenticated session on a workspace on **Professional (Legacy)** (w6/m12/t001). The flow was driven up to, but not through, the final submit. Clicking it on a real workspace would charge a real card. The radio selection, the page layout and the payment-method section were observed directly.

- The entry is Settings' General section: a **Plan** row (plan name, tagline, **Update Plan**) that routes to `/w/<tea-id>/billing/update-plan`, titled "Change plan for `<workspace name>`". The same page is reachable from **Billing** and from the pricing-transition banner.
- **Select a plan**: a radiogroup of plan cards in ladder order, Hobby (new), Pro, Scale and Enterprise (disabled, "Get in touch" to `render.com/contact`). Clicking anywhere on a card selects it. The selected card reads "Plan selected".
- **Payment Method**: the card warning, the card on file and a change-card control.
- Footer: the submit, disabled until a different plan is chosen, and **Cancel** back to `/billing`. Selecting a plan asks nothing; the confirmation comes after submit.

## Parity

| Render | bex | Verdict |
| --- | --- | --- |
| `/billing/update-plan` page with Select a plan, Payment Method and Select plan / Cancel | The same page and sections (w3/m708) | matched |
| `?plan=` aliases, `?next=`, return to `/w/<id>/billing` for the `/w/` form | The same | matched |
| Enterprise Get in touch, not selectable | Get in touch to `mailto:hello@bex.co` | matched (bex's contact address) |
| Select plan disabled for a paid plan with no card | The same when the platform requires a payment method; `RequirePlanBilling` still refuses | matched |
| Titled plan rules with fix links, computed before submit | bex-api's `workspacePlanChangeBlockers`, `ChangePlan`'s own refusals | matched for every rule bex enforces; bex adds its inbound IP rules refusal in the same layout |
| Upgrade applied from the plan page | The same | matched |
| Preview with Choose custom domains for a hard domain limit | Never reached: bex's domain limits are metered | n/a in Render's own flow; a hand-typed preview URL is w3/164 |
| Confirmation with New plan summary and `sudo downgrade workspace <name>` | The same, highlights from bex's own plan facts | matched |
| What a downgrade changes, listed on the confirmation | bex lists the organization's removal only | w3/165 |
| Plan-updated toast | The same words, without Hobby's "new" | matched |
| Changes "applied to the next billing period" (opt-in flow only) | Not shown; bex applies a plan at once | n/a: Render shows it only in its FY2027 opt-in flow |
