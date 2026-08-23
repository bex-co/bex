# w1 · m84 — Persistent disks 2/4: bex-api CRUD + provisioned-capacity billing

**Worker:** worker1 **Goal:** Render-shaped `/v1/disks` CRUD on all three surfaces backed by the control-plane store, with the `disk_gb_seconds` meter billing provisioned `sizeGB` at $0.175/GB-month through the estimate and Stripe — ADR082 D6, D8–D9. **Status:** todo

## Tasks (in order)

| id   | title                                                         | est | depends_on             |
| ---- | ------------------------------------------------------------- | --- | ---------------------- |
| t001 | Store migration + `id.Disk` kind + projector ownership        | 45m | —                      |
| t002 | REST `/v1/disks` CRUD + scope matrix + OpenAPI route pin      | 1h  | t001                   |
| t003 | GraphQL + MCP mirrors + `AppView.Disk`                        | 45m | t002                   |
| t004 | `disk_gb_seconds` meter: rollup, catch-up, usage surfaces     | 1h  | t001                   |
| t005 | `pricing.yaml` disk rate + Stripe `disk_gb_hours`             | 30m | t004                   |
| t006 | Namespace quota re-derivation for disks                       | 30m | t001                   |
| t007 | Render parity check across REST/GraphQL/MCP/dashboard shapes  | 30m | t003, t005, t006       |
| t008 | Simplify pass over the changed backend code                   | 30m | t007                   |
| t009 | Test coverage: CRUD authz/shape + meter lifecycle             | 1h  | t008                   |
| t010 | Closeout                                                      | 15m | t009                   |

## Definition of done

With m83 deployed: `POST /v1/disks {serviceId, name, mountPath, sizeGB}` creates a `dsk-` disk, projects `spec.disk`, and triggers a deploy; GET/PATCH/DELETE match Render's schemas (PATCH grow-only → 400 on shrink; second disk → 409; DELETE removes the PVC via the projector); GraphQL and MCP expose the same verbs with identical semantics; `render_openapi_test.go` accepts the routes (the `/v1/disks` 404 pin removed); `GET /v1/usage` shows a `disk_gb_seconds` row and `estimatedCost` line for a live disk, billed on provisioned `sizeGB` creation→deletion including while suspended; `BillableMeterNames` carries `disk_gb_hours` and the Stripe setup script provisions its Price; free/hobby namespace quotas admit a default disk. Backend suite green on real Postgres + OpenFGA; authz/target/scope sweep tests pass with the new verbs.

## Source + Goal linkage

- **Source:** [docs/ADR082-persistent-disks.md](../../../docs/ADR082-persistent-disks.md) (D6, D8, D9, D11 stage 2); evidence [docs/render-artifacts/disks.md](../../../docs/render-artifacts/disks.md); anti-goal re-opened 2026-08-22 ([.pm/DO_NOT_DO.md](../../DO_NOT_DO.md)).
- **Goal linkage:** Render API parity (ADR006/ADR018) + the 30%-off pricing pillar (ADR030) — disks become the one SKU where estimate equals invoice exactly.
- **Expected outcome:** disks are creatable/billable through every API surface; the unmodified Render CLI ecosystem's API clients can drive them.
- **Why now:** stage 2 of ADR082 D11 — depends on m83's CRD contract; m85 snapshots and m86 dashboard build on these verbs.
- **Render parity closing task included** (t007): this milestone changes REST/GraphQL/MCP surfaces.
