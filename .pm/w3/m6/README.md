# w3 · m6 — Platform alerting: Alertmanager + rules for bex's own health

**Worker:** worker3 **Goal:** Nobody gets paged when bex itself breaks: the Prometheus chart runs server-only (`alertmanager: { enabled: false }`, no exporters), and the nightly etcd/OpenBao backups have no staleness watch — a silently failing backup is data loss discovered at restore time. Enable Alertmanager with one real notification channel and a small, high-signal rule pack over platform and bex-specific invariants. **Status:** todo

## Tasks (in order)

| id   | title                                                                                       | est | depends_on   |
| ---- | -------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Enable Alertmanager in the Prometheus chart + one receiver (webhook/email), secret out-of-band | 30m | —            |
| t002 | Enable kube-state-metrics (minimal exporter scope)                                            | 25m | —            |
| t003 | Rule pack — platform: CrashLoop/not-Ready in platform namespaces, node NotReady, PVC near-full, cert expiry | 30m | t001, t002   |
| t004 | Rule pack — bex: backup CronJob last-success age > 26h, OpenBao sealed, bex-api down, Traefik 5xx spike | 30m | t003         |
| t005 | Acceptance: synthetically break two rules → notifications arrive; recovery resolves          | 25m | t004         |
| t006 | Simplify — `/simplify` over what this milestone changed                                       | 20m | t005         |
| t007 | Test coverage — promtool rule tests (else close n/a)                                          | 20m | t005         |
| t008 | Closeout — DoD met → move milestone to `done/`                                                | 10m | t007         |

## Definition of done

Breaking a monitored invariant (e.g. a backup CronJob suspended past its window, a bex-system Deployment rolled to a bad image) produces a notification on the configured channel within one evaluation window, and recovery resolves it; rules are declarative in gitops; `kustomize build` of both overlays stays green; no secret value in git.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-07-09; `deploy/gitops/base/prometheus.yaml` ("Server-only (no alertmanager/pushgateway/exporters)"); the unmonitored backup CronJobs from w1/m1 (etcd) and w1/m7/t006 (OpenBao); docs/secrets.md's sealed-on-restart failure mode.
- **Goal linkage:** GOAL.md #2 ("Basic obs for operation") — the operator half; continuous complement to w1/m7's point-in-time posture checks.
- **Expected outcome:** platform failures (and silent backup rot) page a human instead of waiting to be noticed; the OpenBao-sealed state that 503s the env-vars API is alerted, not discovered by tenants.
- **Why now:** prod now carries real tenant state (OpenBao credentials; enforced authz queued in w1/m9) with zero paging; every day of silent-failure risk on the backup jobs is unrecoverable-data risk.
- **Render parity closing task: omitted** — pure platform infra; no REST/GraphQL/MCP/UI surface changes.
