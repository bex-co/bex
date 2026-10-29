# w8 · m64 — Keep Blueprint environment copies consistent across applies

**Worker:** worker8 **Goal:** sibling copies equal the source configuration across group links, seed state, immutable literal priority and own mutable overrides. **Status:** todo（2026-10-29 解除阻塞，待线上验收；原记录：implementation tasks shipped; sweep231 accepts observed live group/own-store/literal paths with native wire and cleanup receipts. t006 still needs remaining variants, shared live/snapshot surfaces and the complete inherited DoD.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resolve known values from effective link and seed state — **DONE 2026-10-13** | 55m | — |
| t007 | Guard unknown own mutable overrides — **DONE 2026-10-13** | 25m | t001 |
| t008 | Preserve immutable literal priority over same-key seeds — **DONE 2026-10-13** | 30m | t007 |
| t002 | Audit all parser/current-state consumers and inherited 099 DoD — **DONE 2026-10-13** | 35m | t008 |
| t003 | Render parity — **DONE 2026-10-13** | 15m | t002 |
| t004 | Simplify — **DONE 2026-10-13** | 10m | t003 |
| t005 | Meaningful test coverage — **DONE 2026-10-13** | 30m | t003 |
| t006 | Closeout — **BLOCKED: deployed API + native receipts** | 10m | t004, t005 |

## Definition of done

- All seven [actual group diagnostic cases](evidence/cli-sweep213/actual-backend-results.json) have matching source/copy configuration, or a located refusal before any write where the value is not known; current-state, first/reapply, duplicate, retained/omitted/new links and service-own literal/empty controls are covered with real feature services.
- All five [actual seed/edit/reapply cases](evidence/seed-copies-sweep214/actual-backend-results.json) have truthful source/copy values or a located preflight refusal when unknown. Seed-once keeps edits; first-create seed hints and immutable own literal/empty controls keep correct behavior. Audit prompt and mixed seed/reference/group neighbors without dynamic-value expansion.
- Preserve the complete original 099 acceptance in [finding.md](finding.md), including its now-passing 16 native linked-value/precedence/refusal checks, masks/external limits and all four parser callers. Verify metadata-backed/public IDs and nested/consumer-first cases; no global link-order change, dynamic-value/reference-chain expansion or CLI fork.
- All shared validation surfaces have consistent eligibility/errors at the authored caller entry. Record exact pinned native wire, actual deployment identity only if observable, hosted/snapshot limitations and any tracked follow-up explicitly; declarations are not hosted apply evidence.
- Meaningful regressions fail on the present wrong index and pass corrected. The diagnostic overlays expect current bugs and must not be copied as fixed expectations. Appropriate checks and simplify pass.
- Every future owned fixture/config/process is cleaned or precisely accounted for. All tasks and the complete observable DoD hold before moving to done; shipping code alone is insufficient.

- All eight [same-key literal/seed diagnostics](evidence/literal-seed-sweep222/README.md) retain the source literal (including empty) in accepted first/reapply copies, in either authored order. Preserve generated/literal controls, intentional seed-only existing-source refusal and reference masking; test nested/public-ID paths.

## Source + Goal linkage

- **Source:** promoted w8/099 and functional CLI sweep213, 2026-10-13 UTC; [researched finding/history](finding.md) and [native/source/operator evidence](evidence/cli-sweep213/README.md).
- **Goal linkage:** ADR008 reliable hosting, ADR049 Blueprint configuration, ADR013's deliberate last-linked precedence and ADR018 unchanged CLI contract.
- **Expected outcome:** an accepted sibling copy cannot silently disagree with its source because the parser guessed a different group order.
- **Why now:** the linked-literal fix 7a06a13f4 is live, but four actual source models expose silent mismatches, including a fresh duplicate shape admitted by both native clients; the original current-state acceptance gap is a real implementation gap.
- **Sizing:** originally155m; now210m total across eight tasks including the additional25m own-store and30m literal/seed implementation, so the prior sub-hour note is promoted. Completed original core work and all original DoD dispositions remain; this is one continued issue.
- **Render parity included:** shared REST/GraphQL/MCP/dashboard validation changes. Keep Bex's documented last-linked divergence; authenticated Render-production precedence was not measured.

