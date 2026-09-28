# Reseal the Grafana analytics readers after the bex-db CA renewal

Why: both Grafana analytics datasources verify `bex-db` with `sslmode: verify-full` against a sealed copy of `bex-system/bex-db-ca`; CNPG renews that CA before its 2026-10-08T23:06:42Z expiry (renewal window opens ~2026-10-01), after which both readers fail TLS until the copies are resealed.

- Observed 2026-09-28 during `w7/m153`: `bex-db` `status.certificates.expirations` shows `bex-db-ca` at `2026-10-08 23:06:42 +0000 UTC`; both `monitoring/grafana-{product,cli}-analytics` Secrets are SealedSecret-owned with a CA byte-identical to today's `bex-db-ca`.
- After the renewal, `bash scripts/{product,cli}-analytics-bootstrap.sh` fails closed with the CA-drift error (m153 custody contract). Fix: rerun each with `SEAL_TO=deploy/gitops/overlays/prod/grafana-<profile>-analytics.sealedsecret.yaml`, ship both manifests, then roll `monitoring/deployment/grafana` and confirm `/api/datasources/uid/{product,cli}-analytics/health` is OK.
- Gate: cannot be done before CNPG issues the new CA. If this recurs every 90 days, consider a milestone that removes the manual step (e.g. a longer-lived user-provided CA via `spec.certificates`, or trusting the server cert chain differently) instead of resealing each cycle.

**Parked 2026-09-28 (worker7):** blocked on an external event — CNPG has not yet issued the renewed `bex-db-ca` (current CA valid until 2026-10-08T23:06:42Z; renewal window opens ~2026-10-01). Nothing can be resealed before then; the bootstrap's CA-drift check (w7/m153) will fail closed once it has.
