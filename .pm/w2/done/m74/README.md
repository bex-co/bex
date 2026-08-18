# w2 · m74 — Complete role-aware environment and datastore controls

**Worker:** worker2 **Goal:** a user never enters an environment or datastore edit flow that the authoritative relation will reject, and every denied control explains why without hiding operations the role is allowed to perform **Status:** done

## Problem

w9/m84 added the workspace capability projection, `useCapabilities`, localized denial reasons, and `PermissionTooltip`, but deliberately left adjacent controls for a follow-up. Contributors can still enter the shared service/environment-group editor and reach a server-side `can_create` rejection only after editing. PostgreSQL and Key Value controls also do not reflect their mixed authorization contract: delete requires `can_create`, while plan changes and suspend/resume/restart require `can_operate`.

w2/m73 has since consolidated service and environment-group variables plus secret files into one staged `EnvironmentEditor`. That makes the remaining environment gap smaller and higher leverage: one correctly placed gate can cover both resource families without touching reveal/export authorization.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Gate the shared environment editor on `can_create` — **DONE** | 45m | — |
| t002 | Gate PostgreSQL controls on their exact relations — **DONE** | 40m | t001 |
| t003 | Gate Key Value controls on their exact relations — **DONE** | 40m | t001 |
| t004 | Make datastore menu denial consistent and accessible — **DONE** | 30m | t002, t003 |
| t005 | Render parity — **DONE** | 25m | t001, t002, t003, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

- With definitive contributor capabilities (`canCreate: false`, `canOperate: true`), service and environment-group write entry points, draft mutations, import/generate, and save/deploy/rebuild controls are disabled with the localized `can_create` reason; no write callback or dialog opens.
- Environment reveal and export continue to follow `can_view_sensitive`, not `can_create`, and capability loading/unknown behavior remains fail-open exactly as w9/m84 defined.
- On PostgreSQL and Key Value detail/list surfaces, delete is disabled with the `can_create` reason, while plan changes and suspend/resume/restart remain available to contributors through `can_operate`. A role missing `can_operate` sees those controls disabled with the operation-specific reason.
- Admin/developer controls behave normally, busy/protected-resource states still compose with permission denial, and disabled controls expose their explanation on mouse and keyboard focus without opening confirmation dialogs or issuing mutations.
- Component tests cover the contributor, allowed, loading/unknown, and relation-split cases; relevant dashboard test, lint, and typecheck gates pass; ADR018 remains truthful.

## Source + Goal linkage

- **Source:** the non-build portion of `.pm/w9/048.md`, filed at w9/m84 closeout on 2026-08-17, reconciled against the current post-w2/m73 dashboard. The original note's datastore shorthand is corrected here from “all `can_create`” to the backend's actual split: delete = `can_create`; plan/lifecycle = `can_operate`.
- **Goal linkage:** `docs/ADR008-vision.md`'s honest human client, `docs/ADR018-render-parity.md`, and `docs/ADR024-members.md`'s contributor/developer boundary. The dashboard should predict the same permission outcome as REST/GraphQL/MCP before a user invests work in an edit.
- **Expected outcome:** permission failures become visible, localized preconditions instead of 403-after-edit surprises, while contributors retain every operational action the server already grants.
- **Why now:** w9/m84 shipped the reusable capability/tooltip infrastructure and w2/m73 merged service and group environment editing into one seam, so the remaining work is bounded to a small set of current controls.
- **Standing closing tasks:** t005 Render parity, t006 Simplify, t007 Test coverage, and t008 Closeout are included because this changes user-visible dashboard behavior.

## Guardrails

- The backend remains authoritative. Do not weaken or reclassify any authorization relation to make the UI easier to gate.
- Do not blanket-gate datastore controls on `can_create`: plan changes and suspend/resume/restart are `can_operate` operations today.
- Do not couple reveal/export to write permission; sensitive reads keep their independent `can_view_sensitive` boundary.
- Build Filters are explicitly excluded by user direction. Team-member hide-versus-disable behavior remains the Render-consistent w9/m84 decision.
- Project/Environment membership workflows, billing eligibility, resource protection, and new capability API fields are outside this milestone.

## Completion evidence (2026-08-18)

- The shared service/environment-group editor now disables every `can_create` write entry point and draft mutation after a definitive denial, while reveal/export stays independently governed by `can_view_sensitive` and unresolved capability reads remain permissive.
- PostgreSQL and Key Value plan/lifecycle controls use `can_operate`; datastore delete uses `can_create`. Detail and row-menu handlers guard the same relation as their presentation, and denied menu items remain focusable with localized explanations.
- Render's current role matrix agrees on contributor environment/delete restrictions. ADR018 records Bex's deliberate broader contributor `can_operate` boundary for datastore plan/lifecycle actions; REST/GraphQL/MCP shapes and backend authorization were unchanged.
- `/simplify` reused `PlanCardGrid` for PostgreSQL and hoisted the resource-table capability observer so datastore rows share one query subscription. More speculative permission-prop abstractions were left local to keep the governing relation visible at each control boundary.
- Validation: focused role/control tests **65/65**; full dashboard suite **332 files / 2,279 tests**; `yarn lint` (including typecheck), `yarn build`, and `git diff --check` passed.
