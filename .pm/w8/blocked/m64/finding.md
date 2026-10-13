# Blueprint environment copies disagree with effective source configuration

Why: a sibling copy should equal its source's effective configuration, or refuse an unknowable value, instead of silently baking the wrong group literal.

**Severity:** major — actual persisted configuration mismatch; no production outage/data loss claimed. **Ownership:** backend Blueprint known-value index/current-state integration. **Estimate:** 3h30m total across eight tasks; completed original work is retained. **Status:** todo. Existing w8/099 is promoted because its remaining current-state/order check now requires a researched multi-task correction; the completed linked-literal fix 7a06a13f4 and its full sweep212 native acceptance remain valid.

**Evidence boundary:** seven actual public DeployStack/ValidateBlueprint cases compose the real apps/envgroups services over fake Kubernetes/versioned storage. Four mismatch, three controls agree. Seven actual operator projections retain the stored order. Fourteen production native fresh declarations and four exact wire captures prove admission, not hosted apply/current-state/runtime. Source HEAD d15c1e7fcfe0bc4806f398c9b94535560380e837, deployed revision unknown. Fake IDs are not owned production fixtures.

| Scenario | Stored source group order / effective VALUE | Consumer COPY |
| --- | --- | --- |
| Fresh declarations A,B,A | A,B / group B | group A |
| Existing A,B; reapply B,A | A,B / group B | group A |
| Existing B; declare A,B | B,A / group A | group B |
| Existing A,B; declare only A | A,B / group B | group A |
| Unchanged A,B | A,B / group B | group B |
| Service-local literal/empty override | own / empty | same own / empty |

The consumer is declared before its source in every model, guarding against a late one-service index patch. Group literals and metadata remain harmless fixture values. [Full source/native/operator receipts](evidence/cli-sweep213/README.md).

## Traced cause and scoped target

`apps/deploy.go:1936` publishes knownServiceEnv before classifyRefs. `knownServiceEnv:2232–2268` visits raw se.groupLinks in declaration order, overwriting values on every visit, including duplicates. Actual initial group composition at envgroups/initial_links.go:106 deduplicates refs using addString:2143. Existing-service apply at apps/deploy.go:957–975 calls LinkEnvGroup for each desired name; linkFetched at envgroups/service.go:1179 returns immediately for existing membership, and newly linked groups append at 1188. Omitted links remain. Those are correct established idempotent last-linked semantics, documented in ADR013 and w2/done/m94. Their order differs from the resolver's assumed order. Actual operator envFromSources:4772–4800 retains it, and explicit env overrides it.

Build the known group-derived value set from the refs the apply will actually retain/append, deduplicating by the real link identity. Use current non-secret App/group identity state where available; if a retained source cannot be resolved truthfully from declarations, return the existing kind of located caller-env refusal before any group/App write. Preserve fresh linked-literal copies and explicit service-local values, including empty. Do not globally reorder or unlink groups to make the compiler's guessed order true, read dynamic external values, fork the CLI or expand reference chains/generated-value support. Do not hardcode a duplicate-group ban: link idempotency is established behavior.

Four parseCompiledStack callers need a counted audit: compile/deploy, shared validation, disconnected grouping reclamation and generated-manifest self-check. Pure/fresh contexts and existing-state contexts must remain explicit; distinguish unavailable current state from an existing source with no refs. Preserve the recent reference-index masks (w8/102), empty presence (w8/092), error-entry locations (w8/100), current-state plans and seed-once semantics. The one index consumer is resolveRef through the sole classifyRefs production call. The initial composer and ordinary link paths remain the runtime authority.

## Acceptance, dedupe and limits

- The seven actual service/projection cases either produce matching source/copy values or a truthful located refusal before any mutation. Cover both first/reapply, consumer-first, duplicate and omitted links, newly appended links, undeclared retained groups, masks and own-literal/empty positives. Add metadata-backed/public-ID and nested grouping cases.
- Retain all original 099 DoD bullets below: linked string/zero/empty, local precedence, masks/external limits, current-state/reapply order, all four callers and shared surfaces. Sweep212 already passes the original 16 native decisions; preserve the accepted feature.
- Native validation continues to admit safe fresh duplicates with a truthful copy target, or a deliberately documented refusal only where value truth cannot be established. Capture unchanged pinned requests after the fix; do not claim CLI apply, which is absent.
- REST/GraphQL/MCP/dashboard validation errors agree on eligibility and the authored caller-env location. Real-DB/store-backed and active-snapshot/hosted checks remain unverified here; record explicit dispositions before closeout, never infer them from a desired image pin.
- All actual owned resources used by future acceptance must be reconciled. This sweep created none; baseline/quota and local capture cleanup pass.

