# Ordinary environment conflicts mislocate validation errors — sweep208

Twenty-four native processes use published Bex 0.3.2 and unmodified Render 2.27 at a764810a7682 against production, own workspace tea-daif693dqjvc73e7as3g and the same isolated human grant. Every validity and exit decision is correct. Sixteen refused value/generation conflicts report unrelated service fields or nonexistent group fields. Four ordinary-key refusals and four accepted empty-literal controls retain correct decisions. This extends existing w8/100; it is not a second issue.

Each bad entry declares key IMAGE, PLAN, TYPE or CONNECTIONPOOL with value:"" and generateValue:true. Services should locate services[0].envVars[0] at 12:9. IMAGE instead selects image at 8:16; PLAN selects plan at 7:15; TYPE selects type at 5:15; CONNECTIONPOOL invents services[0].connectionPool at 3:5. Groups should locate envVarGroups[0].envVars[0] at 6:9; every corresponding group image/plan/type/connectionPool path is nonexistent and falls back to3:5. ORDINARY_VALUE controls locate the broader env collection or group declaration; they do not select the unrelated fields. No entry is applied or generated.

Four exact native captures cover Bex service-IMAGE, Render group-PLAN, Bex ordinary-key refusal and Render literal-IMAGE acceptance. All POST /v1/blueprints/validate multipart bytes and own ownerId are forwarded unchanged; HTTP200 responses and decoded native validity/errors agree. Authorization is omitted. The API supplies the wrong location; both clients faithfully display it.

Eight actual current ValidateBlueprint diagnostics independently reproduce all incorrect paths, and prove that the intended authored env entry has a known location. Source HEAD before/after is cb16919ab75d1a72663109b06707fb3663fc33cb. This private virtual Go overlay expects the present bug; it is diagnostic evidence, not a passing corrected regression. The 12 existing reference-location cases pass. A separate correctly named static-neighbor test passes its10 cases; the first batch's mistyped static name matched nothing. No full suite is claimed.

Current deploy.go:2486 wraps validateKeyedEnv without envVars["KEY"] context. Group errors at 2206/2208/2211 likewise use var "KEY". blueprint.go:763 guesses a field from message keywords; indexed context at 764 and 822 would take precedence if supplied. The new envRefError helper at 1767 addresses classifyRefs only. Thus main's reference fix dd9a7ada9 leaves these generic keyed/group branches broken. Carry the known caller key through the bounded emitters, keep refusal decisions intact and audit their neighboring group errors. Avoid extracting a missing target key from arbitrary error text. No launcher, schema, envelope or operator change is needed.

Source identity at native start was not captured: validations began before scoped stash recovery completed. The later local diagnostics explicitly verify their HEAD. The recovery retained both board intents and all owned untracked evidence. Deployed API SHA remains unknown, so no deployment identity is inferred. Hosted apply/generation/runtime and GraphQL/MCP/dashboard are untested. All capture processes/servers/config copies are removed. No hosting resource or paid feature is created; quota remains six baseline services, no terminating services and no datastores. This QA work does not implement or ship fixes.

Replay each harmless fixture with the isolated QA config:

```sh
bex blueprints validate .pm/w8/evidence/100/value-conflicts-sweep208/service-IMAGE.yaml --confirm -o json
render blueprints validate .pm/w8/evidence/100/value-conflicts-sweep208/group-PLAN.yaml --confirm -o json
```

The pinned Render process must use its separate private config pointed at Bex with equivalent own-workspace authentication. The source probe is supplied as a .txt artifact for a private Go overlay; set BEX_QA_LOCATION208_INPUT to location-input.json and run TestQAEnvConflictLocation208 against the recorded source. Do not copy diagnostic expectations into fixed regression tests unchanged.

Evidence:

- [control-IMAGE.yaml](control-IMAGE.yaml)
- [control-TYPE.yaml](control-TYPE.yaml)
- [group-CONNECTIONPOOL.yaml](group-CONNECTIONPOOL.yaml)
- [group-IMAGE.yaml](group-IMAGE.yaml)
- [group-ORDINARY_VALUE.yaml](group-ORDINARY_VALUE.yaml)
- [group-PLAN.yaml](group-PLAN.yaml)
- [group-TYPE.yaml](group-TYPE.yaml)
- [location-input.json](location-input.json)
- [location-probe.go.txt](location-probe.go.txt)
- [location-results.json](location-results.json)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-capture-driver.py.txt](native-capture-driver.py.txt)
- [native-matrix.json](native-matrix.json)
- [native-wire.json](native-wire.json)
- [service-CONNECTIONPOOL.yaml](service-CONNECTIONPOOL.yaml)
- [service-IMAGE.yaml](service-IMAGE.yaml)
- [service-ORDINARY_VALUE.yaml](service-ORDINARY_VALUE.yaml)
- [service-PLAN.yaml](service-PLAN.yaml)
- [service-TYPE.yaml](service-TYPE.yaml)
- [source.txt](source.txt)
- [static-neighbor-results.json](static-neighbor-results.json)
