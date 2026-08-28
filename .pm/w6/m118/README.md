# w6 · m118 — Nothing caps instance count by plan: a free service runs up to 100 pods at `$0.00/second`, under UI copy promising it is "billed accordingly"

**Worker:** worker6 **Goal:** free-plan replica policy is decided and enforced — or deliberately left open with the UI no longer claiming those instances are billed **Status:** implemented and deployed (`9917191f` is contained in the production image pinned by `71fe9660`) — code and all local gates are green; t005/t008 remain open only for the authenticated production over-cap parity and `/instances` re-run. See the Resolution section below.

## Tasks (in order)

| id   | title                                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Gather the Render parity evidence `w2/m56` asked for, and decide the policy           | 45m | —          | — **DONE**
| t002 | Fix the Scaling page's "billed accordingly" claim — true regardless of t001           | 30m | —          | — **DONE**
| t003 | Enforce the decided cap in the API (`Scale`, create, autoscaling min/max)              | 45m | t001       | — **DONE**
| t004 | Operator defense-in-depth, mirroring `w6/m46/t002`                                     | 45m | t001, t003 | — **DONE**
| t005 | Render parity                                                                          | 20m | t003, t004 |
| t006 | Simplify                                                                               | 20m | t005       | — **DONE**
| t007 | Test coverage                                                                          | 30m | t005       | — **DONE**
| t008 | Closeout                                                                               | 10m | t006, t007 |

## Definition of done

- **A free-plan service's instance count matches the decided policy on every write path, and the operator will not run more than the policy allows even if a CR says otherwise.** Today `scaleService(numInstances: 3)` on a free web service returns `replicas: 3` and three pods genuinely run (capture below). The DoD is not "scaling is refused" — `t001` may decide free-plan scaling is intentional — it is that the policy is written down and the same on all four surfaces.
- **The Scaling page does not tell a free-plan user their instances are billed.** It currently reads _"All instances use the same instance type and are billed accordingly."_ over a slider that runs to 100, on a tier whose rate is literally `usdPerSecond: 0.0` (`lego/backend/internal/pricing/pricing.yaml:40-41`). This bullet holds independently of `t001` — the sentence is false on free either way.
- **`GET /v1/services/{id}/instances` and `replicas` continue to agree with reality.** They already do — that half of journey 6's promise ("the applied number is the number that runs") passed this run and must not regress while a cap is added.
- If a cap is introduced, the refusal names the plan and the limit, in the shape `errNoPublicIngress` established for the sibling refusal (`a private_service has no ingress and cannot have custom domains`) — not a bare 400.
- `cd lego/backend && go test ./internal/apps/...` and `make test` (operator) each cover a free-plan service at the boundary and one past it, red before the change.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 47th run, 2026-08-27, journey 6 (Scaling). Fixture `qa-20260827-scale` (`srv-da84u7nm2e9c73ft5o60`, free web service, deleted at end of run — scaled back to 1 within ~90 seconds of the observation).

  ```
  mutation { scaleService(id:"srv-da84u7nm2e9c73ft5o60", numInstances:3){ id replicas plan } }
  => {"data":{"scaleService":{"id":"srv-da84u7nm2e9c73ft5o60","plan":"free","replicas":3}}}

  GET /v1/services/srv-da84u7nm2e9c73ft5o60            -> replicas: 3, serviceDetails.numInstances: 3, plan: "free"
  GET /v1/services/srv-da84u7nm2e9c73ft5o60/instances  (14:55:59Z, phase Running)
    -> 3 instances: …6sbehds2, …8ttcqua7, …a3jlvf0o
  ```

  Three real pods, on the free tier, from one mutation. What the dashboard offers for that same service, read from the live accessibility tree at `/services/…/scaling`:

  ```
  Scaling — Run multiple instances that are automatically load balanced.
            All instances use the same instance type and are billed accordingly.
    Instances   1 [--------slider--------] 100   [1]   [Save Changes]
  ```