Targeted open/done/source searches match existing099's explicit current-state/order gap, so it is promoted rather than duplicated. w2/done/m94's last-linked rule is preserved and not reopened. w8/102 handles local literals shadowed by references, not group membership order. The original corrected false-refusal evidence and all completed core work are retained below. No product implementation, commit, push, release or external issue submission by QA.

## Edited seed-once inputs expose the same known-index defect — sweep214

[Five actual apps/secrets cases and ten native declarations](evidence/seed-copies-sweep214/README.md) add three configured-value mismatches: a provided sync:false literal or empty seed is correctly retained as edited B on reapply, but consumer COPY becomes the original seed A/empty or a newly declared seed C. Unedited seed and immutable-literal controls agree. The normal local GetEnvVar confirms the retained source value, while the actual persisted consumer spec contains the incorrect copy. The source's own env Secret reference remains. This is configured-state proof over fake CR-only storage; hosted runtime, real-DB/public-ID mode and live existing-state native validation are untested.

classifyServiceEnv at deploy.go:2519–2525 treats a provided create-time seed as known on every parse. knownServiceEnv:2257 then overlays that hint before the index is published at 1936. Actual SeedEnvVars at secrets/service.go:664–665 deliberately preserves an existing value. Keep that seed-once behavior; make the current-state known index truthful, or refuse a mutable source value that is not knowable from declarations before writes. First-create seeds and immutable own literals/empty still need their valid behavior. Do not silently reset the source or read dynamic values just to satisfy a guessed literal.

This adds 10m to implementation, 10m to consumer/current-state audit and 10m to tests: total 155m. The milestone is widened around one root cause—declared hints mistaken for effective inputs—rather than creating a second implementation issue. The complete original 099 and prior group-order requirements remain, as do the now-correct w8/092 presence and w8/102 reference masks. Prompt-answer hints and seed/reference/group collisions need explicit counted audit; no passing claim is made for them here. Current group-copy/shadowed-reference neighbor suites pass; all native configs/captures are cleaned and quota is baseline 6/PG0/KV0/term0. No QA code fix or ship.

## Preserved original w8/099 finding and acceptance history

### Original 099 — Blueprint envVarKey copies omit declared linked-group literals

Why: a service's same-file environment-group value is already known, yet another service cannot copy it through the advertised Blueprint reference form; accepting this combination keeps ordinary shared hosting configuration composable (ADR008/ADR049/ADR018).

**Historical status at sweep212:** todo — 25m remaining acceptance. Source fix and primary production native gate pass; remaining original checks are required before closeout.

**Severity:** minor. **Estimate:** 55m. **Attribution:** backend Blueprint reference indexing. **Context:** 2026-10-13 UTC, human OAuth in bex-canary (`tea-daif693dqjvc73e7as3g`), production `https://api.bex.co/v1/`, non-TTY JSON. Bex v0.3.2; unchanged Render v2.27.0 at `a764810a768202704e7206eb7b87a47211fcd98e`, pointed at Bex. Source/filing HEAD `6b5c23b6c5faef6bee076bc0a36582e9441008ca`; deployed revision unknown. Filing only; no implementation or ship by QA.

### Reproduction and expected behavior

[Complete linked-literal document](../../evidence/099/linked-literal.yaml): group qa182-f8e73e-group defines `VALUE: source-182`; source service links that group through `fromGroup`. The consumer uses `fromService: {type: web, name: qa182-f8e73e-source, envVarKey: VALUE}`. Both services are declared Free image web services. The values are harmless fixtures; nothing is applied or created.

```sh
bex blueprints validate linked-literal.yaml --confirm -o json
render blueprints validate linked-literal.yaml --confirm -o json
```

