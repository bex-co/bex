# w2 · m170 — Grafana analytics keep trusting `bex-db` across CA renewals

**Worker:** worker2 **Goal:** both Grafana analytics datasources are healthy again, a `bex-db` CA renewal reaches them with no human reseal, and a failing analytics datasource or stale CA copy raises an alert instead of going unnoticed. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reseal both analytics readers now and confirm datasource health (**production change — confirm authorization**) | 30m | — |
| t002 | Inventory every pinned copy of `bex-db-ca` and CNPG's renewal cadence | 30m | — |
| t003 | Replace the sealed CA copy with an automatic mirror of the public `ca.crt` (or a long-lived CA, if t002's evidence favors it) | 1.5h | t002 |
| t004 | Alert on a failing analytics datasource and on CA-copy drift or approaching expiry | 45m | t002 |
| t005 | Rotation drill on a dev stack: the readers recover from a CA change with no human step | 45m | t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- Production: `GET /api/datasources/uid/product-analytics/health` and `…/cli-analytics/health` both report OK, and the CA each reader trusts equals `bex-system/bex-db-ca` (same `notAfter`/fingerprint).
- No committed SealedSecret carries a copy of the `bex-db` CA; the readers' `ca.crt` follows `bex-db-ca` automatically, and `scripts/gitops-validate.sh` (or the bootstrap test) fails closed if a sealed CA copy reappears.
- A promtool test proves the new alert fires for a failing analytics datasource and for a CA copy that differs from, or is within the warning window of expiring before, `bex-db-ca`.
- A recorded dev drill: the CA changes, the readers stay or become healthy without anyone running the bootstrap or a reseal.
- `w7/060` is retired into this milestone (its text preserved as [source-w7-060.md](source-w7-060.md)).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` (2026-10-19), item 3, chosen by the user the same day. Absorbs `w7/blocked/060.md` (moved here as `source-w7-060.md`), whose external gate — CNPG issuing the renewed CA — cleared around 2026-10-01 with no follow-up since 2026-09-28; also executes that note's own "consider a milestone that removes the manual step".
- **Evidence (read-only, `hetzner-prod`, 2026-10-19):** `bex-system/bex-db-ca` `notAfter=Dec 30 23:56:17 2026 GMT`; `monitoring/grafana-product-analytics` and `monitoring/grafana-cli-analytics` `ca.crt` `notAfter=Oct  8 23:06:42 2026 GMT`. Both datasources use `sslmode: verify-full` with `sslRootCertFile` mounted from those Secrets (`deploy/gitops/base/values/grafana.values.yaml:50-65,110-150`), so both have trusted an expired CA for eleven days. Datasource health endpoints were not queried; failure is inferred from the expired trust root.
- **Goal linkage:** [ADR088](../../../docs/ADR088-platform-observability-ui.md) platform observability (product and CLI analytics are its business-metrics half); [ADR008](../../../docs/ADR008-vision.md) "Ops hardening".
- **Expected outcome:** analytics dashboards work again, and the roughly 90-day manual reseal disappears.
- **Why now:** broken today and unnoticed for eleven days — proof that nothing watches datasource health; the renewed CA expires 2026-12-30, so the same outage recurs in about ten weeks without t003.
- **Render parity omitted:** platform-internal observability; no REST/GraphQL/MCP/UI surface changes.
- **Custody constraint:** `w7/done/m153` made `scripts/lib/analytics-bootstrap.sh` fail closed on CA drift for SealedSecret-managed readers. t003 must keep the reader **password** under sealed custody and move only the public CA out of it, updating that contract rather than bypassing it.
