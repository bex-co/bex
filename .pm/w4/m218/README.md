# w4 · m218 — Recover plan catalogs in creation and datastore details

**Worker:** worker4 **Goal:** failed plan-catalog reads explain unavailable choices and recover in place without losing create drafts or datastore plan selection. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Expose catalog retry without resetting consumer state | 35m | — |
| t002 | Recover service-create plans while retaining the draft | 30m | t001 |
| t003 | Recover Postgres and Key Value create catalogs | 40m | t001 |
| t009 | Recover the Postgres detail plan catalog | 25m | t001 |
| t010 | Recover the Key Value detail plan catalog | 25m | t001 |
| t004 | Audit the 13 hook consumers and route aliases | 25m | t002, t003, t009, t010 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 10m | t005 |
| t007 | Test coverage | 50m | t005, t006 |
| t008 | Closeout | 30m | t007 |

## Definition of done

- Cold-open `/services/new?type=web_service` at1440px and390px. Replay the browser-local `InstanceTypes` execution error in the [durable evidence](../evidence/m218-a785.json), keeping sibling batch results unchanged. Fill a valid name and `node:24-alpine` image. The settled catalog failure displays a localized, accessible explanation and an in-place Retry action; Deploy stays disabled. It does not masquerade as an indefinitely pending plan grid.
- Remove the interception, edit Port to3001 while retaining the typed name/image, and click Retry without navigating. A successful real catalog read restores actual offered plan cards, the normal Free default and valid Deploy eligibility; name, image and Port3001 remain. Pending retry has a visible bounded busy state and cannot start competing requests. No create/deploy mutation is sent by Retry.
- Repeat the captured initial-error and restored-network controls for `/keyvalue/new` and `/new/database` at desktop and390px. Key Value explains its otherwise empty radio group; Postgres explains its otherwise empty plan selector. Retry restores actual eligible catalog choices and normal valid create eligibility while retaining the typed name. No resource needs to be created to verify this contract.
- Repeat the captured `/static/new` control with the same `InstanceTypes` fault and a valid public Git draft: Static Site still has no compute-plan requirement and its valid Deploy action remains enabled. Never click Deploy in this read-only control.
- On new owned private Free Postgres and Key Value fixtures, cold-open their canonical detail URLs with `#plan` at1440px and390px. Replay the selected catalog-entry error from [a796](../evidence/m218-a796-details.json). The healthy resource remains Available; the Instance type card explains the settled failure and offers Retry instead of an empty radio group. Remove interception and click Retry without navigation: restore the three real catalog choices with current Free selected and unchanged Save disabled. Retry sends no plan mutation. These are read-only plan-editor checks; do not buy a plan to verify them.
- Re-run these probes against the deployed fix, record actual requests/results and screenshot the settled error and recovered forms. Restore baseline inventory and revoke only the replay's own session. Carry all unobserved cases in [finding.md](finding.md) through the implementation/audit tasks; this filing does not assert they already fail.

## Source + Goal linkage

- **Source:** user-requested infinite functional `$qa-find-bugs w4` loop, a785, 2026-10-16 UTC; [finding](finding.md), [durable probes and replay code](../evidence/m218-a785.json). Major usability defect under a **deliberately injected browser-local catalog error**; no production catalog outage was observed. Real catalog responses were HTTP200 with7/3/3 tiers. Research main `4052136f3`.
- **Scope extension:** a796,2026-10-16 UTC; [detail finding](detail-finding-a796.md), [complete selected native/synthetic entries and controls](../evidence/m218-a796-details.json). Both datastore detail editors also drop the same hooks' error state and render empty plan groups. Four fresh fault replays and two healthy controls extend this existing milestone instead of duplicating the shared-hook work. Detail-only severity is minor; no plan update or paid submission occurred. Both fixtures and their cluster resources were removed and the exact session revoked. Research main `925cc2010`.
- **Goal linkage:** ADR008 pillar1, reliable hosting creation; ADR004 service deployment, ADR009 managed Postgres, ADR021 managed Key Value, and ADR018/ADR006 surface consistency. A temporary read failure should offer recovery while the user keeps a valid hosting draft.
- **Expected outcome:** five producers (three create forms and two datastore detail plan editors) distinguish pending, settled unavailable and usable catalog states and can retry the read in place. Prices, compatibility checks and plan eligibility remain API-driven. New error rendering is allowlisted to those five; the other eight hook consumers keep their existing behavior.
- **Why now:** the newly landed329/330 and m208 changes fix pending geometry but explicitly preserve settled error/empty behavior. Fresh desktop/mobile fault replays expose a shared terminal-error recovery gap across five producers and three hooks. Reuse the same hook contract and close the observed detail gaps while the change is still scheduled; the13-consumer audit and standing checks cover the final scope.
- **Standing closing tasks:** Render parity is included because the dashboard changes. Preserve existing REST/GraphQL/MCP catalog/create semantics; no backend contract change is proposed. Simplify, behavioral coverage and live closeout follow. Shipping this milestone ships the filing only.

**Total estimate:** 4h50.