Use isolated configuration, explicit Bex host and the QA workspace in both clients. **Actual:** valid:false/exit1, empty stderr, a located error saying the source has no such plainly-defined variable. Root and nested forms fail, as do group literals zero and empty string. [The local-override control](../../evidence/099/local-override.yaml), with the same linked group but a service-local VALUE, validates and copies local-override correctly.

**Expected:** same-file linked group literals participate in the source's parse-time-known variable lookup; these four cases return HTTP200/valid:true, no errors and native exit0. Plain direct copies and local overrides continue to work. Missing keys and unlinked groups still fail; a declared group alone must not inject values into an unlinked service.

Bex's capability inventory calls envVarKey a translated copy of a parse-time-known sibling value, and parsed group literals already meet that condition. Render documents that linked groups supply service variables and that individual service settings override group settings ([environment groups and precedence](https://render.com/docs/configure-environment-variables#linking-a-group-to-a-service)); its [Blueprint references](https://render.com/docs/blueprint-spec#referencing-service-properties) read other service environment values. Applying those rules to this same-file literal combination is an inference; exact validation on Render production is unmeasured. This is not a request for interpolation, dynamic external-group reads or generated-value resolution.

### Durable evidence and controls

[Sixteen original native checks](../../evidence/099/native.json), [matrix driver](../../evidence/099/native-driver.py.txt): eight false refusals (root/nested/zero/empty through both clients), four positive controls (direct literal and local override), four correct refusals (missing group key and unlinked group). Processes finish normally; output/exit semantics are consistent with the server's valid field.

[Four fresh actual native multipart captures](../../evidence/099/native-wire.json), [capture driver](../../evidence/099/capture-driver.py.txt): linked-literal fails and local-override passes through each client. Native durations are 0.893/0.877s for refusals and 1.585/3.765s for controls. Each exact ownerId/file body receives HTTP200. Complete request boundary/content type, body, response headers and response JSON are retained; native validity/errors equal captured validity/errors. No request or file rewrite, no Authorization recorded. The pin builds ownerId+file multipart in cmd/blueprintvalidate.go:96–128 and chooses its exit from JSON200.Valid, so the launcher is not the failing boundary.

One initial capture-only invocation failed because the QA recorder reused compressed response bytes as request bytes and raised UnicodeDecodeError while recording. It is retained and excluded. Separate request/response buffers produce the four eligible captures; the original sixteen normal-client receipts were unaffected. Validation was the only endpoint; no resource mutation was replayed.

[Eight actual compiler/parser diagnostics](../../evidence/099/parser-result.json), [input](../../evidence/099/parser-input.json), [virtual probe](../../evidence/099/parser-probe_test.go.txt), [overlay](../../evidence/099/parser-overlay.json): all eight documents pass the real compiler. Real parseEnvGroup returns present VALUE strings source-182, 0 and empty in the failing cases, while real parseService records the group link but has no VALUE in known. resolveRef and parseCompiledStack reproduce each native refusal. Direct/local-override controls copy the expected strings. This asserts current behavior for diagnosis, not a product fix regression or full suite.

[Four existing targeted tests](../../evidence/099/existing-tests.json) pass under Go1.26.8/GOWORKoff, including group-link/copy integration and numeric root/group behavior. Their group and copied keys differ, so they do not exercise copying an inherited group key.

### Root cause and shared consumers

[Producer/consumer excerpts and exact caller census](../../evidence/099/source.txt):

- deploy.go:1837–1857 parses group literals before services; parseEnvGroup:2136–2165 stores them, including empty and numeric-converted values.
- classifyServiceEnv:2376–2430 records fromGroup only in groupLinks and continues before updating known. parseCompiledStack:1911 then assigns only se.known to idx.serviceKnownEnv. The already-parsed group literal maps never join this source index.
- classifyRefs:2071 passes that incomplete index to the sole production resolveRef caller. resolveRef:2721–2732 checks the source's map and refuses the missing key. There is one production classifyRefs caller and one classifyServiceEnv caller.
- The eventual apply path, deploy.go:942–961, links the group's values to that service. Operator envFromSources:4772–4799 emits group sources before the service's own source; group values are real service environment values, and local values retain precedence. Live apply/runtime was not invoked here.
- Four direct parseCompiledStack consumers: compileStack/deploy preparation at deploy.go:1687, validation at blueprint.go:661, disconnected-Blueprint grouping reclamation at 1607 and generated-manifest self-check at blueprint_generate.go:240. REST:1346, GraphQL:1259 and MCP:1024 validation share ValidateBlueprint; the dashboard consumes the shared result. Recheck all callers while keeping unrelated numeric fields typed.

