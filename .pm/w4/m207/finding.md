# Four independent card-radio producers omit keyboard navigation

Why: the advertised control behaves as an ordinary button set across setup and configuration, so fixing one producer leaves the remaining journeys broken.

Severity minor. Functional QA a449/a450, 2026-10-14 UTC. [source-300.md](source-300.md) preserves the original PlanCardGrid record; this record adds independently traced and replayed siblings. Filing only.

## Replays and durable evidence

Every row was driven from fresh pages at 1440×1000 and 390×844 after actual options appeared. Right/Down leave focus and selection on the first checked option. Tab focuses the second option while the first stays checked. Every option has native `tabIndex: 0`. Space selects the second option; Space on the original restores it. This is active-element/checked-state and accessibility-tree evidence, not a text heuristic.

| Producer / live surface | Options | Initial → Space control | Group name |
| --- | --: | --- | --- |
| PlanCardGrid / `/keyvalue/new` | 3 | Free → Starter → Free | Missing |
| PlanCardGrid / `/services/new?type=web_service` | 7 | Free → Starter → Free | Missing |
| ServiceTypePicker / Service Create | 5 | Web → Private → Web | Missing |
| InstanceTypePicker / owned service Plan page | 7 | Free → Starter → Free | Pick an Instance Type, correct |
| PlanPicker / `/new/workspace` | 4 | Hobby → Pro → Hobby | Plan, correct |

Correct group names do not fix the arrow failure. On the owned service, Space enables Save Changes; restoring Free disables it. Both API checks still say `plan: free`. No save/confirmation was invoked.

[m207-card-radio-family.json](../evidence/m207-card-radio-family.json) preserves six new complete probe/receipt pairs, fixture create/delete, persisted-plan reads and cleanup. Four a449 pairs remain in [300-plan-radio-keyboard.json](../evidence/300-plan-radio-keyboard.json): ten receipts across five surfaces. The observer recorded zero GraphQL mutations during each key/Space sequence. Forms were not submitted; no workspace, paid resource or billing attempt was created.

All ten cropped radio-only screenshots were viewed: `.playwright-mcp/qa-a449-{keyvalue,services}-arrow-{desktop,mobile}.png` and `qa-a450-{type,workspace,instance}-arrow-{desktop,mobile}.png`. The JSON carries durable keyboard proof; the ignored screenshots expose only choice cards.

## Four causes and eight boundaries

An exhaustive production TSX search finds four explicit button-card radio producers. Paths below are under `dashboard/src/`. Each independently has plain `button role="radio"`, `aria-checked`, a click callback, and no arrow handler or roving Tab policy:

- `common/components/plan-card-grid.tsx:45–63`: four boundaries — Service Create, Key Value Create, Key Value detail, Postgres detail. Three omit `ariaLabel`; Postgres detail supplies the correct title. Original trace and React DOM 19.2.3 listener path remain in source-300.
- `features/services/components/service-type-picker.tsx:52–75`: one boundary, `routes/services.new.tsx:134`. Five type cards also lack a group name. Parent selection changes local form type; its eligibility/defaults must stay intact.
- `features/services/components/instance-type-picker.tsx:100–125,181–197`: one boundary, `routes/services.$serviceId.plan.tsx:46`. Two internal InstanceTypeCard references cover Free/Paid groups. Name is correct. `setSelected` changes the draft; separate save/confirmation invokes the plan mutation.
- `features/workspaces/components/plan-picker.tsx:31–49`: two boundaries — `routes/new.workspace.tsx:189` and `features/workspaces/components/change-plan-dialog.tsx:77`. Name and `aria-describedby={usageNoteId}` are correct. Create's onSelect changes selectedPlan/confirmation/error state; Change Plan passes local setPlan. Neither callback submits.

There are **eight production boundary callers**, **four missing names**, and **four correct existing names**. Workspace Change Plan and Postgres/Key Value detail keyboard paths were source traced, not replayed live. Other native inputs, Ory controls, comboboxes and loading placeholders are outside this closed family. Canonical entrypoints are `/services/new`, `/keyvalue/new`, `/services/$serviceId/plan`, `/databases/$databaseId`, `/keyvalue/$keyValueId`, `/new/workspace`, and the existing Workspace Settings change-plan dialog. The original source enumerates Render create/datastore aliases; service `/web`, `/worker`, `/pserv` and `/cron` aliases preserve the `/plan` suffix and redirect to the canonical picker. They add no producer. Workspace Create and Change Plan have distinct parents despite sharing PlanPicker.

## Target and limits

Use one controlled interaction across all four producers, preferably native radios with preserved card presentation; use a small common primitive if needed. Preserve group-local options, controlled checked state, original draft callbacks, Space/click, existing names/descriptions and disabled states. No initial render/focus may silently select a fallback tier. Supply names where missing.

No new UI dependency, price policy, plan eligibility or REST/GraphQL/MCP change is needed. Keep Free/Paid service groups, paid-only worker/private eligibility, workspace capability/usage text and inert unresolved controls. Do not broaden this into a generic accessibility audit.

Left/Up/wrap/reverse entry, empty/unknown values, catalog changes, disabled groups, non-Web variants, three source-only consumers, screen-reader speech and purchases remain unverified live. t002/t005 own those implementation checks; they are not completed acceptance here.

## Dedupe and earlier guarantees

Checked open/blocked/done notes and milestone titles, DO_NOT_DO, exact producer/radio/keyboard terms, recent product history and all four target histories. No completed arrow fix or matching open item found. w4/300 is promoted, not duplicated. Each added producer has its own native replay and source trace.

Earlier guarantees differ: w4/156's visible tier prices remain in the create cards; backend fee/API parity is not re-certified. w5/m7's preselection and Save disable/draft-enable/restore states passed; mutation, resize, untiered and full-catalog guarantees were not replayed. w9/m88's Hobby/card text/fees are visible; usage-description wiring was traced. Payment readiness, create/failure, switcher, skeleton and suites were not re-certified. m196's loading geometry is outside this ready-control correction and was not re-audited. Keep those archived; no keyboard-fix regression is claimed.

Only owned Free web `srv-db7lsfmt41sc7388o87g / qa-20261014-a450-radio` was created. Native deletion succeeded **10:53:23.472Z**, API/public returned **404**, and seven inventory ID sets matched **10:53:34.648Z**. Session revocation/clear gave owners **401**, cookies **0**, one page at `about:blank`, **10:56:44.058Z**; exact handles removed. a449's cleanup remains in its original receipt. No foreign resource changed.
