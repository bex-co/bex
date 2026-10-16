# w4 · m219 — Make creation inputs editable only when their handlers are ready

**Worker:** worker4 **Goal:** creation forms never accept a draft that startup immediately discards. **Status:** todo

**Estimate:** 2h45 (8 tasks). Filing only; no product fix shipped by the hunt.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Gate Key Value creation inputs through hydration | 20m | — |
| t002 | Gate the shared service creation form through hydration | 25m | — |
| t003 | Gate Blueprint creation inputs through hydration | 25m | — |
| t004 | Audit native fields, aliases and existing hydration consumers | 15m | t001, t002, t003 |
| t005 | Render parity across creation entrypoints | 15m | t004 |
| t006 | Simplify the changed code | 10m | t005 |
| t007 | Test SSR edit readiness and retained drafts | 35m | t005 |
| t008 | Replay live evidence and close out | 20m | t006, t007 |

## Definition of done

- Fresh-load `/keyvalue/new` at 1440 and 390px while holding the real fetched startup script response as in [a797 evidence](../evidence/m219-a797-input-startup.json). Name is visibly unavailable for editing until its handlers are ready. Release the response unchanged, type a valid name, and observe the exact text retained through startup queries settling. Current behavior accepts trusted typing before handlers exist, then clears it.
- Repeat at `/services/new?type=web_service`: Name follows the same readiness rule at both widths. In the desktop Port replay, the pre-ready field cannot accept a change; after readiness, enter valid `8197` and observe it remain `8197` through normal renders, rather than restoring `3000`.
- Repeat at `/blueprints/new` at both widths for Blueprint Name. In the desktop Blueprint path replay, the pre-ready field cannot accept a change; after readiness, enter `examples/hello-go/render.yaml` and observe it retained, rather than restoring `render.yaml`. No deployment is required for this check.
- Ready forms retain their present labels, defaults, geometry, validation, source selection, plan/loading/error states and submission predicates. Entering text after handlers attach keeps that text, as the passing controls already demonstrate. Verify pending and ready geometry side by side at desktop and narrow mobile widths.
- The live replay submits no create/deploy action. Fixture inventory remains unchanged and this run's own session is revoked. Preserve the eight original failing captures and record fixed-build replays before moving this milestone to `done/`.

## Source + Goal linkage

- **Source:** ordinary functional `$qa-find-bugs w4`, pass a797, 2026-10-16 UTC; [researched finding](finding.md), [durable replay factories and complete DOM receipts](../evidence/m219-a797-input-startup.json). Source HEAD `00f2afcfb44a3626ef4c2117931d82affaf2f186`.
- **Goal linkage:** ADR008's usable Render alternative; ADR004 service deployment, ADR021 managed Key Value, ADR049 Blueprint configuration and ADR018 dashboard parity. Customers must be able to enter configuration without silent loss.
- **Expected outcome:** these three SSR creation forms expose editable native fields only after event handlers can retain input, then behave normally during client-side navigation and subsequent updates.
- **Why now:** eight fresh-page replays reproduce startup draft loss, including a nonempty default being silently restored. A separate fast-entry disappearance led to this probe but had no recorded cause. This filing relies on the controlled startup replays, not that earlier untraced occurrence.
- **Scope:** a local readiness gate at the three observed form roots, reusing `useIsHydrated`; preserve the shared hook and global Input behavior. Shared service variants, aliases, additional fields and dialog-based creates need the explicit t004 audit; they are not claimed as live failures here.
- **Render parity:** included because the dashboard creation surface changes. Render's public creation documentation describes entering configuration; its authenticated pre-hydration behavior was not tested. Preserve REST/GraphQL/MCP input and validation contracts.
- **Dedupe:** all 85 actual open/blocked milestone titles, open/done text searches and DO_NOT_DO reviewed; no matching item. m218 covers failed plan queries after startup, while this loss occurs before handlers with unchanged successful responses. Recent 40 product commits and targeted hydration/name history show no ready-to-deploy fix.
