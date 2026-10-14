# Plan-card radios ignore arrow keys and omit group names

Source snapshot of inbox **w4/300**, shipped as `a69dd557d`, then promoted to **w4/m207** after a450 independently verified the three other card-radio producers. Its estimate and live limits describe the original a449 scope; the milestone README defines the expanded work.

Why: keyboard users encounter an unnamed radio group whose arrow keys do not select a plan, and must tab through each tier instead.

**Severity:** minor. **Estimate:** 60m. Source: continuous functional `$qa-find-bugs w4`, a449, 2026-10-14 UTC. Goal: usable hosting configuration under ADR008 and the controls' advertised radio semantics. Filing only; no product implementation.

## Reproduction and controls

Fresh-load `/keyvalue/new`, wait for its three actual catalog tiers, and leave Name empty. Focus the checked Free radio. Press Right Arrow, then independently refocus Free and press Down Arrow. Focus and selection stay on Free. Refocus Free and press Tab: focus enters Starter while Free remains checked. Each card's actual DOM node is a BUTTON with `role="radio"` and `tabIndex: 0`.

Repeat from a fresh `/services/new?type=web_service` page with its seven tiers. Repeat both fresh journeys at 1440×1000 and 390×844. All four receipts show the same behavior. The accessibility tree gives the group no name; its `aria-label` and `aria-labelledby` are both absent.

**Positive control:** focus Starter and press Space. Starter becomes the single checked radio on both surfaces. Focus Free and press Space to restore it. The forms remain unsubmitted, their submit buttons remain disabled, and the request observer records **zero mutations** during each probe. This isolates keyboard behavior from missing catalog data, disabled tiers or failed writes. No resource was created and no plan was purchased or saved.

[300-plan-radio-keyboard.json](../evidence/300-plan-radio-keyboard.json) contains exact runnable probes, every before/after accessibility tree, native active-element/checked/tab-index receipts, zero-mutation observations, inventory and session cleanup. Viewed cropped plan-only screenshots: `.playwright-mcp/qa-a449-{keyvalue,services}-arrow-{desktop,mobile}.png`. They show the focused Free card after Right Arrow without capturing connected repositories or unrelated page data.

The target is the [W3C non-toolbar radio-group interaction](https://www.w3.org/WAI/ARIA/apg/patterns/radio/): arrows move focus and selection between options; Tab enters/exits the group at one option; Space selects the focused option; the group has an accessible label. Left/Up, boundary wrapping and reverse entry were not exercised in this hunt and belong in implementation verification. No signed-in Render keyboard comparison is claimed.

## Root and bounded target

`dashboard/src/common/components/plan-card-grid.tsx:45–63` renders a radiogroup and plain button radios with `aria-checked` and `onClick`. There is no arrow-key handler or roving tab index. The browser supplies each button's measured default tab index of zero. Its optional `ariaLabel` prop at `:20–25` is passed to the group, but three callers omit it.

This component explicitly uses plain buttons rather than a RadioGroup primitive. The actual installed React DOM **19.2.3** was inspected: `react-dom-client.development.js:3274–3314` obtains an event listener from the corresponding element prop; `:19328–19337` creates generic keyboard events. It supplies no radio-group interaction merely because a button has that role. Native Space triggers the existing click path, explaining the successful control without supplying arrow behavior.

**Target:** make this existing controlled grid a complete radio control, preserving its catalog order, values, selected highlighting, prices and layout. Add one Tab entry point, focus/selection movement for the four arrow keys with boundary wrapping, and native Space selection. Changing the checked tier still calls the existing local `onChange`; it must not submit a form or persist a plan. When none of the current tiers is selected, use the first available option as the entry point without silently selecting a different tier on render. Empty/disabled groups must remain inactive.

Require an accessible group name and supply the existing translated visible plan/Instance Type label at the three unnamed callers. Preserve the already-correct Postgres label. Keep the component small; no new UI library or unrelated radio-control rewrite is needed.

## Four callers, with different label dispositions

| Production caller | Keyboard scope | Group-name scope | Live coverage |
| --- | --- | --- | --- |
| `routes/services.new.tsx:469–473` | Shared grid correction | Add the existing translated plan label | Web create at both widths; other service types unprobed |
| `routes/keyvalue.new.tsx:164–168` | Shared grid correction | Add the existing translated plan label | Key Value create at both widths |
| `features/keyvalue/components/key-value-plan-section.tsx:66–71` | Shared grid correction | Add the existing translated Instance Type title | Source traced; prior a448 DOM has an unnamed group; keyboard unprobed |
| `features/databases/components/database-plan-section.tsx:111–117` | Shared grid correction | Preserve its existing `ariaLabel={t("databases.planTitle")}` | Source-only control; keyboard unprobed |

An exhaustive production TSX search finds **four** `PlanCardGrid` calls. Service creation uses this grid for plan-bearing web/cron/worker/private forms; static creation has no instance-plan grid. Postgres creation uses a separate combobox. The Service Plan page's picker is also a different producer and is excluded; a named group there is not proof that this grid handles arrows correctly.

Canonical surfaces are the two create URLs above and the plan sections inside `/keyvalue/$keyValueId` and `/databases/$databaseId`. `render-alias.ts:34–42` maps `/web/new`, `/worker/new`, `/pserv/new` and `/cron/new` to typed Service Create, `/r/new` to Key Value Create, and `/d/new` to the separate Postgres dialog. `/r/<id>` and `/d/<id>` redirect to the two detail routes; `/new/redis` is another Key Value create landing. These add no fifth grid implementation. Do not change redirects, source/type eligibility, capabilities or plan-save confirmations.

Implementation verification: assert real focus and checked-state transitions, one Tab stop, native Space/click controls and group names. Replay the two observed create journeys at both widths, then verify Left/Up/wrap/reverse entry, both detail consumers, service-type eligibility, empty/unknown selection, changed catalogs and disabled groups. Preserve the named Postgres control and existing prices/confirmation behavior. Run affected dashboard checks and Simplify. API/runtime behavior is unchanged; no backend suite is needed for this UI-only change.

## Dedupe, original gap, and limits

Checked open/blocked/done notes, DO_NOT_DO, all 64 current unarchived milestone READMEs, exact PlanCardGrid/radiogroup/roving-tab terms, the recent 40 product commits and target history. Existing w4/156 added datastore prices; w4/179 fixes the New Postgres combobox layout; m196 fixes service-plan loading geometry; m183 fixes navigation to a named Service Plan group. None supplies this shared grid's keyboard behavior or the three missing names.

`49c75378b` introduced the shared grid with button radio roles and a click callback, already without arrow handling or roving focus. Later price, capability and skeleton changes leave those omissions in place. This is an original uncovered control contract, not a demonstrated regression of a completed keyboard fix. The existing price guarantee remains visible in all captured tiers; no price/billing issue or full prior milestone re-certification is claimed.

Only create-form drafts were changed and restored. Left/Up/wrapping, disabled/unknown/catalog-transition cases, cron/private/worker creation, both detail-page keyboard journeys, screen-reader speech, purchases and persistence APIs remain unverified live.

All seven baseline inventory ID sets matched at **10:34:42.697Z**. The own session was revoked: owners **401**, cookies **0**, one page at `about:blank`, **10:36:52.114Z**; exact session handles were removed. No fixture or foreign resource changed.