- **Root cause — three layers, no gate in any of them:**
  - `lego/backend/internal/apps/service.go:3010-3038` (`Scale`) validates only `replicas < 1 || replicas > store.MaxReplicas` and writes. It authorizes, runs `RequireBillingMutation`, and never consults the plan.
  - `lego/operator/internal/controller/app_controller.go:3560-3568` (`effectiveReplicas`) returns `spec.Replicas` unchanged, and `clampReplicas` (`:3574-3582`) bounds only `[0, MaxReplicas]`. A grep of `app_controller.go` for a plan-aware replica bound (`tierFree`/`isFreeApp` near `replica`/`scale`) returns nothing.
  - `lego/types/tiers/tiers.go:45-62` (`ComputeTier`) has no instance-count field at all — the plan model carries CPU, Memory and EphemeralStorage only, so there is no cap to enforce even if a caller wanted to read one.

  `MaxReplicas` is `100` (`lego/types/v1alpha1/app_types.go:756`), which is what the dashboard slider's upper bound reflects.

- **This was deliberately deferred, not missed.** `.pm/w2/done/m56/README.md:30` lists under **Explicitly excluded**: _"…or silently changing free-plan replica policy **without separate parity evidence**."_ So the policy is a known open question with a stated bar for changing it. This milestone exists to supply that evidence and settle it — `t001` is the decision, and the remaining tasks implement whatever it decides. Filing it as a straightforward bug would misrepresent a decision the team consciously held open.
- **Goal linkage:** [ADR030](../../docs/ADR030-pricing.md) (pricing) and [ADR018](../../docs/ADR018-render-parity.md) row 68 (manual scale), which documents scale as a parity feature across all four surfaces and records no plan gate either way. Also [ADR003](../../docs/ADR003-control-plane.md), whose free-tier economics assume a free App is cheap to host.
- **Expected outcome:** the free plan's replica policy is stated once and enforced everywhere, and no surface tells a free-plan user something untrue about what they are paying for.
- **Why now:** it compounds an already-open finding. `w6/m110` (open) established that App compute is **never metered at all** — `usage/service.go:808` meters against a name no pod ever has — so today a free service scaled to 100 produces neither a free-tier charge (rate `0.0`) nor a compute meter reading (the m110 defect). The two together mean instance count is currently unbilled and unbounded on every plan; fixing m110 alone would start billing paid plans while leaving free uncapped, which is precisely when the policy question becomes load-bearing.
- **Render parity:** included (t005), and it is also the *input* to t001. bex's own parity ledger does not record what Render does here, and this hunt did not fetch Render's docs — so the Render half of `w2/m56`'s "separate parity evidence" is **not** supplied by this filing and is t001's first step, not an assumption to build on.
- **Blast radius:** `Scale` is one verb reached from REST `POST …/scale`, GraphQL `scaleService`, and MCP `scale_service` (ADR018 row 68) — one implementation, three surfaces. A cap must also cover the two adjacent writers that set replicas without going through `Scale`: create (`specFromCreate`'s replica handling) and autoscaling `minInstances`/`maxInstances` (ADR018 row 69), or the cap is reachable around. `effectiveReplicas` has its own callers in the operator and is the defense-in-depth point — the same shape `w6/m46/t002` used when it hard-gated Ingress creation for `private_service` regardless of `spec.Expose`.
- **Adjacent classes:** if t001 introduces a refusal, place its neighbours — a paid plan at its own ceiling, a plan **downgrade** while replicas exceed the new plan's cap (does it refuse the downgrade, or clamp replicas?), and an autoscaler whose `maxInstances` exceeds the cap. The downgrade case is the one most likely to be missed and the one that can strand a running service.
- **Unverified this run:** (1) Render's actual free-tier instance policy — reasoned from the "billed accordingly" copy and the paid-scaling convention, **not** checked against Render's docs or API; t001 owns it. (2) Whether the free-plan cap should be 1 or something else — a product decision, not observed. (3) Whether `background_worker`, `cron_job`, `private_service` and `static_site` behave identically — only `web_service` was scaled live; they share `Scale` and `effectiveReplicas` by inspection, but a cron job has no replicas concept and a static site has no pods, so the family needs enumerating rather than assuming. (4) Autoscaling on a free plan was not exercised at all.

## Resolution (2026-08-27)

Triaged the filing against the code: **every claim verified accurate** — `Scale` (`apps/service.go`) validated only `1..MaxReplicas`; `effectiveReplicas`/`clampReplicas` bounded only `[0,100]`; `ComputeTier` (`types/tiers/tiers.go`) carried no instance field; free compute is `usdPerSecond: 0.0`; and the dashboard copy "…billed accordingly" shipped over a 1–100 slider. Not noise — a real, well-scoped defect. Fixed rather than deleted.

**t001 — parity evidence + decision (settled).** Render's free instance types **do not support horizontal scaling** at all; multiple instances require a paid instance type (Starter+), and the "billed accordingly" per-instance copy is Render's own but appears only on **paid** services (render.com/docs/scaling, /docs/free — captured in `docs/render-artifacts/free-tier-scaling.md`). Decision: **the free plan caps at 1 running instance** — parity with Render, and the zero rate makes N free instances N× capacity for \$0. This **reverses** ADR018 row 68's earlier w1/m81 "deliberate divergence" (which allowed free to scale, but stated no cost reasoning and predates `w6/m110`'s finding that free compute is also unmetered). The cap lives in the reviewed tier catalog (`tiers.yaml` compute `free: maxInstances: 1`; paid tiers omit → no plan cap, only `MaxReplicas=100`), read via `tiers.Compute.InstanceCap`. Downgrade decided: **refuse** a plan change that would leave the service above the target cap ("scale down first"), mirroring the maintenance-mode guard — never silently shrink a running service.

