# m139 lifecycle acceptance — 2026-10-29 Pacific / 2026-10-30 UTC

The final drill passed in **422.31 seconds** using the actual backend services, a real dev-8 Postgres, real Kubernetes App/Deployment/Pod objects, a namespace-scoped current App reconciler, real kubelet/cAdvisor data evaluated by Prometheus 3.14.0, and the dev-8 Mailpit SMTP relay. No production configuration or clock changed.

- Two Ready Free services each accrued **75 seconds during the same 75-second window**. Their shared workspace therefore accrued 150 seconds. The Starter control stayed Ready.
- After actual operator auto-hibernation, process exit and the normal cAdvisor stop tail, the second Free service accrued **zero seconds** during the tested window.
- A **0.05-hour / 180-second** allowance warned at **144 seconds**, and **182 seconds** suspended both Free services. Read projections identify the billing suspension; manual resume is refused. Upgrading one suspended service to Starter immediately created a Ready running pod.
- Advancing only the test service clock to the next UTC month cleared the other service's budget suspension, returned the accounting total to zero, and preserved the owner's independent suspension. The already-idle service becomes resumable without overriding ordinary auto-hibernation.
- Two sweeps per phase produced exactly one warning and one exhaustion email, with two durable `(workspace, month, stage)` claims. [Mailpit delivery](mailpit-delivery.json) records actual SMTP delivery; it is not merely a database claim.
- [Production REST, GraphQL and MCP](production-readouts.json) independently returned **749.2833333333333 used / 750 included** for bex-canary. The prior hosted check in `done/t008.md` retains the production meter/Usage-card evidence; the current run did not repeat a browser rendering check.

The threshold drill found and fixed a real boundary defect: the previous hours comparison put the 80% threshold of a 0.05-hour allowance at `0.04000000000000001`, above the exact 144-second usage. Comparisons now use seconds. `TestSweepFractionalBudgetThresholds` verifies 143/144/179/180 seconds and repeated-sweep dedupe. Running that new test with the prior production source through a Go overlay failed at 144 seconds (`notices=[]`); it passes with the fix. The default production allowance stays 750 hours.

## Reproduction and bounds

[The exact acceptance test](lifecycle-test.go.txt) runs from the backend module after copying it to `internal/usage/m139_temp_acceptance_test.go`:

```sh
GOWORK=off go test -v -tags=integration ./internal/usage \
  -run '^TestM139LiveAcceptance$' -count=1 -timeout=20m
```

It requires the dev-8 app DSN in `BEX_ACC_DB`, dev-8 `KUBECONFIG`, Mailpit on 53080/60080, and [the reconciler driver](namespace-scoped-reconciler.go.txt) built inside the operator module as `/tmp/m139-reconciler`. The test refuses a DSN outside `localhost:55080`. Both operator clients use `client.NewNamespacedClient` for the freshly minted fixture namespace. [Prometheus configuration](prometheus.yml.txt) scrapes actual kubelet cAdvisor through a loopback Kubernetes API proxy. The timing checks call the production `usage.queryInstanceSeconds`; warning/exhaustion totals are injected through the real `PGStore.UpsertUsageHourly` seam so the drill does not need an elapsed calendar month or a closed production billing hour. The owner email resolver supplies the isolated fixture mailbox; delivery uses the real notifications composer and SMTP transport.

The shared local manager was already unavailable on a stale image, and shared Loki/Prometheus workloads were Pending. The supported `DEV_ENV_OBSERVABILITY=0` option brought dev-8 auth/DB/Mailpit up; the scoped reconciler and loopback Prometheus supplied only this drill's missing components. Shared deployments were left untouched. Earlier attempts exposed an unsuitable stock-nginx fixture and a harness stop-tail/cleanup-order problem, all corrected before the passing run. Local controller/etcd leadership delays prolonged graceful deletion; no cluster rebuild or forced fixture deletion was used.

## Validation and cleanup

- [Passing full lifecycle receipt](lifecycle-receipt.txt).
- [Full backend normal-suite receipt](backend-suite-receipt.txt): every package passed on the current tree. The earlier run overlapped unrelated source changes; the root worker confirmed those failures no longer reproduce. The root ship workflow runs the required fresh real-dependency backend gate.
- Targeted apps/freehours/usage tests and the real Prometheus expression test passed.
- [Cleanup receipt](cleanup.json): all four final-run App GETs returned 404; all three attempted workspaces/namespaces were deleted; no owned tenant, app, usage or notice row remains. Both owned Mailpit messages were deleted by exact ID and the mailbox search returned zero. The production QA login was revoked successfully and its own cookie/state files removed.

The ordinary production 750-hour exhaustion/calendar rollover remains supplemental observation, as specified by the dated unblock decision. It does not replace or gate the completed isolated enforcement drill. D5 (no retroactive announcement) is unchanged.