### Proposed scoped correction and acceptance

Build the source's known-value index from linked, declared group literals in Bex's documented link order, then overlay its service-local declared values. Retain existing group links rather than flatten group values onto the source's spec.Env. Track unresolved local declarations and generated group entries so a higher-priority unknown value cannot silently fall back to a lower group literal. Preserve genuinely missing/unlinked refusals; avoid reading undeclared/existing groups during this parser change. Review current-state/link ordering during reapply instead of assuming manifest order always equals existing link order.

One sub-hour task: indexing correction 35m; meaningful precedence/presence regressions 10m; shared-caller/native acceptance 10m. It does not require a new milestone or a CLI fork.

- [ ] Both native clients accept root/nested/zero/empty linked-literal cases with HTTP200/valid:true/exit0. Actual parsed consumer values are source-182, 0 and empty, with present-empty distinguished from absence.
- [ ] Direct literal and local-override controls retain exact values; missing-key and unlinked controls remain located refusals.
- [ ] Cover forward declarations, ungrouped form, deterministic declared-group order and local-over-group precedence. Unknown local prompt/reference/generated values must not resolve from a shadowed lower group value.
- [ ] Verify current-state/reapply group order and unknown external-group limitations explicitly. Preserve links, seed-once behavior, numeric formatting and original strings.
- [ ] Verify all four parser consumers and shared REST/GraphQL/MCP/dashboard validation semantics; capture fresh native requests after the deployed behavior changes. Record remaining live apply/runtime and surface-write gaps.

### Dedupe, limits and cleanup

Open/blocked/done searches for group-copy, linked literals, envVarKey/fromGroup and serviceKnownEnv found no matching item. w1/done/m35 introduced each form but tests a separate direct copy; w8/092 concerns empty sync:false seed presence, w8/095 concerns applying a newly empty key to an existing group, and w8/done/m53 concerns numeric decoding. This refusal also affects ordinary nonempty group strings, so none fixes the missing inherited-value index. History traces indexing to 8b4e4f18e and refactor 04380d92e; capability classification 2141bfd4e retains the parse-time-known contract. No later landed merge appears in the current source, independently reproduced by the diagnostic; exact deployed SHA remains unknown.

Reviewed .pm/DO_NOT_DO.md and the supported CLI Blueprint ledger. [Metadata, redactions and cleanup](../../evidence/099/metadata-cleanup.json), [redaction policy](../../evidence/099/redactions.json): no hosted resource, group, generated runtime value or local container created. All proxy/config copies removed; final quota services used 6/terminating 0, PG/KV used 0/terminating 0. Baseline resources were not mutated.

Live Blueprint apply/reapply/runtime, current-state group collisions, generated/prompt/external-group values, GraphQL/MCP/dashboard live validation and Render-production behavior remain unverified. This finding schedules a fix; QA did not implement or ship it.

### Outcome (2026-10-13) — fix shipped, blocked on hosted acceptance

