# w4 · m86 — Build-toolchain freshness monitoring and digest refresh cadence

**Worker:** worker4 **Goal:** keep the digest-pinned build toolchain current and observable: upstream digest movement becomes a reviewable scheduled signal, while an unready or over-age kpack `ClusterBuilder` becomes an alert instead of silently shipping stale toolchains. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Build a deterministic pinned-image freshness inventory — **DONE** | 45m | w7/m85/t008 |
| t002 | Add the scheduled digest drift issue workflow — **DONE** | 45m | t001 |
| t003 | Export kpack ClusterBuilder readiness and image-age metrics — **DONE** | 50m | t001 |
| t004 | Alert on stale or unready build toolchains and document the refresh SLO — **DONE** | 40m | t002, t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- One deterministic inventory covers every builder, native base, and helper image pinned by `w7/m85`, with its committed reference and a machine-readable resolution timestamp/source.
- A scheduled check resolves the same upstream references, detects digest movement without applying it, and opens/updates/closes one bounded tracking issue carrying exact replacement digests and affected files.
- The operator exports low-cardinality kpack `ClusterBuilder` readiness and image-age signals derived from the live resource plus the committed resolution metadata.
- Tested Prometheus rules alert when the builder is not Ready or exceeds the documented freshness SLO; normal fresh/Ready state stays quiet.
- No image updates silently or automatically. Accepting a changed digest remains an explicit reviewed commit, and the workflow has no production-cluster credential.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w4`, 2026-08-18, materializing `.pm/w7/030.md` and ADR060 D7's residual after `w7/m85` closes the digest-pinning inventory.
- **Goal linkage:** `.pm/GOAL.md` goals 3 and 7: reliable git-push builds and supply-chain security. A pinned but indefinitely stale builder undermines both goals.
- **Expected outcome:** upstream builder/base-image movement and a stale or unhealthy live `ClusterBuilder` become bounded, actionable signals instead of silent toolchain drift.
- **Why now:** `w7/m85` establishes the permanent no-floating-image invariant. Adding the refresh and live-health lifecycle immediately afterward prevents that security control from freezing old CVEs into every future build.
- **Render parity:** omitted because this milestone changes scheduled repository checks, operator metrics, and Prometheus rules only; it has no REST, GraphQL, MCP, or dashboard surface.

## Closeout

- **Inventory + resolver:** `lego/operator/internal/build/toolchain-freshness.json` lists every builder, stack, native-base, and helper pin in the operator build/publish sites (including aws-cli publish and BuildKit prewarm). `bash scripts/build-toolchain-freshness.sh validate` fails closed on uncovered pin-site digests and malformed `resolved_at`. `resolve` is byte-identical against a fixture map and never edits the tree.
- **Workflow:** `.github/workflows/build-toolchain-freshness.yml` is Thursday-weekly + `workflow_dispatch`, `contents: read` + `issues: write`, `github.token` only. Open/update/close/noop decisions are covered by `scripts/build-toolchain-freshness.test.sh`.
- **Metrics:** scrape-time unlabeled gauges `bex_build_clusterbuilder_present` / `_ready` / `_image_resolved_timestamp_seconds`. Age is committed `resolved_at`, never a live tag. Collection cannot block App reconcile. `go test ./internal/build/ ./internal/controller/` green.
- **Alerts:** `ClusterBuilderNotReady` (15m) and `ClusterBuilderImageStale` (30-day SLO, 1h pending). promtool 2.55.1: 37 rules check SUCCESS; `alerts_test.yml` SUCCESS (ready quiet, unready/absent fire, fresh quiet, over-age and timestamp-0 fire after `for: 1h`).
- **Prerequisite note:** `w7/m85` closeout is still open on that board; this inventory seeds the pins already in tree. The dashboard `node:22-alpine` FROM remains unpinned and is out of this milestone's pin-site set.
