# w8 · m62 — Reject unlaunchable environment values

**Worker:** worker8 **Goal:** Reject environment values that cannot reach an application process before reporting validation or save success. **Status:** blocked — t001/t002/t004/t005 done; sweep295 native validation and focused source checks pass; t003/t006 retain REST bulk route109 and dashboard acceptance; the prior normal-save error remains unassigned and is not reproduced by298.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Validate new environment values and Blueprint literals — **DONE 2026-10-12** | 35m | — |
| t002 | Cover shared writers and preserve repair paths — **DONE 2026-10-12** | 35m | t001 |
| t003 | Render parity — **BLOCKED: native gate passes; remaining live writer/surface receipts** | 20m | t002 |
| t004 | Simplify — **DONE 2026-10-12** | 15m | t003 |
| t005 | Test coverage — **DONE 2026-10-12** | 35m | t003 |
| t006 | Closeout — **BLOCKED: remaining surface acceptance** | 15m | t004, t005 |

## Definition of done

- Repeat the [finding](finding.md) through current/published Bex and the unmodified Render pin against Bex. All three NUL documents return the existing HTTP 200 validation envelope with `valid:false`, field errors and native exit1. All newline controls remain valid/exit0.
- A common environment-value guard rejects newly supplied NUL values before persistence, seed application, group rollout or deploy scheduling, across the enumerated writers. Report the variable key and field location without printing its value.
- NUL at the start, middle, end or as the entire value is rejected. Empty strings, Unicode, newlines, tabs, CRLF and literal backslash-u0000 text retain their established behavior. Binary file content remains byte-exact.
- Replacement/deletion of an already-stored invalid value remains possible; an unrelated patch is not stranded by validating every legacy map entry indiscriminately. Rejected new values cause no partial writes or deploys.
- Run meaningful shared-service and cross-surface tests, native validation controls, and the actual generated-loader/OCI controls. Record any hosted apply or API path still unverified explicitly. Do not claim a production runtime repro from local Docker evidence.
- All tasks and observable checks are complete before moving the milestone to `done/`.

## Progress / block

- **2026-10-12:** one rule (`core.ValidEnvValue`/`CheckEnvValue`, code `ENVIRONMENT_VARIABLE_VALUE_INVALID`) refuses newly supplied NUL environment values on every service, Environment Group, create and Blueprint writer before any effect; Blueprint validation reports `valid:false` with the entry location. Legacy values stay replaceable/deletable; secret-file bytes are untouched. ADR004 documents the rule. Backend `go test ./...` passes.
- **Remaining block:** native validation now passes in sweep295. t003/t006 retain live direct-writer and GraphQL/MCP/dashboard feedback receipts, with exact deployed artifacts unobserved. The common source guard and prior implementation checks remain recorded.

## Source + Goal linkage

- **Source:** user-requested continuous `$qa-find-bugs-cli` in w8, sweep171, 2026-10-12 UTC. [Finding](finding.md) and [sanitized evidence](../../evidence/m62/README.md).
- **Goal linkage:** ADR008 core hosting, ADR004 deploy configuration and ADR018 Render compatibility. Validation should catch configuration that cannot start a process.
- **Expected outcome:** invalid environment values receive useful feedback at validation/save time; accepted representable values and binary files retain their bytes.
- **Why now:** both native clients accept all three invalid forms. The actual parser and shared patch preserve them, while local runtime refuses them and the actual generated loader changes them.
- **Render parity included:** environment values are exposed through REST, GraphQL, MCP and the dashboard. Compare consistent shared behavior and existing error envelopes. The process-value constraint comes from POSIX and observed consumers; Render production has not been tested.
- **Sizing:** 155 minutes across six tasks, including shared writer verification and standing closeout tasks. Filing only; no fix or ship performed by the hunt.

## Native validator acceptance — sweep295

[72 fresh native receipts](../../evidence/m62/native-validation-sweep295/README.md) now pass across installed/published/actual-current Bex and the unchanged pin:28 invalid NUL documents return valid:false/located key-level errors/exit1,44 representable controls return valid:true/exit0. Original literal/seed/group shapes and all four NUL positions are covered; marked invalid values are not echoed. Existing focused checks pass34 named events across10 top-level tests, including legacy repair and binary-file controls. No production resource/env mutation; final services6/terminating0, copied config/authored documents removed. The native validator gate passes. Keep the milestone blocked on remaining live direct-writer and non-REST/UI surface receipts; no complete deployed-writer/runtime or full real-DB suite claim.

## Live validator transport parity — sweep296

[36 GraphQL/MCP API validation calls](../../evidence/m62/validation-adapters-sweep296/README.md) pass:14 invalid documents and22 representable controls match the actual native validator's error text/path/line/column exactly after the fresh authored name substitution. Both adapters use the shared validator and return HTTP200/valid:false for invalid documents; GraphQL string/detail errors agree and MCP returns structured validation results. No resource/env mutation. Keep the remaining direct-writer live and dashboard feedback gates; validator-only parity does not certify ordinary environment writes.

## Direct writer and hosted binary-file checks — sweep297

[Complete receipts](../../evidence/109/README.md#environment-value-and-binary-file-controls) add31 writer passes:16 stable NUL refusals and15 same-value valid controls across all service writer/adapter combinations and the exercised group siblings. The service's actual native build/runtime and later pin --wait deployment preserve its intended normalized environment and the exact20-byte NUL-containing file. Positive legacy saves legitimately add rollouts; their aggregate count is not used as per-invalid side-effect proof.

Full acceptance remains blocked: the group's REST bulk route is unreachable before value admission ([109](../../109.md)); a later normal MCP single control returns ENV_GROUP_RESTORATION_FAILED and busy metadata, with cause unobserved. Two group MCP sparse cases and dashboard feedback remain untested. Do not assign the save failure to m62 or the route without evidence. Both exact-owned fixtures/eight child deployments are fully cleaned through normal APIs; baseline six/groups and quota6/terminating0 retained. The raw native trailing-CRLF expectation was a QA fixture error corrected to the pin's documented outer trimming without a mutation; the API group baseline retains CRLF. No new value-admission issue or product fix.

## Isolated MCP writer follow-up — sweep298

[Seven fresh direct MCP cases](../../evidence/m62/group-MCP-sweep298/README.md) all pass: five positive same/different/Unicode-CRLF/empty/sparse controls and two single/sparse NUL refusals. Both refusals retain the prior value, revision and updatedAt and insert no staged entry. The two previously unexercised MCP sparse cases are covered. The original normal-save restoration error does not recur; its cause remains unconfirmed and no fix is inferred. The fresh unlinked group is fully cleaned through normal APIs, with baseline inventory retained.

Direct-writer passing controls now total38 across297–298. Keep [109](../../109.md)'s REST bulk reachability and dashboard feedback as remaining acceptance gates; do not close this milestone from the validator and writer subsets alone.
