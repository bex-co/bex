# w1 · m20 — Per-service autoscaling (Render `PUT /services/{id}/autoscaling`)

**Worker:** worker1 **Goal:** Ship Render-parity per-service autoscaling — an autoscaling stanza on the `App` CR, a metric→replica reconciler that adjusts `spec.replicas` from live CPU/memory, and the REST/GraphQL/MCP + dashboard surfaces (`…/autoscaling`, Scaling tab) matching Render's names and semantics. **Status:** **DONE** 2026-07-11

## Tasks (in order)

| id   | title                                                                                        | est | depends_on           |
| ---- | -------------------------------------------------------------------------------------------- | --- | -------------------- |
| t001 | Autoscaling stanza on the `App` CRD (`spec.autoscaling`: min/max + target CPU%/mem%)          | 40m | —                    | — **DONE** |
| t002 | Metric→replica reconciler in the operator (live CPU/mem → `spec.replicas` within [min,max])   | 90m | w1/m20/t001          | — **DONE** |
| t003 | REST/GraphQL/MCP autoscaling surface (`PUT`/`DELETE …/autoscaling`, mutation, MCP tool)        | 60m | w1/m20/t001          | — **DONE** |
| t004 | Dashboard Scaling section (min/max + target controls, read+write via GraphQL)                  | 60m | w1/m20/t003          | — **DONE** |
| t005 | Render parity — verify `…/autoscaling` matches Render across REST/GraphQL/MCP/UI               | 30m | w1/m20/t002,w1/m20/t003,w1/m20/t004 | — **DONE** |
| t006 | Simplify — `/simplify` over what this milestone changed                                        | 20m | w1/m20/t005          | — **DONE** |
| t007 | Test coverage — reconciler unit + envtest, surface tests                                       | 40m | w1/m20/t005          | — **DONE** |
| t008 | Closeout — verify DoD holds, then move the milestone to `done/`                                | 10m | w1/m20/t007          | — **DONE** |

## Definition of done

- The `App` CR carries `spec.autoscaling{enabled, minReplicas, maxReplicas, targetCPUPercent, targetMemoryPercent}`; deepcopy + CRD manifest regenerated; `make test` green.
- With autoscaling enabled, a service under sustained CPU/mem above target scales `spec.replicas` up (bounded by `maxReplicas`) and back down when idle — observable via `kubectl get app` revisions and replica count; the loop never drives replicas below `minReplicas` and never fights the m3 node-level autoscaler (it moves replicas, m3 moves nodes) or m4 scale-to-zero.
- `PUT /v1/services/{id}/autoscaling` and `DELETE …` exist with Render-identical request/response fields and error shapes; GraphQL mutation + MCP tool expose the same; dashboard Scaling tab reads and writes them.
- Metrics read as % of the tier requests/limits shipped by m8 (no new metrics plumbing).

## Source + Goal linkage

- **Source:** inbox note `w1/008` (from the `docs/ADR018-render-parity.md` audit, m13, 2026-07-08); moved to `w1/done/008.md` on promotion.
- **Goal linkage:** pillar 1 (Render parity) + elastic substrate (w1/m3, w1/m4).
- **Expected outcome:** services auto-scale on load without operator intervention, matching Render's Scaling tab (Pro+).
- **Why now:** the gating dependency is satisfied — w1/m3 landed node elasticity (2026-07-11) for replica scale-ups to land on, and m19's rebuild explicitly names this note as unblocked. The metric→replica reconciler is new per-service work (m3 is aggregate/node-level and never touches `spec.replicas`).
- **Render parity task included:** the change adds REST/GraphQL/MCP/UI surfaces, so cross-surface parity must be verified against render.com.
