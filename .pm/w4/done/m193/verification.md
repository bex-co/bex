# m193 verification — 2026-10-11 UTC

Shipped fix: `6bd92c89a` (deploy.yml run 38109529626 succeeded: all test jobs, build, deploy; images pinned in `6585f455c`).

## Change (t001)

- `keyValueStorageIntent` no longer passes `pvcCapacityStorageGB(pvc)` into `growOnlyIntent`; only sizes bex itself requested feed the high-water (recorded allocation, plan floor, spec, claim template, PVC request).
- `reconcileKeyValueStorage` computes `desiredGB = max(desiredGB, requestedGB)`. Physical capacity no longer raises the PVC request or `status.allocatedStorageGB`. It is still stamped into `status.storageCapacityGB` and still gates `StorageReady` (`PVCResizePending` / `FileSystemResizePending` while it trails the request). No PVC is ever shrunk.
- `growOnlyIntent`'s variadic is now `requestedSizesGB`, documenting that observed capacity must never be passed.
- ADR021's grow-only paragraph now separates logical allocation from physical capacity and records the legacy disposition.

## Caller audit (t002)

| Caller / guarantee | Disposition |
| --- | --- |
| `growOnlyIntent` (2): Key Value | fixed (KV-scoped; shared arithmetic unchanged) |
| `growOnlyIntent`: Postgres `database_controller.go` | unchanged — passes CNPG requested size, never PVC capacity. Live 1 GB control `dpg-d9nqg95cavls73fp8m20`: Details 1 GB, Metrics Capacity 1 GiB, GraphQL 1073741824 |
| `expandPVCTo` (2): KV, App disk | KV now expands only to logical intent; App disk unchanged |
| App disk `disk.go` `growDiskPVC` `max(spec, request, capacity, allocated)` | source-traced, **not changed**: since 2026-08-24 `validateDiskSize` enforces a 10 GB minimum (= Hetzner's floor), so capacity cannot exceed the request for disks created under current policy. Pre-policy sub-10 GB disks report their provisioned 10 GB volume, which ADR082 D8 names as the billing basis. No live service-disk fixture (paid-only, out of scope) |
| `DatastoreMetrics` (5 callers), panel mounts (3) | consumers unchanged; they already read logical `EffectiveStorageGB` |
| Disk USED / `PersistentVolumeFillingUp` | unchanged (kubelet physical series) |
| `keyvalue_backup.go` job sizing from `AllocatedStorageGB` | now sized from logical allocation (≥ plan floor; 2× + headroom) — smaller for new Free stores, still covers the 1 GB data bound |
| 5 GB Postgres control | none in the QA workspace; covered by the unchanged Postgres path + unit cases |

**Legacy residual (explicit):** stores reconciled before the fix keep a floor-inflated `allocatedStorageGB`/PVC request. Bookkeeping cannot separate that from genuine growth (a Free→Standard→Free store looks identical), so they are retained, not silently decreased. Read-only live: `red-d9p49kdrtmes73c34ovg` (starter) and `red-da4086iii7bs73drbqh0` (standard) still report 10737418240. A repair would need per-store evidence and an explicit decision; not filed as work here.

## Automated gates (t005)

- `TestKeyValueLogicalStorageIgnoresPhysicalFloor` drives `keyValueStorageIntent` + `reconcileKeyValueStorage` twice on a 10 Gi physical floor and reads `tiers.Valkey.EffectiveStorageGB`: default Free stays 1 GB, explicit growth to 5, plan floor to 5, downgrade keeps 5, legacy 10 retained, growth to 20 stays `PVCResizePending`. Against the old producer 4 of 6 cases fail ("allocated = 10 GB").
- `TestDiskCapacityIdenticalAcrossRESTGraphQLMCP` now also covers a Free KV with status allocated 1 / capacity 10 and a 10 GiB physical source: REST/GraphQL/MCP all 1073741824. Existing shrink-rejection, expansion/StorageClass/quota and envtest resize cases unchanged and green.
- `make test` (operator), `make lint` (0 issues ×4), backend `go test ./...` and the real-DB ship gate passed. govulncheck on main is red from new `golang.org/x/net` advisories (GO-2026-6603/6610/6611/6612/6617), pre-existing on prior commits — unrelated.

## Live acceptance (t003) — fresh default Free fixture `red-db5gvq4npnks73cvv4jg`

- Created with the finding's exact `CreateKeyValue` variables (plan free, version 8, public, allkeys-lru, journal-snapshot, **no storageGB**); catalog `free.storageGB` 1. Reached Available.
- Capacity: GraphQL `datastoreMetrics{kind:keyvalue,name:DISK_CAPACITY}` 1073741824; REST `/v1/metrics/disk-capacity?resource=…&kind=keyvalue` and kind-inferred both 1073741824; MCP `get_datastore_metrics {resourceId, metricTypes:["disk_capacity"]}` structuredContent 1073741824. Dashboard `/r/<id>` → `/keyvalue/<id>`, Metrics tab after reload: **Capacity 1 GiB** (`.playwright-mcp/qa-m193-kv-capacity.png`).
- Data controls over TLS (SNI): PING, marker SET and counter 40→INCR 41 with absolute PXAT 1791700050000. Suspend → resume → rename to `qa-20261011-m193-renamed`: both values and PEXPIRETIME 1791700050000 survived (TTL 7115s), id and external endpoint unchanged, capacity still 1073741824.
- Render parity: Render Free Key Value has no persistence; bex's durable 1 GB Free disk is a deliberate divergence (ADR021). No API field or policy added.

## Cleanup

Fixture deleted via `deleteKeyValue` (REST read 404; workspace KV list back to the two pre-existing stores). This run's Kratos session revoked with `qa-login.sh --logout` ("ok logged-out"); jar/state files removed. No other resources touched.
