# w7 · m80 — Render-parity health checks: probe semantics and the three disagreeing deploy timers

**Worker:** worker7 **Goal:** a service is judged healthy the way Render judges it — TCP by default, a 5-second budget, and 15 minutes to boot — so an ordinary app that Render deploys without complaint also deploys on bex. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on                     |
| ---- | --------------------------------------------------------------------------- | --- | ------------------------------ |
| t001 | Readiness probe: 1s → Render's 5s timeout — **DONE**                         | 25m | —                              |
| t002 | Align the three deploy timers on Render's 15 minutes — **DONE** (premise corrected: not equal, observer sits above) | 30m | —                              |
| t003 | `startupProbe`: give boot its own 15-minute budget, decoupled from steady state — **DONE** | 35m | w7/m80/t001, w7/m80/t002       |
| t004 | Drop the CRD `/` default — TCP probe when `healthCheckPath` is unset — **DONE** | 50m | w7/m80/t003                    |
| t005 | Correct ADR004, the parity ledger, and the CRD field comment — **DONE**      | 30m | w7/m80/t004                    |
| t006 | Render parity sweep: health-check surface across REST/GraphQL/MCP + dashboard — **DONE** (found + fixed a blocker on all four surfaces) | 30m | w7/m80/t005                    |
| t007 | Simplify the code this milestone changed — **DONE**                          | 30m | w7/m80/t006                    |
| t008 | Test coverage for the shipped behavior — **DONE**                            | 40m | w7/m80/t006                    |
| t009 | Closeout — **DONE**                                                          | 15m | w7/m80/t007, w7/m80/t008       |

## Outcome

Shipped 2026-08-10. Both premises the milestone was written on survived; two things it did not anticipate did not.

**t006 found the change was inert.** Every one of the four surfaces coerced an empty path back to `"/"` — `SetHealthCheckPath` in the service layer (`service.go`), and the dashboard row independently (`value || "/"`). So after t004 made TCP the default, no existing service could ever reach it and no caller could express "unset": the field was one-way. The milestone's headline fix would have shipped unreachable. Fixed in the service verb (which REST, GraphQL, and MCP all funnel through) and in the dashboard row, with the MCP tool description and both locale strings corrected — an MCP description promising "restore the platform default /" is the contract an agent reads, so a stale one is a live defect, not a typo.

**t002's premise was wrong and its own new test caught it.** See the corrected DoD item 3 above: the invariant is not three equal timers but two mechanism timers equal at 15m with the control-plane observer strictly above them at 18m.

Two incidental findings, neither introduced here: `make lint` was already failing on the clean tree (18 issues, none in files this milestone touches — this diff reduces it to 16 by removing two repeated `"/"` literals), and `config/crd/bases/app.bex.co_databases.yaml` was stale against its types (`DeletedUsers`, added by `eab1799d`, never regenerated). Codegen corrected the latter; production's CRD already carried the field, so nothing was broken by it.

Not verified in production: the code is uncommitted, per the repo rule that only `/ship` commits. The observable proof is in test, as the DoD requires.

## Definition of done

All four hold, each proven by a test rather than by inspection:

1. **A service whose root returns 404 deploys.** With `healthCheckPath` unset, the operator writes a `TCPSocket` probe, so an API-only service (whose `/` is a legitimate 404) reaches Ready. Today it can never become Ready — Kubernetes scores an HTTP probe healthy only on `200 ≤ code < 400` — so this class of service is undeployable on bex while deploying fine on Render.
2. **A slow page no longer flaps.** Every probe carries `timeoutSeconds: 5`. A service whose health path answers in ~2.5s stays continuously Ready instead of cycling NotReady.
3. **A slow boot still deploys.** A service that needs ~8 minutes before first serving completes its deploy. The three timers that bound a rollout stand in the right relationship rather than the accidental one, asserted by tests on both sides of the module boundary: the `startupProbe` budget and `Deployment.progressDeadlineSeconds` are the mechanism and both derive from one constant at Render's 15 minutes, while bex-api's `DeployGateTimeout` **observes** that mechanism and is therefore strictly longer (18m).

   > **Corrected during t002.** This item first read "all three read 15 minutes and are asserted equal", which was wrong and the new test caught it. The file's existing gates already encode the real rule — a control-plane budget must outlast the mechanism it watches (build Jobs 30m → gate 35m, pre-deploy 10m → gate 12m) — because an equal gate races the mechanism's own specific verdict and can report the generic "did not become healthy within the health-gate window" instead, re-creating exactly the vagueness w7/m79 removed.
