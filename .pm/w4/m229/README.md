# w4 · m229 — Attribute Blueprint changes without repeating source switches

**Worker:** worker4 **Goal:** a source-migration preview names actual configuration differences instead of blaming unchanged fields. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Attribute service fields with source-transition context | 40m | — |
| t002 | Audit shared callers and inherited plan/source contracts | 30m | w4/m229/t001 |
| t003 | Render parity | 15m | w4/m229/t002 |
| t004 | Simplify | 10m | w4/m229/t003 |
| t005 | Test coverage | 35m | w4/m229/t003 |
| t006 | Closeout | 20m | w4/m229/t004, w4/m229/t005 |

Six tasks,150m total (2h30m). One **minor** field-attribution regression, with a shared producer audit and meaningful source-transition controls; this ships scheduling only.

## Definition of done

Replay the ordinary owned Free workflow in [finding.md](finding.md), with the [durable requests, source probe and controls](evidence.json). Start only when the committed QA manifest's service name is absent; never adopt a foreign resource.

- Fresh1440 and390, from a settled image and the explicit Git target: one Update still identifies the real source/command differences; unchanged matching name and Free plan are absent from changedFields and the rendered Changes list. Verify all declared candidates against actual before/after intent, rather than excluding those two labels. Retain real differences and an explicit disposition when evaluation is unknown.
- Cancel each reviewed dialog: no SyncBlueprint write, no extra deploy; the existing image continues serving. Keep one reviewed commit/owner/resource identity across preview and a confirmed sync.
- Preserve the controls actually run: initial image Live/HTTP200; identical image Sync opens no deployment; confirmed Git transition reaches the correct own latest Live child and serves the independently expected package hash and exact build/runtime literal; settled Git preview is No change; reverse image preview names its actual image/runtime differences and the image becomes Live again. Do not suppress source changes to make the list short.
- Complete t002's shared-surface, resource-family and historical-DoD dispositions. Local source/adapter regressions distinguish the failing direction and passing controls. Relevant backend and dashboard checks pass; no fake passing diagnostic or blanket UI filter substitutes for the producer repair.
- Disconnect preserves the service; then delete the owned service and exact children. Restore all seven inventory-ID sets, verify exact runtime metadata/public404 and revoke only that run's session before closeout.

## Source + Goal linkage

- **Source:** functional qa-find-bugs w4,a903,2026-10-17 UTC; [finding](finding.md), [complete durable evidence](evidence.json). Current research includes origin45fabea3046b06810ead7ee18f60f2a44d2d1de0; serving platform SHA unverified.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) reliable hosting and [ADR049](../../../docs/ADR049-render-yaml-parity.md) a plan that describes what apply changes; restores done w4/m124's changedFields guarantee in the source-switch context introduced by eebd8ce27.
- **Expected outcome:** the same true Update is reviewable without equal name/plan/config being mislabeled; actual migration, no-op and reverse-source behavior remain correct.
- **Why now:** both fresh breakpoints reproduce a backend-generated twelve-field list; actual-source probes identify repeated implicit migration, while normal migration and reverse/no-op controls pass. A user cannot use this list to assess a source edit's real scope.
- **Render parity included:** REST/GraphQL/MCP preview/validation and both dashboard consumers share the plan. Render's public review/manual-sync/disconnect behavior was checked; exact authenticated field presentation remains unverified.

## Historical compatibility disposition

| Original m124 DoD | a903 evidence / work still required |
| --- | --- |
| Export → validate is noop | Existing focused local tests pass; full hosted web/PG/KV export replay not run. t002 records the maintained guard and t005 extends it where needed. |
| Domain-carrying manifest stable | No owned domain replay. Preserve existing domain normalization/tests; no new domain work or foreign mutation. |
| Key Value empty allow-list stable | No live Key Value in this pass. Existing round-trip guard passed locally; preserve its branch. |
| changedFields names only differences | Fails for image→Git; fresh1440/390 and actual-applier name-only probe are retained. Ordinary health/PG-plan/KV-policy local single-field controls pass. |
| A real change remains detected | Actual source Update deploys correct Git artifact; reverse preview names image/runtime. Preserve and extend those plus ordinary local controls. |
| Guard all exportable families | Current focused local guards passed; no full hosted web/static/cron/worker/private/PG/KV/group matrix claimed. t002 enumerates source eligibility and t005 records meaningful test coverage. |

Done m70/m78 core migration behavior passed here, including explicit different service repository and exact artifact/literal; their complete native-client, rollback, omitted-source and persistence matrices were not replayed. Preserve their tests and record each relevant neighbor. Original milestones remain archived and byte-preserved; this new regression does not certify or reopen their entire scope.
