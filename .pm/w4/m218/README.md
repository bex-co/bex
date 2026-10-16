# w4 · m218 — Recover create-form plan catalogs in place

**Worker:** worker4 **Goal:** a failed initial plan-catalog read explains why creation is disabled and can be retried without losing the draft. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Expose catalog retry without resetting consumer state | 35m | — |
| t002 | Recover service-create plans while retaining the draft | 30m | t001 |
| t003 | Recover Postgres and Key Value create catalogs | 40m | t001 |
| t004 | Audit the 13 hook consumers and create aliases | 25m | t002, t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 10m | t005 |
| t007 | Test coverage | 40m | t005, t006 |
| t008 | Closeout | 20m | t007 |

## Definition of done

- Cold-open `/services/new?type=web_service` at1440px and390px. Replay the browser-local `InstanceTypes` execution error in the [durable evidence](../evidence/m218-a785.json), keeping sibling batch results unchanged. Fill a valid name and `node:24-alpine` image. The settled catalog failure displays a localized, accessible explanation and an in-place Retry action; Deploy stays disabled. It does not masquerade as an indefinitely pending plan grid.
- Remove the interception, edit Port to3001 while retaining the typed name/image, and click Retry without navigating. A successful real catalog read restores actual offered plan cards, the normal Free default and valid Deploy eligibility; name, image and Port3001 remain. Pending retry has a visible bounded busy state and cannot start competing requests. No create/deploy mutation is sent by Retry.
- Repeat the captured initial-error and restored-network controls for `/keyvalue/new` and `/new/database` at desktop and390px. Key Value explains its otherwise empty radio group; Postgres explains its otherwise empty plan selector. Retry restores actual eligible catalog choices and normal valid create eligibility while retaining the typed name. No resource needs to be created to verify this contract.
- Repeat the captured `/static/new` control with the same `InstanceTypes` fault and a valid public Git draft: Static Site still has no compute-plan requirement and its valid Deploy action remains enabled. Never click Deploy in this read-only control.
- Re-run these probes against the deployed fix, record actual requests/results and screenshot the settled error and recovered forms. Restore baseline inventory and revoke only the replay's own session. Carry all unobserved cases in [finding.md](finding.md) through the implementation/audit tasks; this filing does not assert they already fail.

## Source + Goal linkage

- **Source:** user-requested infinite functional `$qa-find-bugs w4` loop, a785, 2026-10-16 UTC; [finding](finding.md), [durable probes and replay code](../evidence/m218-a785.json). Major usability defect under a **deliberately injected browser-local catalog error**; no production catalog outage was observed. Real catalog responses were HTTP200 with7/3/3 tiers. Research main `4052136f3`.
- **Goal linkage:** ADR008 pillar1, reliable hosting creation; ADR004 service deployment, ADR009 managed Postgres, ADR021 managed Key Value, and ADR018/ADR006 surface consistency. A temporary read failure should offer recovery while the user keeps a valid hosting draft.
- **Expected outcome:** three create producers distinguish pending, settled unavailable and usable catalog states and can retry the read in place. Prices and plan eligibility remain API-driven.
- **Why now:** the newly landed329/330 changes fix pending geometry but explicitly preserve error/empty behavior. Fresh desktop/mobile fault replays expose a separate terminal-error dead end across three producers and three shared hooks. The multi-form change, explicit13-consumer audit and meaningful interaction coverage exceed one hour; this is one scoped milestone.
- **Standing closing tasks:** Render parity is included because the dashboard changes. Preserve existing REST/GraphQL/MCP catalog/create semantics; no backend contract change is proposed. Simplify, behavioral coverage and live closeout follow. Shipping this milestone ships the filing only.

**Total estimate:** 3h40.
