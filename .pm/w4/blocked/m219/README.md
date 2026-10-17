# w4 · m219 — Gate creation forms and group search until handlers are ready

**Worker:** worker4 **Goal:** four observed SSR roots retain configuration and search intent by accepting edits only when handlers are ready. **Status:** blocked — t001–t007, t009 done 2026-10-16; t008 (fixed-build live replay) needs the release pipeline

**Estimate:** 3h30 (9 tasks). Filing only; no product fix shipped by the hunt.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Gate Key Value creation inputs through hydration — **DONE** | 20m | — |
| t002 | Gate the shared service creation form through hydration — **DONE** | 25m | — |
| t003 | Gate Blueprint creation inputs through hydration — **DONE** | 25m | — |
| t009 | Gate Environment Groups search through startup — **DONE** | 15m | — |
| t004 | Audit native fields, aliases and existing hydration consumers — **DONE** | 30m | t001, t002, t003, t009 |
| t005 | Render parity across creation and group search — **DONE** | 20m | t004 |
| t006 | Simplify the changed code — **DONE** | 10m | t005 |
| t007 | Test SSR edit readiness and retained drafts — **DONE** | 45m | t005 |
| t008 | Replay live evidence and close out | 20m | t006, t007 |

## Definition of done

- Preserve [a805's passing webhook control](control-a805.md): on `/webhooks/new` at1440/390px no Name/URL input is exposed before startup, ready native typing persists, and event-catalog Retry retains the draft. This create page stays outside the four-root readiness fix.

- Fresh-load `/keyvalue/new` at 1440 and 390px while holding the real fetched startup script response as in [a797 evidence](../../evidence/m219-a797-input-startup.json). Name is visibly unavailable for editing until its handlers are ready. Release the response unchanged, type a valid name, and observe the exact text retained through startup queries settling. Current behavior accepts trusted typing before handlers exist, then clears it.
- Repeat at `/services/new?type=web_service`: Name follows the same readiness rule at both widths. In the desktop Port replay, the pre-ready field cannot accept a change; after readiness, enter valid `8197` and observe it remain `8197` through normal renders, rather than restoring `3000`.
- Repeat the four additional observed Name cases in [a800](finding-a800.md): private and cron at1440px, worker and static at390px. Pre-ready inputs cannot accept edits that will be discarded; after readiness exact typing persists. Preserve the observed Postgres deep-link control: no Name field exists before startup, and the ready dialog retains native input.
- Repeat at `/blueprints/new` at both widths for Blueprint Name. In the desktop Blueprint path replay, the pre-ready field cannot accept a change; after readiness, enter `examples/hello-go/render.yaml` and observe it retained, rather than restoring `render.yaml`. No deployment is required for this check.
- Repeat [a815](finding-a815.md) at `/env-groups` at1440/390px: **Search environment groups by name** stays visible but uneditable before handlers. After release, exact native typing is retained and filters to **No matching environment groups** with the existing Reset search controls. Preserve workspace-reset and ready matching semantics. The qualified desktop Project Unassigned comparison stays outside this local gate; its later ready typing remains retained.
- Ready forms retain their present labels, defaults, geometry, validation, source selection, plan/loading/error states and submission predicates. Entering text after handlers attach keeps that text, as the passing controls already demonstrate. Verify pending and ready geometry side by side at desktop and narrow mobile widths.
- The live replay submits no create/deploy action. Fixture inventory remains unchanged and this run's own session is revoked. Preserve the eight original a797 captures, four a800 sibling cases and three a815 search replays, and record fixed-build replays before moving this milestone to `done/`.

## Source + Goal linkage

- **Source:** ordinary functional `$qa-find-bugs w4`, pass a797, 2026-10-16 UTC; [researched finding](finding.md), [durable replay factories and complete DOM receipts](../../evidence/m219-a797-input-startup.json). Source HEAD `00f2afcfb44a3626ef4c2117931d82affaf2f186`.
- **Goal linkage:** ADR008's usable Render alternative; ADR004 service deployment, ADR021 managed Key Value, ADR049 Blueprint configuration and ADR018 dashboard parity. Customers must be able to enter configuration without silent loss.
- **Expected outcome:** the three SSR creation forms and Environment Groups list search expose editable native fields only after event handlers can retain input, then behave normally during client-side navigation and subsequent updates.
- **Why now:** eight fresh-page replays reproduce startup draft loss, including a nonempty default being silently restored. A separate fast-entry disappearance led to this probe but had no recorded cause. This filing relies on the controlled startup replays, not that earlier untraced occurrence.
- **Scope:** a local readiness gate at the four observed roots (three creation forms and Environment Groups list search), reusing `useIsHydrated`; preserve the shared hook and global Input behavior. All five service types have observed Name failures at the a797/a800 widths; aliases, other source tabs/widths, additional fields and other dialog-based creates remain the explicit t004 audit. Preserve the passing Postgres dialog control.
- **Follow-up a800:** [four more service types and the Postgres dialog control](finding-a800.md), with [seven durable receipts](../../evidence/m219-a800-siblings.json). All five service types now have observed Name failures at their listed widths. Aliases, other widths/source tabs, additional fields and other dialogs remain the bounded t004 audit. The Postgres dialog was observed to avoid pre-handler editing and should retain that behavior.
- **Passing control a805:** the [webhook create page](control-a805.md) exposes Name/URL only after startup and preserves drafts through catalog retry at both widths. Preserve that existing behavior; this is not another affected form root. [Durable evidence](../../evidence/m219-a805-webhook-controls.json).
- **Follow-up a815:** [Environment Groups search startup loss](finding-a815.md), with [three failing replays, qualified Project comparison, complete native-field census and cleanup](../../evidence/m219-a815-search-startup.json). Add this fourth local root; direct-field census is 103 Input + 8 Textarea + 12 native input + 2 native textarea sites (125 syntactic sites; wrappers need expansion). Nine tasks total3h30. This search loss is minor; the creation-draft loss remains major.
- **Render parity:** included because the dashboard creation/search surface changes. Render's public creation documentation describes entering configuration; its authenticated pre-hydration behavior was not tested. Preserve REST/GraphQL/MCP input and validation contracts.
- **Dedupe:** all 85 actual open/blocked milestone titles, open/done text searches and DO_NOT_DO reviewed; no matching item. m218 covers failed plan queries after startup, while this loss occurs before handlers with unchanged successful responses. Recent 40 product commits and targeted hydration/name history show no ready-to-deploy fix.

## Implementation record (2026-10-16)

- **t001–t003:** `HydrationFieldset` (`dashboard/src/common/components/hydration-fieldset.tsx`) wraps the `CardContent` of `/keyvalue/new`, `/services/new` (every type, so Name and Port among them) and `/blueprints/new` (Name and path). It is `disabled` with `aria-busy` while `useIsHydrated()` is false: a native disabled fieldset, so the server HTML itself rejects typing. It uses `display: contents`, so `space-y` and the card's flex layout are unchanged, and it becomes an ordinary enabled fieldset once hydrated (or on client navigation).
- **t009:** the `/env-groups` search `Input` is `disabled={!hydrated}`: visible, but not editable before handlers. Matching and reset are unchanged.
- **t004:** only these four roots were gated. The passing controls (webhook create, the Postgres create dialog) and the shared Input, `useIsHydrated` and the other native fields are untouched. Aliases (`/web|pserv|worker|cron|static/new`, `/new/redis`, `/r/new`) redirect into the gated canonical routes.
- **t005/t006:** ADR018's create-service row records the startup behavior. Reviewed inline: one small component and four call sites.
- **t007:** `hydration-fieldset.test.tsx` runs a real `renderToString` → `hydrateRoot`. The server HTML has a disabled fieldset with its input `:disabled`, and hydration enables it with `aria-busy` cleared. Route tests check that the Key Value Name, service Name/Port and Blueprint Name/path sit inside the fieldset and that a client render is enabled. Dashboard `yarn test` (5,105) and `yarn lint` pass. **Not run:** the browser startup replays with a held script (t008); the env-groups search is covered by the same hook, not an SSR route test.
