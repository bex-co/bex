# Existing service-host references skip target type and addressability

Why: a Blueprint can validate an internal host to a static site with no internal listener, or silently accept a reference whose declared type disagrees with the actual existing target.

**Severity:** minor. **Owner:** shared server Blueprint validation/apply. Source: functional CLI QA w8/sweep316,2026-10-15 UTC. [Evidence](evidence/cli-sweep316/README.md). Source d5220a952; serving revision unknown. Correct same-file cases and current existing-web/missing-name controls distinguish this from a client wire or deploy-lag diagnosis.

## Native reproduction

With fresh exact-owned Free static and web targets in the selected workspace, run:

```sh
bex blueprints validate .pm/w8/m74/evidence/cli-sweep316/existing-static-host-root.yaml --confirm -o json
bex blueprints validate .pm/w8/m74/evidence/cli-sweep316/existing-web-wrong-type-nested.yaml --confirm -o json
```

The archived files reference the now-deleted fixture names; replace only those names with newly proven-owned targets of the same kinds before replay. Use the isolated unchanged Render pin against Bex too. Existing static typed web/property host must be refused because static has no internal listener. Existing web typed worker/property host must be refused at fromService.type, naming supplied and expected types. Both currently exit0/valid:true. Root and project-environment layouts reproduce through all three clients:12 false accepts,12 passing controls. Four native wire captures add two false accepts/two controls and prove server ownership:28 commands total.

Equivalent same-file declarations refuse correctly. An existing web target with type web stays valid; an absent host still refuses. Two exact-owned fixtures reach Live/HTTP200 and are cleaned. Current-source full validator reproduces all four incorrect cases. Actual DeployStack with a fake K8s client also accepts both invalid root documents and creates the consumer with the synthetic target host, while the valid web control succeeds. This is source execution only: no hosted Blueprint apply, connection failure or production DNS lookup is claimed.

## Cause and correction

- `deploy.go:2250` allows only property host to defer an undeclared service lookup; the same-file type/address guards at2257–2269 use stackIndex.serviceTypes/servicePorts. Out-of-file hostport/port remain deliberately unsupported and must stay refused.
- `classifyRef:2278` stores hostRef key/target/port/hostport but loses the supplied fromService.type (and the ordinal tracked by [the314 origin finding](110.md)). `unknownServiceReferences`, `blueprint_ownership.go:652–663`, checks only whether the name is declared or exists in the workspace map; it never reads existing.Spec.Type. Validation at637 and apply at deploy.go:781 both trust that existence result.
- Carry supplied type and the authored reference occurrence into the structured reference problem flow planned for the origin repair. For an undeclared host target, use the resolved same-workspace App's effectiveType and the existing serviceRefTypes spelling map, and require web/private addressability. Preserve declared target precedence, normal valid existing web/private hosts, missing-name behavior and sorted/deterministic refusal order. Emit the mismatch at fromService.type and the no-address error at the actual caller reference. Both validation and apply must share these checks before any group/datastore/service write.
- `workspace_snapshot.go:93` already provides resolved workspace Apps, with manifest-facing name, namespace/twin handling and duplicate-name checks. Reuse that map; do not introduce a second query, cross-workspace lookup or name/display policy. `resolveServiceRefs:1085` and hostRef.env:601 must receive only admitted host references. Keep database/Key Value references and same-file envVarKey copies unchanged.
- Census and consumers are the same as the origin repair: unknownServiceReferences has validation/apply callers and direct126 tests; parse/compile feeds validate/preview/deploy/create/sync, generated-manifest checking and disconnect cleanup. REST, GraphQL errorDetails, MCP validate/preview/deploy and dashboard validation share the core. The exact native wire and HTTP200/valid:false versus apply400 contracts stay intact; no CLI fork or request repair.
- ADR041 D1–D3 limits internal addresses and slug Services to web/private;081 already enforces this for declared targets.096 enforces the supplied type on declared services. This work extends those invariants to actual existing workspace targets using the same type spelling/default rules.

## Dedupe, acceptance and limits

Searches across all open/blocked/done board items, open milestones, DO_NOT_DO, the CLI ledger and targeted history find no existing target-metadata owner. Closed081/096 implement declared-target checks, which still pass;206 refuses absent targets,051 aggregates misses and126 handles per-kind names. Their33 focused named tests/9 top-level tests pass, but complete historical live/apply DoDs were not replayed or reopened. The [unshipped110 origin defect](110.md) touches the same hostRef/unknownServiceReferences data path, so110 is promoted here instead of scheduling overlapping parallel rewrites.

Future source and native acceptance must cover actual target kinds web/private/worker/cron/static, matching/wrong reference spellings, default web type, root/nested origins, correct existing web/private, missing targets, same-file guards and declared-vs-existing precedence. Use source fixtures for paid families; the current hunt authorizes no paid resources or new cron. Assert zero writes for every invalid apply and preserve all original314/081/096/206/051/100/126 controls. Record unexercised runtime/surface behavior. Both future native clients must refuse the exact invalid documents after deployment, with correct origins, before closing.

No real existing worker/cron/private fixture, hosted apply, consumer DNS/connection, full module suite, browser or exact live GraphQL/MCP execution is claimed. Current evidence establishes server admission and fake-client apply behavior; the repair remains scheduled, not implemented.
