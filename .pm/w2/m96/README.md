# w2 · m96 — Charges and webhook deliveries name their resource, even after deletion

**Worker:** worker2 **Goal:** a charge line and a webhook delivery row keep the display name their resource had while the usage accrued or the event fired, and say when that resource no longer exists. A bare `srv-…` or sandbox UUID appears only for rows recorded before the fix, never for a resource bex once knew the name of. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Retain a tenant-scoped last display name per metered id, written on create and rename, surviving deletion, purged with the workspace  | 45m | —          |
| t002 | Usage cost rows resolve names from the retained record on REST, GraphQL and MCP; sandboxes label by session repo and branch or by plan and image | 35m | t001       |
| t003 | Charges card renders `name (deleted)` for a resource that no longer resolves and falls back to the id only when no name was recorded  | 20m | t002       |
| t004 | Webhook delivery view carries `serviceName` from the stored payload; the deliveries table shows the linked name with the id secondary | 30m | —          |
| t005 | Render parity                                                                                                                          | 15m | t003, t004 |
| t006 | Simplify                                                                                                                               | 15m | t005       |
| t007 | Test coverage                                                                                                                          | 40m | t005       |
| t008 | Closeout                                                                                                                               | 10m | t007       |

## Definition of done

Run each bullet on production (workspace `bex` / `tea-d98210cbbpdc73dcrkvg`) after the deploy, with fixtures created and deleted inside the run:

- **Deleted services are named in Charges.** On `/billing` → Charges → Expand all, period `2026-09`, the three rows that read `srv-dae9lb988i5c7399e9m0`, `srv-da7o6ovvqdcc73bpn9hg` and `srv-dah41v9c7cos73dm39ug` at filing time (2026-09-14) show a display name with a `(deleted)` marker, or a bare id **only** if the backfill decision recorded below explicitly leaves pre-fix rows unnamed. A fresh probe — create a web service, let one usage rollup pass, delete it — shows `<name> (deleted)` on the next rated period, on the Charges card and in `usage(ownerId) { estimatedCost { resources { serviceName } } }`.
- **Sandboxes are labelled.** The `271ec9ce-a32b-4128-bb43-02a9e57b01b6` row ($69.98 at filing) and every other sandbox row shows either its owning agent session's repo and branch or `plan · image`, never a bare UUID.
- **Live resources show their current name.** Rename a fixture service; the Charges row shows the new name on the next read. Delete it and re-create one with the same display name; the two stay separate lines keyed by id.
- **A delivery row names its service.** Create a webhook subscribed to Deploy events, deploy a fixture, open `/webhook/<whk-id>` → Activity → Recent deliveries. The Service column shows the service's name linked to the service, with the `srv-…` id visible as secondary text or tooltip.
- **A delivery for a deleted service keeps its name.** Delete the fixture service; the same delivery row still shows the recorded name (with a deleted marker), not a bare id. REST and GraphQL delivery reads both carry `serviceName`.
- **Workspace deletion purges retained names.** Deleting a throwaway workspace (the `w1/m61` teardown path) leaves no rows in the retained-name record for that tenant.
- **Backfill decision recorded** in this README under `## Backfill decision` by t002.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-15 (proposal 3), absorbing [w1/088](../../w1/done/088.md) (Billing → Charges names deleted services and every sandbox only by bare id) and [w1/090](../../w1/done/090.md) (a webhook's Recent deliveries table names each row's service only by bare `srv-` id). Both found by the live `/qa-find-bugs` hunt of 2026-09-14 (passes 6 and 13).
- **Goal linkage:** pillar 2 of [ADR008](../../../docs/ADR008-vision.md) — billing lines and delivery history are machine-readable state, and an opaque id fails that bar. Sandbox charges are real money on the reopened pillar 5.
- **Expected outcome:** "what did this cost me" and "which service's event failed" are answerable from the page itself, including for resources deleted since.
- **Why now:** `w1/088` explicitly asks to be promoted if name retention needs a migration, and it does. Both notes share one root cause — names resolved only from live rows (`usage/service.go:286-313`, `worker.go:346-349` payload not projected to the delivery view) — so one retained-name record closes both.
- **Render parity included:** REST `GET /v1/usage`, GraphQL `usage` and MCP share `monthToDateAt`, so the name resolution moves together on all three; the webhook delivery view is read on REST and GraphQL and must agree on `serviceName`. Render's own billing view for deleted services and its Recent deliveries column were not captured at filing and t005 records them.