## Source extension — sweep214

Three edited-seed configured-value mismatches join the four group-order mismatches under the same known-value index. [Actual feature-method proof and qualified native admission](evidence/seed-copies-sweep214/README.md). Source e2cc474297ba, deployed revision unknown; all local/native copies cleaned, no hosted resource. The milestone remains todo and retains every inherited original 099 obligation. No QA source implementation or ship.

The complete DoD also requires same-key own mutable overrides (including empty) to produce truthful copies or located preflight refusals before writes, with unrelated own keys and literal precedence preserved.

## Own mutable override follow-up — sweep219

[Five current feature-method states](evidence/own-overrides-sweep219/README.md) confirm two remaining mismatches after d6bc82929: a normal own VALUE edit, including empty, still wins at the source but accepted validation/reapply bakes group A into the consumer. Target-name inventory returns VALUE; unrelated-key, manifest-literal and no-own-store controls agree. Actual fake-backed apps/envgroups/secrets calls are used, with persisted process precedence inferred rather than hosted execution. Eight fresh native declarations are admitted, which is not existing-state/apply proof.

At lego/backend/internal/apps/deploy.go:1371–1425, currentStateCopyProblems checks seed and linked-group state, then derives knownServiceEnv without inspecting the same-key own mutable override. The existing names-only EnvNames seam is wired at lego/backend/internal/api/server.go:915 and implemented by secrets.ListEnvVarNames at service.go:1134. Operator envFromSources at app_controller.go:4777 places own after groups; literal env remains highest. Use names for target-key eligibility and a located consumer refusal for unknown current values; do not read arbitrary values or reject unrelated/manifest-literal controls.

This is the same m64 effective-known-index contract, not a new milestone. Add t00725m and reopen t002–t005/t006 for expanded shared eligibility and regressions. The existing group-order/seed fixes and prior shipped scope remain recorded; complete original099, group, seed, prompt/mixed-source, caller, native/surface and cleanup obligations are retained. No live fixture, product fix or ship by QA.

## Same-key immutable literal / seed priority — sweep222

[Eight actual feature-method states and sixteen unchanged native validation captures](evidence/literal-seed-sweep222/README.md) identify a remaining m64 index gap. With VALUE declared as both an immutable literal and a sync:false seed, the persisted source uses the literal in either authored order (including empty). If the seed is last, accepted first apply copies the seed instead: two configured-value mismatches. Reapply refuses all four literal/seed shapes as unknowable, although the literal is fixed. Generated/literal pairs in both orders and literal-only controls remain correct; the seed-only existing-source refusal stays intentional.

At deploy.go:2640–2662, classifyServiceEnv writes both ordinary literals and seeds into se.known by authored order. knownServiceEnv:2364–2376 sets knownValue.seed by membership in seedLiterals, even when the effective value is an immutable literal. classifyRefs:2223–2224 therefore adds an immutable copy to stateCopies, and currentStateCopyProblems:1389–1391 refuses it unconditionally when the source exists. Actual persisted explicit spec.Env has priority over own envFrom; ADR013's two-writers contract and the current operator projection preserve that rule.

Separate effective immutable literals from seed hints when forming the known index; literals, including empty, take priority over mutable seed sources in either authored order. Set seed provenance only when a seed is actually the effective source. Keep later resolved spec.Env references masking same-key literals (w8/102), seed-once retention and unknown-only refusals, groups, generated controls, all parser callers and the names-only own-store guard. Add t00830m before the shared audit, then rerun expanded regressions and complete acceptance.