- **Fix:** `knownServiceEnv` (`lego/backend/internal/apps/deploy.go`) builds each source's known-value index from its linked in-file groups' literals in link order (a later group wins, as the operator's `envFrom` does), with a group `generateValue` key masking lower literals, then overlays the service's own literals/seeds; a local reference, generated value or unanswered prompt masks any group value. A link to a group not declared in the file makes its keys unknowable, so then no group value is indexed and the existing refusal stands (recorded limitation: external/existing groups are not read at parse time). Group links stay links; nothing is flattened onto `spec.Env`. The index's only consumer is `resolveRef`, behind all four `parseCompiledStack` callers and REST/GraphQL/MCP validation.
- **Tests:** `TestBlueprintEnvVarKeyCopiesALinkedGroupLiteral` (`blueprint_group_copy_test.go`) asserts the copied value through the real compile+parse path: `source-182`, `0` and present-empty from a linked group, local override, later-group precedence, and refusals for a masking later generated group value, local generated value, local prompt, missing key, unlinked group and undeclared linked group; plus a nested project/environment validation. The four linked-literal cases fail on the original index; backend apps tests pass.
- **Blocked:** the native replay (root/nested/zero/empty valid in both clients, controls unchanged) needs the fix deployed to production; live apply/runtime and current-state reapply link order stay unexercised.

### Production native acceptance — sweep212, 2026-10-13 UTC

All 16 original linked-group native cases now match intended decisions: root/nested/zero/empty linked literals are accepted, direct/local overrides pass, missing/unlinked values refuse. Remaining declared-group/current-state/reapply order, masking/external-group, caller and non-REST acceptance stays explicit. Validation alone does not prove runtime copy values. [Complete matrices, seven exact wire receipts and cleanup](../../evidence/blueprint-fixes-sweep212/README.md). Same published/pinned clients and isolated human grant; desired API pin c1dab29d39c9, actual deployed SHA unknown. No resource or paid feature applied, no duplicate finding or QA implementation/ship. The previous native deployment blocker is removed; existing source-fix and historical failure evidence is retained.

## Own mutable override follow-up — sweep219

[Five current feature-method states](evidence/own-overrides-sweep219/README.md) confirm two remaining mismatches after d6bc82929: a normal own VALUE edit, including empty, still wins at the source but accepted validation/reapply bakes group A into the consumer. Target-name inventory returns VALUE; unrelated-key, manifest-literal and no-own-store controls agree. Actual fake-backed apps/envgroups/secrets calls are used, with persisted process precedence inferred rather than hosted execution. Eight fresh native declarations are admitted, which is not existing-state/apply proof.

At lego/backend/internal/apps/deploy.go:1371–1425, currentStateCopyProblems checks seed and linked-group state, then derives knownServiceEnv without inspecting the same-key own mutable override. The existing names-only EnvNames seam is wired at lego/backend/internal/api/server.go:915 and implemented by secrets.ListEnvVarNames at service.go:1134. Operator envFromSources at app_controller.go:4777 places own after groups; literal env remains highest. Use names for target-key eligibility and a located consumer refusal for unknown current values; do not read arbitrary values or reject unrelated/manifest-literal controls.

This is the same m64 effective-known-index contract, not a new milestone. Add t00725m and reopen t002–t005/t006 for expanded shared eligibility and regressions. The existing group-order/seed fixes and prior shipped scope remain recorded; complete original099, group, seed, prompt/mixed-source, caller, native/surface and cleanup obligations are retained. No live fixture, product fix or ship by QA.

## Same-key immutable literal / seed priority — sweep222

[Eight actual feature-method states and sixteen unchanged native validation captures](evidence/literal-seed-sweep222/README.md) identify a remaining m64 index gap. With VALUE declared as both an immutable literal and a sync:false seed, the persisted source uses the literal in either authored order (including empty). If the seed is last, accepted first apply copies the seed instead: two configured-value mismatches. Reapply refuses all four literal/seed shapes as unknowable, although the literal is fixed. Generated/literal pairs in both orders and literal-only controls remain correct; the seed-only existing-source refusal stays intentional.

At deploy.go:2640–2662, classifyServiceEnv writes both ordinary literals and seeds into se.known by authored order. knownServiceEnv:2364–2376 sets knownValue.seed by membership in seedLiterals, even when the effective value is an immutable literal. classifyRefs:2223–2224 therefore adds an immutable copy to stateCopies, and currentStateCopyProblems:1389–1391 refuses it unconditionally when the source exists. Actual persisted explicit spec.Env has priority over own envFrom; ADR013's two-writers contract and the current operator projection preserve that rule.

Separate effective immutable literals from seed hints when forming the known index; literals, including empty, take priority over mutable seed sources in either authored order. Set seed provenance only when a seed is actually the effective source. Keep later resolved spec.Env references masking same-key literals (w8/102), seed-once retention and unknown-only refusals, groups, generated controls, all parser callers and the names-only own-store guard. Add t00830m before the shared audit, then rerun expanded regressions and complete acceptance.

The source evidence uses real feature methods with fake storage/Kubernetes; process precedence is inferred from persisted state, not observed in a hosted pod. Sixteen native fresh declarations are admitted with exact multipart captures, which proves admission only. No production existing-state/apply claim, source fix or QA ship. This extends the same known-index contract; no duplicate milestone or issue.