**t002 — UI copy (done).** `services.scalingManualDescription` kept for paid; new `services.scalingManualDescriptionFree` ("Free services run a single instance. Upgrade to a paid plan to run multiple load-balanced instances.") shown for free, in `en`+`zh`; the manual-scaling slider caps free at 1.

**t003 — API enforcement (done).** `checkInstanceCap` gates `Scale`, create (`specFromCreate`), and autoscaling `min`/`maxInstances`; `planDowngradeError` gates `SetPlan`/`PreviewSetPlan`. Refusals name the plan and the limit (the `errNoPublicIngress` shape). `Scale` is the shared REST/GraphQL/MCP verb, so all three inherit it.

**t004 — operator defense-in-depth (done).** `clampReplicas(app, n)` now reads the plan cap from the tier catalog, so a hand-applied CR or projector bug can never run more pods than the plan allows. Three callers updated (`app_controller.go` ×2, `deployment_projection.go` ×1). An untiered/bare CR keeps only `MaxReplicas` (matching how it also gets no tier resource limits).

**t005 — parity ledger (docs done; live re-run pending).** ADR030 records the policy + cost reasoning; ADR018 rows 68 & 69 updated (row 68's w1/m81 divergence superseded). The fix is deployed: `9917191f` is an ancestor of the production image pinned by `71fe9660`. **Remaining, credential-gated:** attempt an over-cap scale through REST/GraphQL/MCP on a live free service and re-run the scale→`replicas`→`/instances` sequence to confirm agreement.

**t006/t007 — simplify + tests (done).** New coverage red-before-green: `tiers_test.go` (`TestComputeInstanceCap`), backend `instance_cap_test.go` (Scale/create/autoscaling/downgrade boundary + the 5-type family), operator `deployment_projection_test.go` (`TestApplyDeploymentSpecCapsFreePlanReplicas`). Four pre-existing fixtures that scaled a free/untiered service past 1 were corrected (maintenance-mode ×2, blueprint worker, stack autoscaling). Code is minimal and lint-clean (0 issues on the three touched packages); no separate `/simplify` agent pass was run.

**Gates green locally:** `go test ./tiers/…`, backend `go test ./internal/apps/`, operator `make test` (envtest, 81% controller), `golangci-lint` (0 issues) on all three modules, dashboard `yarn typecheck`+`yarn test`. Left `[ ]` open pending the t005 live re-run on the next deploy.
