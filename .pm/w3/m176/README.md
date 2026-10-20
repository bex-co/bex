# w3 · m176 — New workspaces get actionable empty states and a getting-started checklist (ADR075 D1+D2)

**Worker:** worker3 **Goal:** a new account that clears the email and card gates lands on an Overview that tells it what to do next and lets it do it — every new-user-path empty state has a working primary action, and a dismissible checklist tracks verify → card → first service → GitHub → teammate **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Give `EmptyState` an action slot and move the three new-user empties onto it | 30m | — |
| t002 | Primary actions on the Overview, Blueprints and Env groups empty states | 45m | t001 |
| t003 | Getting-started checklist on Overview for zero-resource workspaces | 1h30m | t001 |
| t004 | Pending state that matches the ready layout at desktop and narrow mobile | 40m | t003 |
| t005 | Live first-run walk on dev-3 with a fresh account | 45m | t002, t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 40m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- On dev-3, a freshly registered account's Overview shows the checklist; each item's state is correct before and after its action (verify email, add card, create a service, connect GitHub, invite a teammate), and the card disappears once dismissed — persisted across reload, per workspace, with no backend state.
- The Overview, Blueprints and Env groups empty states each show a primary action that opens the right create flow (en and zh).
- Pending and ready screenshots match at 1440 and 390 widths (AGENTS.md skeleton rule).
- Component tests cover every checklist item state, dismissal persistence and the empty-state actions; `yarn typecheck && yarn lint && yarn test` pass.

## Evidence (2026-10-19)

- [ADR075](../../../docs/ADR075-user-onboarding.md):45-46 decides D1 (action slot + CTAs on Overview, Blueprints, Env groups) and D2 (dismissible checklist, client-side dismissal); `:78` says D1–D3 "remain pending `/pm` materialization"; `:69` non-goals: no tour framework, no per-user onboarding state in the control-plane DB.
- `EmptyState` takes only `title`, `description`, `iconName` (`dashboard/src/common/components/empty-state.tsx:24-28`) and is used only by deploy and log views; the three target empties are inline markup with no action: `dashboard/src/routes/index.tsx:256-260`, `routes/blueprints.tsx:114-118`, `routes/env-groups.tsx:160-165`.
- No checklist exists in `dashboard/src`. D3's sessionStorage `next` relay already ships (`features/auth/lib/auth-next.ts`); D4 is `w5/161`; D7/D8 shipped in `w6/m42`.
- "Deploy a sample" has no dashboard path: ADR075's "w9/m43–m44 funnel" is the CLI-verified `examples/` lifecycle, and `parseNewServiceSearch` (`features/services/lib/create-context.ts:103`) accepts only type/project/environment — t003 decides.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-10-19, item 5 (approved by the user the same day); [ADR075](../../../docs/ADR075-user-onboarding.md) D1+D2.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) — a Render alternative a new user can adopt without help; ADR075's "working path to a first deploy without leaving the dashboard".
- **Expected outcome:** a new signup moves from the payment wall to a first deploy through the dashboard's own guidance; ADR075's funnel counters (workspaces at zero resources) have a lever to move.
- **Why now:** the open-signup gates (`w6/m42`) and the payment wall shipped, so every new account now clears verify + card and lands on dead-text empties; this is the unscheduled remaining half of ADR075, and `w5/161` already covers the auth-error dead end.
- **Render parity included:** dashboard UI change — compare against Render's new-workspace dashboard empty states and onboarding prompts.
