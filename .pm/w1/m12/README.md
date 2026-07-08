# w1 · m12 — Render scale API (`POST /v1/services/{id}/scale` → spec replicas)

**Worker:** worker1 **Goal:** Render's manual-scaling verb as an App CR spec patch the operator converges — the degenerate case of m3 (bin-pack/autoscale) and m4 (scale-to-zero), so the spec field semantics settled here must stay compatible with both. **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `spec.replicas` semantics + REST `POST /v1/services/{id}/scale`                 | 30m | —          |
| t002 | GraphQL + MCP parity (`scaleService` / `scale_service`)                          | 25m | t001       |
| t003 | Acceptance: 1→3→1 converges; suspend still wins; m3/m4 compat notes             | 20m | t002       |
| t004 | Simplify — `/simplify` over the code this milestone changed                      | 20m | t003       |
| t005 | Test coverage — meaningful tests for the behavior this milestone shipped         | 30m | t003       |

## Definition of done

`POST /v1/services/{id}/scale` with `{"numInstances": 3}` yields 3 ready pods and `replicas: 3` on the service object across all three surfaces; suspend/resume behavior is unchanged (suspended app stays at 0 regardless of replicas).

## Source + Goal linkage

- **Source:** promoted from inbox `w1/004` (2026-07-08, originally from `/pm-brainstorm more milestones for w5`).
- **Goal linkage:** Render parity, pillar-1 API-first.
- **Expected outcome:** unblocks the paired dashboard note `w5/005` (manual-scaling section in service Settings).
- **Why now:** trivially small surface over an existing spec field, and it pins down the replica-field semantics m3 (autoscale) and m4 (scale-to-zero) will build on — cheaper to settle now than after the autoscaler lands.
