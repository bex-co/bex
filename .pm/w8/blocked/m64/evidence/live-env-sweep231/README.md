# Existing Blueprint environment runtime — sweep231

Live functional QA in the isolated human session, workspace bex-canary/tea-daif693dqjvc73e7as3g. Source a11a77272e9a85debb788c4c6eb1aa53f7e8ac54; deployed API/operator revision unknown. Published Bex 0.3.2 and unchanged pinned Render 2.27.0/a764810a7682. No product implementation, QA commit/push or external filing.

Two owned Free Node services and two owned environment groups exercised the actual existing state. Both clients ran 24 native validation commands, with ten additional [unchanged multipart requests and replies](native-validation-wire.json) captured through owned temporary forwarding relays. Validity/errors/source locations agree with wire; native success decoding can omit server-only plan/pricing extras, so full-object equality is not assumed. Inline setup/reapply and sparse own-env editing used the documented APIs because the pin has no Blueprint apply/env-group command. Four actual native same-commit manual deploys exited zero. All state transitions and receipts are in [journeys](journeys.json) and [owned resources, runtime and cleanup](owned-native-runtime-cleanup.json).

| Case | Observable result |
| --- | --- |
| Fresh A → B → A group links | Source VALUE and sibling COPY both B at build and runtime. |
| Reverse authored links on reapply | Existing A → B links remain; both values stay B. |
| Omit B while A is still authored | Retained B still wins; accepted copy equals B. |
| Retained group not declared in-file | Both native validators refuse at the consumer entry; apply 400 before writes. |
| Own ordinary VALUE edit | Native Bex deploy builds/runs the edited own value. Both validators refuse to guess its copy; API 400 leaves the prior consumer B. |
| Own empty VALUE edit | Native Render deploy builds/runs exact empty. Both validators refuse it as unknown; API 400 retains prior consumer B. |
| Literal + sync:false seed, either order | Accepted first update/reapply; source and copy both literal, despite retained own empty. Native Bex consumer rebuild succeeds. |
| Empty literal + seed, either order | Accepted; source and copy both exact empty at build/runtime. Native Render consumer rebuild succeeds. |

The literal/seed and own-store fixes are live on these observed paths. An intentional refusal is not an accepted wrong copy: after refusing an edited source, the consumer correctly retains its previous B configuration. These receipts are partial m64 acceptance, not full closeout. Remaining hosted variants, live GraphQL/MCP/dashboard/snapshots and complete inherited 099 dispositions are listed in [metadata](metadata.json).

## Conservative omission boundary

Omitting every authored group link (source envVars: []) refuses the copy in both clients, while runtime retains both links and B. Keeping one authored link admits it. Four [actual current feature-method diagnostics](omitted-probe-results.json) confirm that omission, explicit empty and an unrelated-only env entry all stop in pure compilation, before current-state reconciliation; the one-link control is valid. They first prove a no-reference reapply retains both links. The [diagnostic](retained-link-diagnostic_test.go.txt) asserts current behavior, not a fix regression.

At deploy.go:2052 the pure index uses authored links; classifyRefs:2237 calls resolveRef:2991–3009 before currentStateCopyProblems can run at validation/apply. The documented local contract restricts copies to plainly defined, parse-time-known sibling values. Expanding that eligibility to otherwise undeclared service vars has not been established as required, so this conservative refusal is recorded as a limitation, not a new defect or a new m64 task. No dynamic external-group value lookup is proposed. Render's [Blueprint reference](https://render.com/docs/blueprint-spec#referencing-service-properties) documents preservation and re-copy on sync, but exact authenticated Render behavior for this shape was not measured.

## Verification and cleanup

Three current backend parent suites passed: TestBlueprintCopiesFollowTheSourcesEffectiveConfiguration, TestBlueprintExplicitEmptyValueIsALiteral, TestBlueprintEmptySeedIsSeededOnceAndRetained. See [test events](focused-test-events.jsonl), [diagnostic events](omitted-probe-events.jsonl) and [source identity](source-identity.json). Private Go overlays modified no repository source; no full suite, real DB suite or new fixed regression is claimed.

All two services, 10 recorded deploy rows and two groups are removed by exact ownership-qualified IDs. Native service deletion is followed by API 404, list absence, former runtime 404, baseline service presence and quota termination zero. Group deletion waits for no live links, then returns API 204/404. The pre-existing Blueprint remains. [Runtime/setup/cleanup driver](native-and-API-driver.py.txt), [capture driver](native-capture-driver.py.txt). Temporary forwarding servers and copied configurations are removed. The ongoing hunt keeps only its isolated session.
