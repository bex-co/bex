# w7 · m155 — Parse mixed application log formats without error floods

**Worker:** worker7 **Goal:** Preserve structured log levels and every supported line format without expected parse mismatches flooding Alloy errors. **Status:** todo

**Estimate:** 90m implementation; 190m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce mixed formats and correct parser selection | 45m | — |
| t002 | Verify output and parser diagnostics with real Alloy | 45m | t001 |
| t003 | Render parity | 30m | t002 |
| t004 | Simplify | 20m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 10m | t004, t005 |

## Definition of done

- [ ] JSON and logfmt retain explicit severity; plain text stays unknown; all messages survive and supported mixed-format input emits no decode-error flood.
- [ ] Assertions cover output counts, message preservation, severity and absence of expected-format error spam; error/warning filters still find the appropriate records.

- [ ] All required closing tasks are complete; production-dependent claims have dated runtime evidence, not just green unit tests.

## Source + Goal linkage

- **Source:** User-approved platform-log brainstorm, 2026-09-28 UTC (September 27 America/Denver); user requested all five in w7. Read-only collection at source revision 016391810: 342 container streams, 167 pods, 22 namespaces, plus Loki platform history. Requested prior 24 hours; rotation/deleted pods limit coverage. Raw local evidence: /tmp/bex-platform-log-audit/{manifest.json,pods.json,events.json,argo.json,loki-platform.jsonl}; temporary artifacts may expire, so the observed findings are preserved below and must be revalidated at execution.
- **Observed finding:** The audit counted 8,979 Alloy errors with msg="failed to decode logfmt" from loki.process.app_logs. deploy/gitops/base/log-shipper.yaml:224 unconditionally runs stage.logfmt after stage.json. Log loss was not established; verify output before making that claim.
- **Goal linkage:** ADR008 reliable self-hosted Render-compatible hosting, deterministic lifecycle state and platform operability. Preserve structured log levels and every supported line format without expected parse mismatches flooding Alloy errors.
- **Expected outcome:** Preserve structured log levels and every supported line format without expected parse mismatches flooding Alloy errors.
- **Why now:** Thousands of expected format mismatches obscure genuine ingestion failures and consume log capacity.
- **Existing work / deduplication:** Follow-up to w9/done/071.md, which shipped and live-verified logfmt severity extraction. Preserve that support and the warning vocabulary from w8/done/031.md rather than reopening them.
- **Render parity:** Included: this fix affects tenant-facing cron lifecycle or log-level filtering; validate the corresponding ADR018 row and all exposed adapters.
- **Scheduling:** approved priority 3 of five in w7. Infrastructure rollout verification follows m153's root-sync repair; isolated implementation can proceed independently. No dependency on blocked w7 datastore/sandbox walkthroughs is inferred.

## Scope and evidence limits

This is planned work, not an implemented fix. The read-only audit did not authorize implementation during collection. Revalidate the deployed revision and failure before executing these tasks. Raw logs and credentials stay out of Git; preserve only sanitized observations. Do not reopen the accepted PSL finding, duplicate the known blockeden.xyz certificate work, or infer root causes from transient etcd/storage warnings.
