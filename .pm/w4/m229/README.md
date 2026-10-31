# w4 · m229 — Attribute Blueprint changes without repeating source switches

**Worker:** worker4 **Goal:** a source-migration preview names actual configuration differences instead of blaming unchanged fields. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Attribute service fields with source-transition context — **DONE** | 40m | — |
| [t002](done/t002.md) | Audit shared callers and inherited plan/source contracts — **DONE** | 30m | w4/m229/t001 |
| [t003](done/t003.md) | Render parity — **DONE** | 15m | w4/m229/t002 |
| [t004](done/t004.md) | Simplify — **DONE** | 10m | w4/m229/t003 |
| [t005](done/t005.md) | Test coverage — **DONE** | 35m | w4/m229/t003 |
| [t006](t006.md) | Closeout | 20m | w4/m229/t004, w4/m229/t005 |

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

## Implementation record (2026-10-17)

- **t001:** `ApplyBlueprintServiceSpec` now wraps `probeBlueprintServiceSpec(dst, want, fields, only)`. With `only` empty it is the unchanged full apply. With `only` set, it runs just that path's applier, or what omitting that path does, and judges the context rules against every field the file declares. Those rules are the inherited repo, the baked start command drop, the Dockerfile settings a strategy change clears, buildFilter's clear-on-omission and numInstances beside scaling. The planner (`blueprint_state_plan.go`) probes each declared field plus every service path the file omits (`blueprintServiceProbePaths`, from `BlueprintServiceFieldPolicies`). So an omission that changes the service is listed once, under its own path: an image service taking the Blueprint's repo shows as `repo`, and a cleared filter as `buildFilter`. The old one-field probe made a declared `repo` look omitted, so each probe re-ran the w8/m70 switch. The implicit repo rule, the real apply, migration and branch resolution are unchanged. A probe error is still listed rather than dropped.
- **t002 census:** all five action-plan callers (inline deploy `deploy.go:712`, validation `blueprint.go:732`, create `:1117`, prepare-sync `:1308`, run-sync `:1488`) reach the one `PlanBlueprintResource` service branch, so they share the repair. Of the three service-applier calls, the actual-apply wrapper (`deploy.go` `applyBlueprintServiceSpec`) and the full plan probe go through the unchanged full mode; only the isolated probe moved. Postgres and Key Value keep their `oneField` probes (now passed `maps.Keys`). EnvGroup keeps its conservative declared-set update, and generated copies keep their `envVars` possible update. Web, static, cron, worker and private services share the one family-agnostic applier, and there is no per-type branch in the probe. The dashboard's two PlanSummary mounts (`/blueprints/new`, `/blueprints/$blueprintId`) render the producer list unfiltered and are unchanged. m124's DoD rows: the export → noop guard (`TestGeneratedBlueprintRePlansAsNoop`, `TestGenerateBlueprintRoundTripsRealCreates`), single-field detection (`TestPlanNamesOnlyTheFieldThatChanged`), domains/KV allow-list round trips and the m70/m78 source suites (`blueprint_source_switch_test.go`, `blueprint_service_source_test.go`, `blueprint_default_branch_test.go`, `blueprint_implicit_repo_test.go`) all pass unchanged. The hosted web/PG/KV export matrix stays unverified, as the finding records. The grouped appliers (`autoDeploy|autoDeployTrigger`, `domains|domain`) still list both declared names when either moves; that is pre-existing and unchanged.
- **t003:** `TestBlueprintSourceSwitchAttributionAcrossSurfaces` validates the image→Git manifest through REST multipart, GraphQL and MCP, and each returns the same exact update paths. Its shared `blueprintValidationSurfaces`/`actionPaths` helpers now also drive the empty-environment surface test. ADR049 D6 records the attribution rule. Render's authenticated field presentation remains unverified, so no Render parity claim is made beyond its public review-before-sync flow.
- **t004:** /simplify ran three reviews. Fixes: one `inScope` check, `blueprintPlanFieldChanges` takes path names (`iter.Seq`) rather than a map padded with dummy fields, two stale comments, the dead `anyPresent` removed, and the cross-surface harness shared. Efficiency: the extra undeclared-path probes are in-memory and run only for services already planned as updates, so no change.
- **t005:** `blueprint_source_attribution_test.go` runs through the real `PreviewBlueprint` path. It covers image → declared Git, image → the Blueprint's inherited Git (`repo` listed once), the reverse Git → image (`image`, `runtime`), settled image/Git no-ops, an omitted buildFilter alone and beside a health change, and numInstances beside scaling. `TestBlueprintPlanAttributesASourceSwitchToRepo` now requires the exact list; its old "repo and runtime among them" check is what let the regression through. Against the pre-fix planner the switch, inherited, omission, scaling, cross-surface and tightened tests fail (the declared-Git case reproduces the live twelve-path list exactly). The reverse and no-op controls pass both before and after. The `internal/apps` package and backend lint pass.


## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m229（同组 D4：w4/m169）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D4，非本次测试的旧 Blueprint）：** 允许删除 bex-canary 的 Blueprint `blp-db136288mmqc73d4hpug`（"bex"，2026-10-04 创建，autoSync，跟踪 `bex-co/bex@main:render.yaml`——该文件在 main 上已不存在；不管理任何资源，2026-10-29 只读核实）。删除后用 worker 自建的 Blueprint 跑验收，完成后删除自建 Blueprint 及其资源。不需要 QA 分支。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 删除该 Blueprint 并确认 404，再按本项原 DoD 重放剩余检查。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