4. **Explicit stays strict.** Setting `healthCheckPath` still produces an HTTP probe requiring 2xx/3xx, unchanged apart from the 5s budget.

## Source + Goal linkage

- **Source:** designed 2026-08-10 while diagnosing why `beancount-cms-v2` (`srv-d9bj8s3eg85c7390eb9g`) would not deploy. Render's behavior captured from [Health Checks](https://render.com/docs/health-checks) and [Deploying on Render](https://render.com/docs/zero-downtime-deploys): TCP socket probe by default, 2xx/3xx-within-5s once a path is set, deploy cancelled after 15 minutes, and — after deploy — 15s of failures removes an instance from rotation while 60s restarts it. Present state: `lego/operator/internal/controller/deployment_projection.go:114` sets `httpGet` and nothing else, inheriting `timeoutSeconds: 1`; `lego/types/v1alpha1/app_types.go:355` pins `+kubebuilder:default=/`; `lego/backend/internal/store/reconciler.go:72` sets `defaultDeployGateTimeout = 3 * time.Minute`.
- **Goal linkage:** Render parity ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)) and the deploy contract in [docs/ADR004-app-deployment.md](../../../docs/ADR004-app-deployment.md). Parity here is not cosmetic: bex is stricter than Render on two independent axes at once, and the intersection makes ordinary apps undeployable.
- **Expected outcome:** the health check stops being a hazard users must discover. An app that Render accepts deploys on bex unchanged; a slow-rendering page stops flapping; a slow boot stops being killed at 3 minutes by a timer nobody chose.
- **Why now:** it is actively breaking production. `beancount-cms-v2` flaps NotReady on a live pod (`/` serves in 1.3–2.9s against a 1s budget), and deploy generations 67 and 77 both closed on the 3-minute gate. Cost is concentrated in defaults nobody set deliberately — the `1s` and the `600s` are Kubernetes' defaults, inherited rather than decided.
- **Complements [w7/m79](../done/m79/README.md), does not duplicate it.** m79 made the failure *message* honest, replacing the literal "did not become healthy within the health-gate window" with the diagnosis the platform already held. m80 addresses why that window is reached: the probe and the timers. m79 fixed the reporting; this fixes the cause.
- **Render parity task included** (t006) because the change is user-facing: `healthCheckPath` semantics shift for services that leave it unset, and the field is exposed on REST, GraphQL, MCP (`set_health_check_path`), and the dashboard's Health & Alerts section.

## Out of scope

- **`livenessProbe` (Render restarts an instance after 60s of consecutive failures).** Deliberately deferred to inbox note [`w7/027.md`](../027.md) rather than scheduled here. Adding liveness on a tenant's health path lets a service that merely slows under load be restarted, and a cold start makes the next probe slower still — a restart spiral. It only becomes safe once t003's `startupProbe` lands, and it wants user-facing guidance to point `healthCheckPath` at a cheap endpoint first. Parity is knowingly incomplete until that note is promoted; t006 records the divergence in the ledger rather than leaving it silent.
- **Backfilling existing App CRs.** The API server materialized `healthCheckPath: "/"` into every App already stored, so t004 changes new services only. Silently rewriting the health semantics of running production services is a larger, riskier decision than this milestone should make on its own; t005 documents how an owner opts in by clearing the field.
