# w7 · m78 — A failed reconcile step must not strand a service with no Service and no Ingress

**Worker:** worker7 **Goal:** _(original)_ a web service that reaches the Deployment stage always gets its ClusterIP Service and Ingress. **Status:** **FOLDED into `w7/m79`** 2026-08-09 — the premise was disproven by t001.

## Disposition

**t001 disproved the milestone.** There is no aborting step. The reconcile completes successfully and creates no Ingress **on purpose**: production runs with `BEX_BASE_DOMAIN` unset (a deliberate security decision — `onbex.co` is a registrable domain, so tenant JavaScript could set parent cookies a sibling tenant receives), so a service with no custom domain of its own has zero effective hosts and `reconcileIngress` takes its documented `len(hosts) == 0` branch.

Established by deterministic reproduction rather than inference — `internal/controller/tenant_namespace_routing_envtest_test.go` reconciles the exact production shape under both configurations and pins all three outcomes (platform host routes / production configuration does not / a custom domain routes either way). Full evidence in [`done/t001.md`](done/t001.md).

Every reported observation fits, and two dissolve entirely: "annotating the CRs forced no re-reconcile" is the App controller's `generationOrDeletionPredicate` filtering an annotation-only update, and "Update Failed" is the health-check failure from w7/m77's root cause. `serviceDetails.url: null` has its own independent explanation — the operator writes `status.URL` only on the Running path, and bex-api's `pendingPublicURL` fallback depends on bex-api's own (also unset) `BEX_BASE_DOMAIN`.

**What survives is one missing signal, not a fix to the routing path.** Two states, neither visible anywhere:

1. A web service created today gets no public URL unless the user attaches a custom domain — with no indication at create, on the service, or in the deploy.
2. Removing `BEX_BASE_DOMAIN` silently deleted the public route of every service that had only a platform host. Deleting is the correct security behavior (that was `w7/m54`'s point); doing it without a trace is not.

Both reduce to _expose is set, yet no public host is derivable_ — which is already `w7/m79/t003`'s subject. Building the same condition in two milestones would be the duplication `.pm/DO_NOT_DO.md` warns against, so m78 folds into m79, which was sharpened with this finding rather than left to rediscover it.

The reproduction test is kept: it is the regression guard for the routing path either way, and it documents the deliberate zero-host behavior so the next reader does not mistake it for a bug a second time.

## Tasks (in order)

| id   | title                                                          | est | depends_on               |
| ---- | -------------------------------------------------------------- | --- | ------------------------ |
| t001 | Live evidence: identify which reconcile step actually aborted — **DONE** | 30m | —                        |
| t002 | Fix the abort so routing cannot be stranded by an earlier step — **SUPERSEDED** (no abort exists) | 45m | w7/m78/t001              |
| t003 | Render parity: `serviceDetails.url` + routing surfaces — **MOVED to w7/m79** | 25m | w7/m78/t002              |
| t004 | Simplify the code this milestone changed — **N/A** (no production code changed) | 25m | w7/m78/t003              |
| t005 | Test coverage: reconcile-abort invariants — **DONE as the t001 reproduction** | 35m | w7/m78/t003              |
| t006 | Closeout — **DONE** (folded)                                    | 15m | w7/m78/t004, w7/m78/t005 |

## Definition of done

_Superseded._ The original DoD ("the specific production case is root-caused to a named step, fixed, and pinned by a test that fails against the pre-fix code") cannot be met as written, because no step fails. It is replaced by what t001 actually had to establish, all of which holds:

- the production behavior is root-caused, with the cause named and cited to the manifests that set it;
- the behavior is pinned by a deterministic reproduction covering both configurations;
- the residual gap is handed to the milestone that owns it, with its premise corrected rather than restated.

**Not carried forward:** the hand-made production Ingress objects from 2026-08-08 stay in place. `w7/m77/t007` deferred their removal to this milestone on the assumption that a fix would recreate them — it will not. They are now the *only* thing routing those two forums, and removing them requires attaching real custom domains first. Recorded in `w7/m77/t007` and `.pm/w7/026.md`.

## Source + Goal linkage

- **Source:** production bug report 2026-08-08, symptom 4, triaged the same day; disproven by `t001` on 2026-08-09.
- **Goal linkage:** deploy-flow correctness ([ADR004](../../../docs/ADR004-app-deployment.md)) and the public-routing contract ([ADR005](../../../docs/ADR005-custom-domain.md), [ADR041](../../../docs/ADR041-service-addresses.md)).
- **Outcome:** the routing path is proven correct and is now regression-guarded for the tenant-namespace shape it had never been tested in; the real gap is visibility, owned by `w7/m79`.
- **Why it stayed open long enough to matter:** the symptom (no Ingress, null URL) is identical whether the cause is a failure or a deliberate configuration, because neither produces a signal. That is the gap `w7/m79` closes.
