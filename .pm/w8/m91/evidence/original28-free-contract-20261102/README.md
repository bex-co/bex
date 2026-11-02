# Original28 acceptance runner: current Free budget contract

The original28 runner now binds the same actual workspace identity used by the accepted m83 run and checks the canonical current-month instance-hours allowance. Owner, usage and limits must each have a successful response captured within 300 seconds; at least10 hours and available service capacity remain mandatory. Aggregate meter coverage is retained as a diagnostic. All original programs, commands, data, readiness/adoption and once/cleanup rules remain unchanged.

Root reviewed the three Free-contract diffs and the later optional-coverage correction. The original success-capture method raises `KeyError('coverage')` when a legal response omits that optional field; the correction captures null while keeping the budget fields strict. Full70 host controls pass in13.069 seconds. The exact old-method failure and initial test-harness failures remain retained. These are local runner controls, not fresh QA usage, application or hosted acceptance results.

The [complete source and custody](capsule/RECEIPT.json), [full70 log](capsule/full-host.stderr), [source manifest](capsule/source/MANIFEST.json) and [current evidence-to-task map](capsule/EVIDENCE-TO-TASK-MAP.json) preserve the earlier source preparation. All actual adoption, Ready, Free and approval fields remain null. No sign-in, deployment or cluster mutation occurred.

[ADR060](../../../../../docs/ADR060-build-worker-reliability-and-performance.md#d7-addendum--shared-native-utility-versions-and-limits-w8m91-2026-11-01) records the selected utility versions and exercised limits. The per-tool milestones still require the common qualification/adoption and their unchanged original both-client hosted checks.
