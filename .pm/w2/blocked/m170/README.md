# w2 · m170 — Grafana analytics keep trusting `bex-db` across CA renewals

**Worker:** worker2 **Goal:** both Grafana analytics datasources are healthy again, a `bex-db` CA renewal reaches them with no human reseal, and a failing analytics datasource or stale CA copy raises an alert instead of going unnoticed. **Status:** blocked — t001, t002, t004 done and live 2026-10-20 (both readers verify the renewed CA; the probe passes in production); t003 needs the user's authorization to give `bex-db` a long-lived cert-manager server CA (a production TLS change on the control-plane database), then t005's drill

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reseal both analytics readers now and confirm datasource health — **DONE** | 30m | — |
| t002 | Inventory every pinned copy of `bex-db-ca` and CNPG's renewal cadence — **DONE** | 30m | — |
| t003 | Give `bex-db` a long-lived cert-manager server CA via CNPG `spec.certificates` so renewals stop invalidating the sealed copy (**STOP: production TLS change on the control-plane DB — needs the user's authorization**; the automatic-mirror option is rejected, see Decisions) | 1.5h | t002 |
| t004 | Alert on a failing analytics datasource and on CA-copy drift or approaching expiry — **DONE** (probe + alert + panel; the expiry-window warning moves to t003) | 45m | t002 |
| t005 | Rotation drill on a dev stack: the readers recover from a CA change with no human step | 45m | t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- Production: both analytics readers connect with `verify-full` and the trusted CA equals `bex-system/bex-db-ca` (same `notAfter`/fingerprint) — shown by the `analytics-datasource-probe` CronJob succeeding.
- A failing analytics datasource cannot go unnoticed again: the probe connects exactly as Grafana does every 10 minutes, `AnalyticsDatasourceProbeStale` fires after 30 minutes without a success, a promtool test proves fire/quiet cases, and a `bex-data-plane` panel draws the same series.
- A `bex-db` CA renewal no longer requires a reseal: `bex-db`'s server certificate chains to a long-lived cert-manager CA (CNPG `spec.certificates`), Grafana trusts that CA, and cert-manager's expiry metric alerts well before it expires. _(Revised 2026-10-19: the original "automatic mirror of `ca.crt`, no sealed CA copy" bullet is not achievable safely — see Decisions.)_
- A recorded dev drill: the server certificate rotates, and the readers stay healthy without anyone running the bootstrap or a reseal.
- `w7/060` is retired into this milestone (its text preserved as [source-w7-060.md](source-w7-060.md)).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` (2026-10-19), item 3, chosen by the user the same day. Absorbs `w7/blocked/060.md` (moved here as `source-w7-060.md`), whose external gate — CNPG issuing the renewed CA — cleared around 2026-10-01 with no follow-up since 2026-09-28; also executes that note's own "consider a milestone that removes the manual step".
- **Evidence (read-only, `hetzner-prod`, 2026-10-19):** `bex-system/bex-db-ca` `notAfter=Dec 30 23:56:17 2026 GMT`; `monitoring/grafana-product-analytics` and `monitoring/grafana-cli-analytics` `ca.crt` `notAfter=Oct  8 23:06:42 2026 GMT`. Both datasources use `sslmode: verify-full` with `sslRootCertFile` mounted from those Secrets (`deploy/gitops/base/values/grafana.values.yaml:50-65,110-150`), so both have trusted an expired CA for eleven days. Datasource health endpoints were not queried; failure is inferred from the expired trust root.
- **Goal linkage:** [ADR088](../../../../docs/ADR088-platform-observability-ui.md) platform observability (product and CLI analytics are its business-metrics half); [ADR008](../../../../docs/ADR008-vision.md) "Ops hardening".
- **Expected outcome:** analytics dashboards work again, and the roughly 90-day manual reseal disappears.
- **Why now:** broken today and unnoticed for eleven days — proof that nothing watches datasource health; the renewed CA expires 2026-12-30, so the same outage recurs in about ten weeks without t003.
- **Render parity omitted:** platform-internal observability; no REST/GraphQL/MCP/UI surface changes.
- **Custody constraint:** `w7/done/m153` made `scripts/lib/analytics-bootstrap.sh` fail closed on CA drift for SealedSecret-managed readers. t003 must keep the reader **password** under sealed custody and move only the public CA out of it, updating that contract rather than bypassing it.

## Inventory

**t002 (2026-10-19, read-only).**

- **Pinned copies of the `bex-db` CA:** only `monitoring/grafana-product-analytics` and `monitoring/grafana-cli-analytics` (`ca.crt` beside the reader `password`, mounted by Grafana as `sslRootCertFile`). They are the only `verify-full` clients of `bex-db` in the repository; in-namespace clients use the CNPG app Secret URI without CA verification.
- **Renewal cadence:** CNPG self-manages the CA (`spec.certificates` empty). `status.certificates.expirations` shows `bex-db-ca`, `bex-db-server` and `bex-db-replication` all expiring 2026-12-30 23:56:17Z, i.e. a ~90-day cycle that rotates the CA together with the server certificate — every cycle invalidates a pinned copy.
- **Failure confirmed:** an `openssl s_client -starttls postgres -verify_hostname bex-db-rw.bex-system.svc` handshake fails with `certificate has expired` against Grafana's pinned CA and verifies OK against the current `bex-db-ca`; `psql` with the live reader password, the current CA and `verify-full` succeeds for both readers (`SELECT 1 FROM <profile>_analytics.events`).

## Decisions

- **No automatic CA mirror (t003 option (a) rejected).** `bex-system/bex-db-ca` contains the CA **private key** (`ca.key`) alongside `ca.crt`, and Kubernetes RBAC cannot grant one key of a Secret. Any mirror — an operator reconciler or a CronJob — would give a long-lived identity read access to the key that signs `bex-db` client certificates (e.g. `streaming_replica`), which contradicts the operator RBAC rule that keeps platform credentials out of its reach (`deploy/gitops/base/operator-registry-rbac.yaml` header). The one-off human reseal keeps that key with operators only.
- **Durable fix = long-lived server CA (option (b)).** A cert-manager self-signed CA (`isCA`, multi-year) plus a CA Issuer and a short-lived server `Certificate` for `bex-db-rw`/`-r`/`-ro` (`.bex-system`, `.svc`, `.svc.cluster.local`), wired through CNPG `spec.certificates.serverCASecret` / `serverTLSSecret`; CNPG keeps managing the client CA. Grafana is resealed once with the long-lived CA, cert-manager rotates the server certificate underneath it, and `certmanager_certificate_expiration_timestamp_seconds` (already scraped) drives the expiry-window alert. This changes TLS on the production control-plane database, so it waits for the user's go-ahead.
- **Detection ships independently (t004).** `deploy/gitops/overlays/prod/analytics-datasource-probe.yaml` (no Kubernetes API access; mounts only the two reader Secrets), `AnalyticsDatasourceProbeStale` over the recording rule `bex:analytics_datasource_probe_age_seconds` in `deploy/gitops/base/prometheus.yaml` with `deploy/gitops/base/rules/analytics_probe_alerts_test.yml` (fire after 30m, never-succeeded fires, recent/suspended/unrelated stay quiet; a threshold mutation fails the test), and panel 12 on `bex-data-plane`. Runbook: ADR088 § Analytics datasource custody.

## Evidence

- **t001 (2026-10-19):** both SealedSecrets resealed with the unchanged live reader passwords and the current `bex-db-ca` (`kubeseal` against the production controller; only the two `encryptedData` values change per file; nothing printed). The probe was dry-run against production through a port-forward with the same script: stale CA → `certificate verify failed`; current CA → both readers OK. Live confirmation after the Argo sync is recorded below.
- **Live (2026-10-20):** after the root sync stalled 06:17Z–07:21Z on an unrelated placeholder (fixed as `w2/048`), Argo applied `e7d71fd10`: both `monitoring/grafana-{product,cli}-analytics` now carry `ca.crt` `notAfter=Dec 30 23:56:17 2026 GMT`, and the first scheduled `analytics-datasource-probe` run (07:30Z, `analytics-datasource-probe-29874690`) logged `ok: product analytics reader (verify-full)` / `ok: cli analytics reader (verify-full)`.
