# w2/m93 · t001 — Legacy-read evidence coverage

**Date:** 2026-09-09 · **Mode:** repository + live-architecture inventory (read-only)  
**Clean window claimed by m92:** 2026-09-08 → earliest Phase 4 **2026-09-22** (14 days)

## Migrated resource map

From `.pm/w2/done/m92/evidence/` (phases 1–3): seven labeled Apps tombstoned; live Deployments on scoped `W/A` image refs; **no labeled `static_site`** in the worklist. Unlabeled `default/hello-static` is out of scope (legitimate legacy prefix forever).

| Resource class | Legacy read signal | Concrete source today | Coverage |
| --- | --- | --- | --- |
| Tombstoned App registry (`A`) | kubelet/OCI pull of `/v2/A/(manifests\|blobs\|tags)/…` | Zot stdout (`module=http`, path, status) at `log.level=info` | **Gap** — Zot pods in `bex-registry` are **not** shipped to Loki (`log-shipper` has no `bex-registry` pipeline) |
| Tombstoned App static (`A/<rev>/`) | static-server origin GET under legacy prefix | _(none)_ — static-server never logs object keys/prefixes | **Gap** — no signal; also no labeled static migrated in m92 |
| Unlabeled App registry/static | same shapes | same sources | **Exclude** from readiness (not dual-read migration debt) |
| Current scoped image refs | Deployment/`status.image` `…/W/A:…` | `scripts/verify-workspace-scoped-identity.sh` | **Present** — inventory only; **not** historical traffic evidence |

## Retention and continuity

| Knob | Checked-in value | Need for 14-day window |
| --- | --- | --- |
| Loki `limits_config.retention_period` | `168h` (7d) | ≥ `336h` (14d) plus evaluation buffer → **`504h` (21d)** |
| Loki PVC | `20Gi` (local overlay `2Gi`) | Grow with retention + new Zot volume (~**60Gi** prod) |
| Collection restart / shipper dark | not modeled | Must surface as **insufficient evidence**, never clean |

## Earliest defensible continuous coverage start

**Not 2026-09-08.** Until Zot + static-server streams are retained in Loki at ≥14-day retention, any zero-read claim over that calendar window is **insufficient evidence**. The continuous clock starts at the first hour after the shipper pipelines and raised retention are live and producing `type=platform,service=zot` (and `service=static-server`) lines.

## Distinctions

- **Origin / registry reads** (this milestone) ≠ public Traefik URL access (`type=request`).
- **Scoped current refs** prove cutover inventory, not absence of legacy pulls.
- **Missing history, gaps, source errors** ≠ zero reads.

## Next (t002+)

Reuse Loki + Alloy; add bounded `service=zot` / `service=static-server` platform pipelines; raise retention+PVC; emit one bounded static legacy-prefix log line; add fail-closed `scripts/registry-migration-readiness.sh`.
