# w2/m93 · t002 — Collection activation

**Date:** 2026-09-09

## Changes

| Seam | Change |
| --- | --- |
| `deploy/gitops/base/loki.yaml` | `retention_period: 504h` (21d); PVC `60Gi`; `ignoreDifferences` for StatefulSet VCT (online grow) |
| `deploy/gitops/base/log-shipper.yaml` | unified `platform_pods` keep for `dashboard` + `bex-registry` → `service=zot` + `bex-system`/`static-server` → `service=static-server`, all `type=platform` |
| `lego/operator/internal/staticserver` | bounded `static_legacy_origin_get` Info line when serving legacy `<app>/<rev>/` prefix |
| `scripts/gitops-validate.sh` | pins shipper pipelines + Loki 504h/60Gi |

## Controlled-read proof (post-roll)

After Argo syncs the shipper + retention:

1. Scratch only — never tenant blob mutation: `curl -u scratch:… http://zot:5000/v2/<scratch-legacy>/manifests/…` (or skopeo against a non-tenant scratch repo) and confirm a Loki hit on `{type="platform",service="zot"} |= "/v2/<scratch-legacy>/"`.
2. Optional static: hit a host still on legacy prefix (unlabeled `hello-static` is fine) and confirm `{type="platform",service="static-server"} |~ "static_legacy_origin_get"`.
3. Confirm collection failures (kill shipper / wrong selector) flip readiness to `insufficient_evidence`, not `clean`.

Retention: Loki retains 21d; path/prefix stay line-only (no secrets, no unbounded labels).

## Ops note

Existing Loki PVC must be grown online to ≥60Gi before the new retention can fill; Argo ignores the VCT size field so the live claim is the source of truth until expanded.
