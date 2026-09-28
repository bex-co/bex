# w7 · m155 — Parse mixed application log formats without error floods

**Worker:** worker7 **Goal:** Preserve structured log levels and every supported line format without expected parse mismatches flooding Alloy errors. **Status:** done

**Estimate:** 90m implementation; 190m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce mixed formats and correct parser selection — **DONE** | 45m | — |
| t002 | Verify output and parser diagnostics with real Alloy — **DONE** | 45m | t001 |
| t003 | Render parity — **DONE** | 30m | t002 |
| t004 | Simplify — **DONE** | 20m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
| t006 | Closeout — **DONE** | 10m | t004, t005 |

## Definition of done

- [x] JSON and logfmt retain explicit severity; plain text stays unknown; all messages survive and supported mixed-format input emits no decode-error flood.
- [x] Assertions cover output counts, message preservation, severity and absence of expected-format error spam; error/warning filters still find the appropriate records.

- [x] All required closing tasks are complete; production-dependent claims have dated runtime evidence, not just green unit tests.

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

## Evidence (2026-09-28 UTC, worker7)

**Revalidated:** production Alloy logged `failed to decode logfmt … unexpected '"'` from `loki.process.app_logs` (38 in the hour before 06:30Z); `stage.logfmt` ran on every line, so JSON and quoted plain text failed to decode. Log loss was not observed: the real-Alloy harness shows every such line still emitted.

**Fix (t001), shipped `d88b3f28a`:** `stage.logfmt` now runs inside `stage.match` gated to non-JSON lines carrying a top-level `level=`/`severity=` token. JSON severity, logfmt severity, normalization and the `unknown` bucket are unchanged.

**Tests (t003/t005):** `scripts/test_log_shipper.py` runs the Helm-rendered production pipeline in the chart's pinned Alloy image, now with the production app label set, and adds quoted plain text, escaped-quote JSON, truncated JSON, indented JSON and escaped-quote logfmt cases. It asserts every line survives with the expected level **and** zero `app_logs` warn/error diagnostics. Pre-fix pipeline: levels correct but the diagnostics assertion fails with the production error text. `gitops-validate.sh` passes.

**Production (t003):** the gated config reloaded at 07:11:00Z. `failed to decode logfmt`: **190** in 05:11–07:11Z, **0** from 07:11Z to 07:39Z (spanning a second reload at 07:22–07:23Z), and no other Alloy errors. JSON-levelled tenant lines keep their labels (Loki, 24h before: error 5, warn 5 from JSON). Production had **no** tenant logfmt lines with a severity key in the prior 24h, so the logfmt path is proven by the harness, not by live traffic.

**Render parity (t002):** the `level` label vocabulary and every log-filter surface (REST `level=`, GraphQL, MCP, dashboard, pinned CLI `--level`) are unchanged; only which lines reach the logfmt decoder changed. No ADR018 drift.

**Simplify (t004):** inline review; one gate, no new stages.
