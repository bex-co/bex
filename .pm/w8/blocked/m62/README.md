# w8 · m62 — Reject unlaunchable environment values

**Worker:** worker8 **Goal:** Reject environment values that cannot reach an application process before reporting validation or save success. **Status:** blocked — t001/t002/t004/t005 done and shipped; t003 native/surface acceptance and t006 need the fix deployed to production plus fresh native Bex/Render receipts.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Validate new environment values and Blueprint literals — **DONE 2026-10-12** | 35m | — |
| t002 | Cover shared writers and preserve repair paths — **DONE 2026-10-12** | 35m | t001 |
| t003 | Render parity — **BLOCKED: needs deployed fix + native receipts** | 20m | t002 |
| t004 | Simplify — **DONE 2026-10-12** | 15m | t003 |
| t005 | Test coverage — **DONE 2026-10-12** | 35m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

- Repeat the [finding](finding.md) through current/published Bex and the unmodified Render pin against Bex. All three NUL documents return the existing HTTP 200 validation envelope with `valid:false`, field errors and native exit1. All newline controls remain valid/exit0.
- A common environment-value guard rejects newly supplied NUL values before persistence, seed application, group rollout or deploy scheduling, across the enumerated writers. Report the variable key and field location without printing its value.
- NUL at the start, middle, end or as the entire value is rejected. Empty strings, Unicode, newlines, tabs, CRLF and literal backslash-u0000 text retain their established behavior. Binary file content remains byte-exact.
- Replacement/deletion of an already-stored invalid value remains possible; an unrelated patch is not stranded by validating every legacy map entry indiscriminately. Rejected new values cause no partial writes or deploys.
- Run meaningful shared-service and cross-surface tests, native validation controls, and the actual generated-loader/OCI controls. Record any hosted apply or API path still unverified explicitly. Do not claim a production runtime repro from local Docker evidence.
- All tasks and observable checks are complete before moving the milestone to `done/`.

## Progress / block

- **2026-10-12:** one rule (`core.ValidEnvValue`/`CheckEnvValue`, code `ENVIRONMENT_VARIABLE_VALUE_INVALID`) refuses newly supplied NUL environment values on every service, Environment Group, create and Blueprint writer before any effect; Blueprint validation reports `valid:false` with the entry location. Legacy values stay replaceable/deletable; secret-file bytes are untouched. ADR004 documents the rule. Backend `go test ./...` passes.
- **Blocked:** t003 and t006 need the fix deployed to production, then the three finding documents and newline controls through released Bex and pinned Render (exit1 vs exit0), REST/GraphQL/MCP/dashboard receipts, and cleanup.

## Source + Goal linkage

- **Source:** user-requested continuous `$qa-find-bugs-cli` in w8, sweep171, 2026-10-12 UTC. [Finding](finding.md) and [sanitized evidence](../../evidence/m62/README.md).
- **Goal linkage:** ADR008 core hosting, ADR004 deploy configuration and ADR018 Render compatibility. Validation should catch configuration that cannot start a process.
- **Expected outcome:** invalid environment values receive useful feedback at validation/save time; accepted representable values and binary files retain their bytes.
- **Why now:** both native clients accept all three invalid forms. The actual parser and shared patch preserve them, while local runtime refuses them and the actual generated loader changes them.
- **Render parity included:** environment values are exposed through REST, GraphQL, MCP and the dashboard. Compare consistent shared behavior and existing error envelopes. The process-value constraint comes from POSIX and observed consumers; Render production has not been tested.
- **Sizing:** 155 minutes across six tasks, including shared writer verification and standing closeout tasks. Filing only; no fix or ship performed by the hunt.
