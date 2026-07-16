# w8 · m14 — Postgres disk autoscaling

**Worker:** worker8 **Goal:** Render's `enableDiskAutoscaling`/`diskAutoscalingEnabled` exists for managed Postgres: the operator grows `spec.storageGB` automatically when usage crosses the captured threshold, with a cap and an audit trail — never a silent resize. **Status:** done

## Tasks (in order)

| id   | title                                                                 | est | depends_on |
| ---- | --------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's trigger/step/cap semantics — **DONE**                 | 30m | —          |
| t002 | Operator: usage-watch → grow `storageGB` (cap + events) — **DONE**     | 60m | t001       |
| t003 | Field on REST/GraphQL/MCP create + PATCH + read — **DONE**             | 40m | t002       |
| t004 | Dashboard: toggle beside the disk chart — **DONE**                     | 30m | t003       |
| t005 | Render parity — **DONE**                                               | 30m | t004       |
| t006 | Simplify — **DONE**                                                    | 30m | t005       |
| t007 | Test coverage — **DONE**                                               | 45m | t005       |
| t008 | Closeout — **DONE**                                                    | 15m | t007       |

## Definition of done

A Database with autoscaling enabled whose PVC crosses the captured usage threshold gets its `spec.storageGB` grown by the captured step (bounded by the cap), the CNPG PVC expands, and the action lands in the service-events/status trail; disabled instances never resize; the boolean round-trips as `enableDiskAutoscaling` (write) / `diskAutoscalingEnabled` (read) on all three surfaces and toggles in the dashboard. Proven live on the mock cluster by filling a small volume past the threshold.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 7, 2026-07-14 — systematic field-diff of Render's pinned OpenAPI (`enableDiskAutoscaling` on postgresPOSTInput/PATCHInput, `diskAutoscalingEnabled` on the postgres object). Zero hits in `lego/`.
- **Goal linkage:** GOAL #4 (PostgreSQL) + Render parity. Every ingredient exists: `spec.storageGB` is grow-only by design (`database_types.go:40`), and kubelet's `kubelet_volume_stats_{used,capacity}_bytes` are already scraped (w3/m10's disk metrics) — this milestone is the control loop between them.
- **Expected outcome:** a busy tenant DB stops hitting disk-full at 3am; the field pair leaves w7/m30's future allowlist.
- **Why now:** spec-verified gap whose mechanism is two existing pieces plus a loop; w8's datastore thread (m12/m13 siblings). Render parity task included — all-surface change.
