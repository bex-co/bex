# w5/m108 isolated live verification

Executed 2026-10-02 against dev-5 (API :54050, dashboard :50050), disposable service `srv-davjlq9jg4r0n61fs7rg` / App `tea-davjlmhjg4r0n61fs7qg-w5-m108-cron`. The final acceptance cycle ran 05:11–05:14 UTC.

| Check | Observed result |
| --- | --- |
| Manual trigger and cancellation | Run `crr-rpvbs0f3lvo43gb7r34s` reached Running. DELETE current run returned **204 with an empty body** at 05:11:12 UTC. Its Job `…-run-62390693` disappeared and history reported Canceled. |
| More than ten later executions | **12 real Kubernetes Jobs** each completed exactly one Pod with a distinct `W5_M108_ACCEL_NN` log marker. Their history evicted the canceled manual entry by 05:12:40 UTC. |
| Reused cancellation slot | A later CronJob-owned Job was canceled at 05:12:52 UTC (204). `spec.cancelRun` changed to `…-c2-later-cancel`; the original manual Job remained absent. |
| Persistent intent acknowledgement | `spec.runAt` and `status.manualRunHandledAt` both remained `2026-10-02T05:11:07.707529Z` through eviction, later cancellation, fresh reconciler process and suspend/resume. |
| Fresh reconciler | New process first reconciled at 05:13:10 UTC. The old manual Job remained absent with ten retained history entries. |
| Suspend/resume | Waited for the controller to observe **App Hibernated AND CronJob suspend=true**, recorded at 05:13:12 UTC. Then waited for **App Running AND CronJob suspend=false**, recorded at 05:13:14 UTC. The old Job remained absent. |
| Fresh manual execution | New trigger `crr-lstnnias7f57hpk4gm7g` produced Job `…-run-5fb26acf`, one Job UID and exactly one successful Pod, logging `W5_M108_FRESH`. Completed at 05:13:26 UTC. |

An observer collected 104 changed snapshots across both setup/verification cycles. Machine assertions confirm that the final canceled manual Job never reappeared after cancellation, the saved trigger remained unchanged through lifecycle checks, retained history stayed bounded at ten entries, and the final fresh Job had exactly one UID and one successful Pod.

REST, GraphQL and MCP run IDs/statuses were compared as maps and matched for both the canceled and final histories. The browser initially displayed Canceled for the manual run. At completion its first page showed the expected five rows: latest Succeeded, later Canceled, then three Succeeded, with no Running row. The APIs retained ten entries; the dashboard page size is five.

## Scope and limits

- The real dev-5 API, store, App CR, CronJob, Job and Pod lifecycle paths were used. No production or other dev stack was modified.
- History was accelerated by constructing sequential real Jobs from the fixture CronJob's template, preserving its owner reference and labels but replacing each command with a short marker. These were **not twelve natural wall-clock schedule ticks**. Every actual Pod completion and log was checked before creating the next Job; status history was never fabricated or patched.
- The cron schedule was yearly to prevent unrelated natural ticks during this controlled experiment. The temporary reconciler polled only the fixture App. Restart proof covers a fresh reconciler process and persisted state, not the shared manager's informer/watch delivery.
- The first cycle exposed an evidence gap: suspend/resume calls completed before reconciliation observed suspension. The complete cancellation/12-run sequence was repeated and the final acceptance explicitly waited for both controller and CronJob suspension/resumption. First-cycle artifacts are preserved separately and do not substitute for that proof.
- This verifies Bex's bounded replay guard. It does not claim a Render history-retention contract or exact-once execution under arbitrary external deletion. Legacy-state/conflict/crash boundaries are addressed by repository regressions.

## Cleanup

Deleted the service through REST, both the explicit fixture workspace (`tea-davjlmhjg4r0n61fs7qg`) and the identity's auto-created workspace (`tea-davjlmhjg4r0n61fs7pg`) through GraphQL, and the disposable Kratos identity. Kubernetes reported both namespaces absent. Scoped reconciler and observer stop files were written and private auth-state credentials removed. No isolated secret store was required for this fixture.
