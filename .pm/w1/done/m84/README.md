# w1 · m84 — Persistent disks 2/4: bex-api CRUD + provisioned-capacity billing

**Worker:** worker1 **Goal:** Render-shaped `/v1/disks` CRUD on all three surfaces backed by the control-plane store, with the `disk_gb_seconds` meter billing provisioned `sizeGB` at $0.175/GB-month through the estimate and Stripe — ADR082 D6, D8–D9. **Status:** done

## Tasks (in order)

| id   | title                                                         | est | depends_on             |
| ---- | ------------------------------------------------------------- | --- | ---------------------- |
| t001 | Store migration + `id.Disk` kind + projector ownership — **DONE** | 45m | —                      |
| t002 | REST `/v1/disks` CRUD + scope matrix + OpenAPI route pin — **DONE** | 1h  | t001                   |
| t003 | GraphQL + MCP mirrors + `AppView.Disk` — **DONE** | 45m | t002                   |
| t004 | `disk_gb_seconds` meter: rollup, catch-up, usage surfaces — **DONE** | 1h  | t001                   |
| t005 | `pricing.yaml` disk rate + Stripe `disk_gb_hours` — **DONE** | 30m | t004                   |
| t006 | Namespace quota re-derivation for disks — **DONE** | 30m | t001                   |
| t007 | Render parity check across REST/GraphQL/MCP/dashboard shapes — **DONE** | 30m | t003, t005, t006       |
| t008 | Simplify pass over the changed backend code — **DONE** | 30m | t007                   |
| t009 | Test coverage: CRUD authz/shape + meter lifecycle — **DONE** | 1h  | t008                   |
| t010 | Closeout — **DONE** | 15m | t009                   |

## Definition of done

With m83 deployed: `POST /v1/disks {serviceId, name, mountPath, sizeGB}` creates a `dsk-` disk, projects `spec.disk`, and triggers a deploy; GET/PATCH/DELETE match Render's schemas (PATCH grow-only → 400 on shrink; second disk → 409; DELETE removes the PVC via the projector); GraphQL and MCP expose the same verbs with identical semantics; `render_openapi_test.go` accepts the routes (the `/v1/disks` 404 pin removed); `GET /v1/usage` shows a `disk_gb_seconds` row and `estimatedCost` line for a live disk, billed on provisioned `sizeGB` creation→deletion including while suspended; `BillableMeterNames` carries `disk_gb_hours` and the Stripe setup script provisions its Price; free/hobby namespace quotas admit a default disk. Backend suite green on real Postgres + OpenFGA; authz/target/scope sweep tests pass with the new verbs. — **Met**: verified against ephemeral `postgres:17` + `openfga/openfga` containers matching CI's service definitions (`go test -p 1 ./...` clean), `make lint` clean on all four modules, and the operator suite still green.

## Outcome notes

- **Two bugs the new tests caught.** (1) `UpdateDisk`/`DeleteDisk` mixed Go's clock with Postgres's when closing a size period, so a grow could write `to_ts < from_ts` — a constraint violation, and worse, negative billed time. Every period boundary now comes from the database clock (`now()` inside the transaction), so a close and the open beside it land on the identical instant. (2) `ListDisks`/`UpdateDisk` reported "store unavailable" *before* authorizing, which the authz and audit sweeps caught: an unauthorized caller learned which subsystems this deployment runs and no denial was audited. Authorization now runs first, and production still passes through exactly one gate.
- **The meter deliberately escapes the Prometheus gate.** `RunWithIntervals` previously skipped the entire rollup when `BEX_PROM_URL` was empty. Disks are billed from control-plane rows, so gating them on Prometheus would silently stop charging for volumes that still exist; the tick now always runs and the Prometheus-backed meters skip themselves instead. Pinned by `TestDiskMeterRunsWithoutPrometheus`.
- **Billing is integrated, not sampled.** `service_disk_sizes` records one row per size a disk has held, so a grow landing mid-hour bills the old size up to the change and the new size after it, and a deletion stops the clock at the deletion instant. Proven against real Postgres (`TestPGDiskUsageIntegratesSizePeriods`: 10 GB for 30 min + 40 GB for 30 min = 90 000 GB-seconds).
- **Guards that had to be updated rather than worked around:** the Render route inventory (+5 disk operationIds; the two snapshot ops arrive with m85), the MCP parity counts (176 → 181 tools, all Extension class since Render's MCP has no disk tools), the cross-workspace REST matrix (both collection routes classified caller-scoped with a justification), the Stripe catalog meter list, and the scope matrix (regenerated — which also filled in two pre-existing gaps, `cloneEnvGroup` and `setImage`).
- **Disk changes now appear in the service activity feed** as `disk_attached` / `disk_updated` / `disk_detached`; the events vocabulary guard requires every targeted write verb to be either an event or explicitly excused, and silently excusing a billable change would have been the wrong answer.
- **Known documented subset:** `GET /v1/disks` supports Render's `serviceId` filter plus cursor/limit, not its full `diskId[]`/`name`/`createdBefore`/`updatedAfter` filter set. Snapshot routes are m85. Both belong in ADR018's row when m86 closes the records out.

## Source + Goal linkage

- **Source:** [docs/ADR082-persistent-disks.md](../../../docs/ADR082-persistent-disks.md) (D6, D8, D9, D11 stage 2); evidence [docs/render-artifacts/disks.md](../../../docs/render-artifacts/disks.md); anti-goal re-opened 2026-08-22 ([.pm/DO_NOT_DO.md](../../DO_NOT_DO.md)).
- **Goal linkage:** Render API parity (ADR006/ADR018) + the 30%-off pricing pillar (ADR030) — disks become the one SKU where estimate equals invoice exactly.
- **Expected outcome:** disks are creatable/billable through every API surface; the unmodified Render CLI ecosystem's API clients can drive them.
- **Why now:** stage 2 of ADR082 D11 — depends on m83's CRD contract; m85 snapshots and m86 dashboard build on these verbs.
- **Render parity closing task included** (t007): this milestone changes REST/GraphQL/MCP surfaces.
