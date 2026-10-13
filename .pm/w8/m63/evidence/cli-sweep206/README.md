# Completed initial hook selected again — CLI sweep206

Eight native production Blueprint validations confirm initial-only, initial+regular, empty and omitted hook declarations are admitted in both clients. Two exact unchanged multipart captures return HTTP200/valid:true. Admission is correct and is not this bug. No Blueprint is applied to production.

Four actual native deploys-create processes run against current backend handlers over loopback. The handlers share real public DeployStack initial creation and real deployment Trigger, with a fake Kubernetes client and DeployStore. First Live rev-1 and successful predeploy generation1 are simulated. Without a Blueprint re-sync, both native clients request generation2 and retain echo qa206-initial-once with no ran marker. Re-sync controls mark ran and retain echo qa206-regular. Native HTTP requests/replies and process stdout/stderr are complete; local dummy environment credential is not recorded in HTTP headers. Local fake IDs are not production ownership proofs or deletion targets.

The actual current operator prepareAppReleaseDecision and reconcilePreDeploy then consume those post-native Apps. Fake-client API metadata generation and server-assigned UID are modeled explicitly because the fake client does not supply them. The operator creates generation2 Jobs in its fake client: two use the completed initial command; two re-sync controls use the regular command. No Job/container is executed. All five existing initial-hook tests pass; they cover re-sync marking and already-marked states, not a native manual deploy before re-sync. Diagnostic assertions expect current incorrect selections, not a future fixed regression. No full platform suite is claimed.

Initial operator setup omitted generation increments, the preceding release-selection stage and UID. Those failed QA attempts are retained separately and excluded. The corrected final probe calls actual release selection, preserves standard API effects and exercises actual job creation. The Go JSON harness also had to concatenate output chunks for long native-wire records; that decoding issue is not a product finding.

No production hosting resource or paid feature is created. Native local children exit0, httptest servers close and Go temporary configs are removed by test completion; comparison/forwarding configs and servers are removed. Production quota remains six baseline services, zero terminating and no datastores. Source c5a0eb9dd15f6b417bf586fdf26510706497b032; deployed API/operator revisions unknown. Hosted hook execution, real first rollout, persistence and auth are untested. No implementation/commit/push/release/external issue.

Evidence:

- [empty-initial.yaml](empty-initial.yaml)
- [excluded-empty-fake-uid.json](excluded-empty-fake-uid.json)
- [excluded-fake-generation.json](excluded-fake-generation.json)
- [excluded-missing-release-selection.json](excluded-missing-release-selection.json)
- [initial-and-regular.yaml](initial-and-regular.yaml)
- [initial-only.yaml](initial-only.yaml)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-local-events.jsonl](native-local-events.jsonl)
- [native-local-overlay.json](native-local-overlay.json)
- [native-local-probe_test.go.txt](native-local-probe_test.go.txt)
- [native-local-results.json](native-local-results.json)
- [native-production-validation.json](native-production-validation.json)
- [native-production-wire.json](native-production-wire.json)
- [native-validation-capture-driver.py.txt](native-validation-capture-driver.py.txt)
- [omitted.yaml](omitted.yaml)
- [operator-events.jsonl](operator-events.jsonl)
- [operator-input.json](operator-input.json)
- [operator-overlay.json](operator-overlay.json)
- [operator-probe_test.go.txt](operator-probe_test.go.txt)
- [operator-results.json](operator-results.json)
- [source.txt](source.txt)

- [Source identity follow-up](source-identity-followup207.json) confirms relevant tree equivalence and records uncaptured per-process HEAD limits.
