# w3 · m6 — Platform alerting: Alertmanager + rules for bex's own health

**Worker:** worker3 **Goal:** Nobody gets paged when bex itself breaks: the Prometheus chart runs server-only (`alertmanager: { enabled: false }`, no exporters), and the nightly etcd/OpenBao backups have no staleness watch — a silently failing backup is data loss discovered at restore time. Enable Alertmanager with one real notification channel and a small, high-signal rule pack over platform and bex-specific invariants. **Status:** done

## Tasks (in order)

| id   | title                                                                                       | est | depends_on   |
| ---- | -------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Enable Alertmanager in the Prometheus chart + one receiver (webhook/email), secret out-of-band | 30m | — — **DONE** |
| t002 | Enable kube-state-metrics + explicit scrape jobs (KSM incl. cronjobs collector, kubelet /metrics, cert-manager :9402, OpenBao telemetry) | 40m | — — **DONE** |
| t003 | Rule pack — platform: CrashLoop/not-Ready in platform namespaces, node NotReady, PVC near-full, cert expiry | 30m | t001, t002 — **DONE** |
| t004 | Rule pack — bex: backup CronJob last-success age > 26h, OpenBao any-member sealed, bex-api replicas==0, Traefik 5xx spike, stranded node-local images | 30m | t003 — **DONE** |
| t005 | Acceptance: synthetically break two rules → notifications arrive; recovery resolves          | 25m | t004 — **DONE** |
| t006 | Simplify — `/simplify` over what this milestone changed                                       | 20m | t005 — **DONE** |
| t007 | Test coverage — promtool rule tests (else close n/a)                                          | 20m | t005 — **DONE** |
| t008 | Closeout — DoD met → move milestone to `done/`                                                | 10m | t007 — **DONE** |

## Outcome (2026-07-12)

Built against the 2026-07-11 re-materialized definition (rebased onto latest main; the docs `ADR###-` renames and the w1/m19 platform-pool invariant were absorbed).

- **t001 — Alertmanager + email receiver.** Single-replica AM (no persistence), pinned to the platform pool. Receiver is **email on the existing SendGrid relay** (`smtp.sendgrid.net:587` STARTTLS, `apikey` user) — no new channel cred to mint. The API key is out-of-band: `smtp_auth_password_file` off Secret `alertmanager-smtp` (key `smtp-password`, ns `monitoring`), never committed; `to`/`from`/smarthost are non-secret and in-repo.
- **t002 — KSM + explicit scrape jobs.** kube-state-metrics on (platform-pool pinned); added scrape jobs (the inline config replaces the chart default, nothing auto-discovered): `kube-state-metrics`, `kubernetes-kubelet` (`kubelet_volume_stats_*` only), `cert-manager` (`:9402`), and `openbao` (per-pod `/v1/sys/metrics`). Still no pushgateway/node-exporter.
- **t003 — platform rules.** CrashLoop, Deployment-not-ready, PVC>85%, cert not-ready/expiring<14d, and node-not-ready split: `ControlPlaneNodeNotReady` = **page** (single CP node), worker `NodeNotReady` = warn, via `kube_node_role`. Fixed the stale `local-path on both mock and prod` + single-host storage comments.
- **t004 — bex rules.** Backup >26h stale (×2), **`OpenBaoSealed` via per-pod telemetry** `vault_core_unsealed==0` (readiness would miss a sealed follower — the chart keeps it in the Service; telemetry stanza + `unauthenticated_metrics_access` added to `openbao.values.yaml`), `BexApiDown`, **`StrandedNodeLocalImages`** (ImagePullBackOff/ErrImageNeverPull in tenant ns — the node-local-image failure mode), Traefik 5xx>5%. 12 rules total.
- **t005 — acceptance.** `scripts/alerts-verify.sh` (mock cluster) passed live: a throwaway AM (email receiver swapped for an in-cluster capture webhook — no real mail, no SMTP secret) delivered **firing then resolved** for a hand-fired alert; the two required rule-breaks are proven deterministically by `promtool test rules`. Latency ≈ eval interval (1m) + `for:` + `group_wait` (30s).
- **t006 — simplify.** 4-agent `/simplify` applied (before rebase): scrape intervals tuned to 60s where minute-scale alerts allow, `yq|yq`→single `from_yaml`, deduped verify wait-loops, local overlay disables AM (parity with the `$patch:delete`d backup jobs).
- **t007 — tests.** `promtool check`+`test rules` wired into `scripts/gitops-validate.sh` + CI (`gitops.yml` installs promtool); unit tests pin backup-age (>26h not 25h), 5xx-floor, CrashLoop `for:`, and per-member OpenBao seal detection.

`kustomize build` both overlays + `scripts/gitops-validate.sh` (incl. promtool) green; no secret value in git.

## Definition of done

Breaking a monitored invariant (e.g. a backup CronJob suspended past its window, a bex-system Deployment rolled to a bad image) produces a notification on the configured channel within one evaluation window, and recovery resolves it; rules are declarative in gitops; `kustomize build` of both overlays stays green; no secret value in git.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-07-09; `deploy/gitops/base/prometheus.yaml` ("Server-only (no alertmanager/pushgateway/exporters)"); the unmonitored backup CronJobs from w1/m1 (etcd) and w1/m7/t006 (OpenBao); docs/ADR013-secrets.md's sealed-on-restart failure mode.
- **Goal linkage:** GOAL.md #2 ("Basic obs for operation") — the operator half; continuous complement to w1/m7's point-in-time posture checks.
- **Expected outcome:** platform failures (and silent backup rot) page a human instead of waiting to be noticed; the OpenBao-sealed state that 503s the env-vars API is alerted, not discovered by tenants.
- **Why now:** prod now carries real tenant state (OpenBao credentials; enforced authz queued in w1/m9) with zero paging; every day of silent-failure risk on the backup jobs is unrecoverable-data risk.
- **Render parity closing task: omitted** — pure platform infra; no REST/GraphQL/MCP/UI surface changes.
