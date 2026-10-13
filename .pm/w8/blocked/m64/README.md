# w8 · m64 — Keep Blueprint environment copies consistent across applies

**Worker:** worker8 **Goal:** sibling copies equal the source configuration across group links, seed state, immutable literal priority and own mutable overrides. **Status:** blocked — t001–t005, t007, t008 done and shipped (2026-10-13); t006 needs the API deployed to production plus fresh pinned native validation captures and hosted/snapshot dispositions.

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