The source evidence uses real feature methods with fake storage/Kubernetes; process precedence is inferred from persisted state, not observed in a hosted pod. Sixteen native fresh declarations are admitted with exact multipart captures, which proves admission only. No production existing-state/apply claim, source fix or QA ship. This extends the same known-index contract; no duplicate milestone or issue.

## Independent source replay — sweep226

[Current-source verification](../evidence/current-source-sweep226/README.md) reruns the original relevant diagnostics after the fix. Its source-only scope and remaining production/native/runtime gates are explicit; this receipt does not close the milestone. No QA product implementation or ship.

## Live existing-state acceptance — sweep231

[Owned runtime and exact native wire receipts](evidence/live-env-sweep231/README.md) accept the duplicate/reordered/retained-link, own ordinary/empty refusal and literal/seed priority paths in production. Four same-commit native deploys succeed and all two services, two groups and recorded deploy children are cleaned; the baseline Blueprint remains. This is partial acceptance: remaining variants, shared live surfaces/snapshots and complete original099 dispositions still gate closeout. The conservative all-authored-links-omitted refusal is recorded as a parse-time eligibility limitation, with no new defect filed.

## Nested source/seed acceptance — sweep241

[Sixteen same/split-environment declarations](evidence/nested-source-sweep241/README.md) preserve immutable literal/empty and generated/literal validation decisions against eight actual root-created fake states. Seed-only existing-state refusal locates its sole consumer entry. Both native clients accept all 32 fresh variants with exact multipart/decision receipts. This is source-only existing-state and fresh native validation, not nested apply, hosted current-state/runtime, public-ID or full m64 acceptance. No new defect or hosted fixture; temporary capture configs/relays are cleaned.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m64
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## Hosted remaining-variant acceptance — 2026-10-29 (`/loopx w8`)

Production pin `b33dd43c4428`, bex-canary, released Bex v0.3.3 and pinned Render v2.27.0 against Bex, human device-login QA grant. One nested Blueprint (`projects → environments → services`, consumer listed **before** its source) applied through `POST /v1/blueprints/deploy`: groups `-a` (`QA_G=LX8B_A`) and `-b` (`LX8B_B`), source `qa-20261029-lx8b-src` with a seed-only `QA_VALUE` (`value: LX8B_SEED, sync: false`) plus both group links, consumer `qa-20261029-lx8b-copy` copying `QA_VALUE` and `QA_G` via `fromService`.

- **Fresh seed-only first creation, nested, consumer-first:** both native validators and GraphQL/MCP report valid; apply 200. Stored and runtime (echo `ECHO_INCLUDE_ENV_VARS`) env agree — source `QA_VALUE=LX8B_SEED`, `QA_G=LX8B_B`; consumer `QA_COPY=LX8B_SEED`, `QA_GCOPY=LX8B_B`.
- **Existing seed-only source refusal:** reapplying with a new group still copying the seeded `QA_VALUE` is refused before any write by both native clients, MCP with `workspaceId`, GraphQL with `ownerId` and REST apply (HTTP 400), all locating `projects[0].environments[0].services[0].envVars[1]` (line 28, column 9).
- **New group appended:** without the seed copy, group `-c` (`LX8B_C`) appended last → source runtime `QA_G=LX8B_C`, consumer stored and runtime `QA_GCOPY=LX8B_C`.
- **Defect found and fixed:** the dashboard Blueprint page's **Validate** panel sent `validateBlueprint(bexYaml)` with neither `ownerId` nor `blueprintId`, so it validated in the caller's default workspace and reported the refused manifest above as valid. `ValidatePanel` now passes the Blueprint's id (server scopes to that Blueprint's own workspace, as `blueprintPreview` already does); regression `use-validate-blueprint.test.ts`. Live re-check of the panel waits for this fix's deploy.
- Cleanup: both services (API 404), three groups (204 → 404), project `prj-dbhftvtlp6pc7389fj90` and environment (404). No Blueprint record was created by the inline apply.
