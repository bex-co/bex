# w7 · m78 — A failed reconcile step must not strand a service with no Service and no Ingress

**Worker:** worker7 **Goal:** a web service that reaches the Deployment stage always gets its ClusterIP Service and Ingress, so a transient or unrelated reconcile failure can never leave a running pod permanently unreachable with `serviceDetails.url: null`. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est | depends_on               |
| ---- | ------------------------------------------------------------------ | --- | ------------------------ |
| t001 | Live evidence: identify which reconcile step actually aborted       | 30m | —                        |
| t002 | Fix the abort so routing cannot be stranded by an earlier step      | 45m | w7/m78/t001              |
| t003 | Render parity: `serviceDetails.url` + routing surfaces              | 25m | w7/m78/t002              |
| t004 | Simplify the code this milestone changed                            | 25m | w7/m78/t003              |
| t005 | Test coverage: reconcile-abort invariants                           | 35m | w7/m78/t003              |
| t006 | Closeout                                                            | 15m | w7/m78/t004, w7/m78/t005 |

## Definition of done

For a web service whose reconcile hits a failure in any step between the Deployment apply and the Ingress apply, the resulting state is either (a) routing present and `serviceDetails.url` populated, or (b) an explicit, user-visible failure that names the blocking step — never today's silent third state of running pods, no Service, no Ingress, `url: null`, and a bare "Update Failed".

The specific production case (`tianpan-forum` / `blockeden-forum` on hetzner-prod) is root-caused to a named step, fixed, and pinned by a test that fails against the pre-fix code. The hand-made Ingress objects created as a workaround on 2026-08-08 are removed and the platform recreates them.

## Source + Goal linkage

- **Source:** production bug report 2026-08-08, symptom 4 ("Deploy 'Update Failed'; `<svc>.onbex.co/forum` → 404; service URL was `null`"), triaged 2026-08-08. Split out of `w7/m77` because the three datastore-link symptoms share one root cause and this one does not: `reconcileIngress` (`lego/operator/internal/controller/app_controller.go:1155`) runs unconditionally after the Deployment and is **not** health-gated, so a crashlooping pod should still have received an Ingress.
- **Goal linkage:** deploy-flow correctness (`docs/ADR004-app-deployment.md`) and the public-routing contract (`docs/ADR005-custom-domain.md`, `docs/ADR041-service-addresses.md`). A service that deploys but is unreachable, with no signal saying why, is the failure mode a Render alternative cannot have.
- **Expected outcome:** the class of failure disappears, not just the instance — no ordering in the reconcile can leave a service running but permanently unroutable.
- **Why now:** two live production services were reachable only because an Ingress was hand-written for each, and the underlying condition is still present. It also blocks the last step of `w7/m77/t007`, which cannot retire the hand-made Ingresses until the platform creates them.
- **Render parity task included:** yes — the defect's visible symptom is a wrong `serviceDetails.url`, which is a Render-compatible REST/GraphQL/MCP field.
