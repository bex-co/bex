# w7 · m81 — livenessProbe: restart a wedged instance after 60s (Render steady-state stage 2)

**Worker:** worker7 **Goal:** a wedged instance — process alive, event loop blocked, deadlocked — is restarted within ~60s of consecutive health-check failures, matching Render's documented steady-state behavior, instead of staying broken until someone notices. **Status:** done

## Tasks (in order)

| id   | title                                                                                                         | est | depends_on                 |
| ---- | ------------------------------------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Decide and record the liveness contract (unconditional parity vs explicit-path opt-in) — **DONE**             | 30m | —                          |
| t002 | Operator: add the `livenessProbe` per the t001 contract (5s budget, 10s period, 6 failures) — **DONE**        | 40m | w7/m81/t001                |
| t003 | Guidance + docs: cheap health endpoint, ADR004 restart stage, retire the ledger divergence — **DONE**         | 30m | w7/m81/t002                |
| t004 | Render parity sweep: health-check descriptions across REST/GraphQL/MCP + dashboard — **DONE**                 | 30m | w7/m81/t003                |
| t005 | Simplify the code this milestone changed — **DONE**                                                           | 30m | w7/m81/t004                |
| t006 | Test coverage for the shipped behavior — **DONE**                                                             | 40m | w7/m81/t004                |
| t007 | Closeout — **DONE**                                                                                           | 15m | w7/m81/t005, w7/m81/t006   |

## Outcome

Shipped 2026-08-17. **t001 chose unconditional parity**: every non-worker service now carries a `livenessProbe` on the shared health handler (`timeoutSeconds: 5`, `periodSeconds: 10`, `failureThreshold: 6` = Render's 60s restart), TCP default included. The rejected alternative — liveness only on an explicit `healthCheckPath` — lost because the spiral hazard does not exist in TCP mode (a kernel-level connect never invokes the tenant's SSR route) and in HTTP mode it is Render's own documented behavior; opt-in would have denied TCP-mode services a restart Render gives them while adding a mode distinction Render users never learn. The hazard is bounded by m80's `startupProbe` (kubelet suspends liveness during boot), and that coexistence is pinned by tests in both `deployment_projection_test.go` and the envtest suite (`app_controller_test.go`), which previously pinned the *absence* of liveness — both old pins inverted.

The t004 sweep caught one real stale contract m80 missed: `AppView.HealthCheckPath`'s doc comment in `internal/apps/service.go` still claimed "Empty means the default '/'" — the pre-m80 one-way coercion. Fixed, along with the MCP `set_health_check_path` description + jsonschema, the GraphQL resolver comment, both dashboard locales, and `types.ts` — every surface now states the 60s restart and the cheap-endpoint guidance.

t005's `/simplify` (three reviewers) found the diff itself clean apart from two consistency fixes it applied (the derived `livenessRestartWindowSeconds / healthCheckPeriodSeconds` threshold replacing a literal 6, matching the startup threshold's idiom; the `healthCheckHandler` "both probes" comment) — and surfaced one **pre-existing** defect on the same seam: the Deployment `CreateOrUpdate` PUTs every reconcile and self-retriggers through `Owns(..., ResourceVersionChangedPredicate{})`, because the mutate output can never DeepEqual the server-defaulted object (the w9/m57 Service bug, unspotted on the Deployment path). Filed as inbox `w7/028.md`; m81 neither introduces nor worsens it.

Verification: `make test` (operator, incl. envtest) green, `go test ./internal/apps/...` green, dashboard settings-page vitest green; anti-tautology proven — deleting the probe turns all three new assertions red. Not verified in production: the code is uncommitted, per the repo rule that only `/ship` commits.

## Definition of done

All four hold, each proven by a test or a checked-in record rather than by inspection:

1. **A wedged instance is restarted, not just pulled from rotation.** For a service in the contract's liveness scope, the projected Deployment carries a `livenessProbe` sharing the health handler with `timeoutSeconds: 5`, `periodSeconds: 10`, `failureThreshold: 6` — Render's "after 60 seconds of consecutive failures, we restart the instance" — pinned by a projection test.
2. **Boot is never killed by liveness.** The `startupProbe` from m80/t003 stays alongside liveness with its 15-minute budget intact (kubelet suspends liveness while startup runs — the hard precondition this milestone waited for); a test asserts both probes coexist and the m80 timer invariants do not regress.
3. **The contract is decided, not defaulted.** Whichever scope t001 picks — Render-parity unconditional (liveness on the TCP default too) or explicit-`healthCheckPath`-only (a deliberate, argued divergence from Render) — is recorded in the `deployment_projection.go` comment and ADR004 with the rejected alternative and why, and the parity ledger's "known divergence" entry (written by m80/t006) is retired or rewritten to match what shipped.
4. **Out-of-scope shapes are byte-identical.** Workers (`p.worker`, no health check) get no probes; a service outside the chosen liveness scope keeps today's exact pod template.

## Source + Goal linkage

- **Source:** inbox note [`w7/027.md`](../027.md), split out of [m80](../m80/README.md)'s Out of scope on 2026-08-10 with an explicit "promote once m80 closes" instruction. Render's two-stage steady-state from [Health Checks](https://render.com/docs/health-checks): 15s of failures → out of rotation (shipped — the readiness probe), 60s → **restart** (this milestone).
- **Goal linkage:** Render parity ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)) and the deploy contract in [docs/ADR004-app-deployment.md](../../../docs/ADR004-app-deployment.md). This was the last gap in m80's health-check milestone — its t006 recorded the missing restart stage as a known divergence so it would stay visible rather than silently absent; the ledger row now records the restart stage as shipped.
- **Expected outcome:** a wedged instance self-heals within about a minute. Before, a process that was alive but not serving stayed broken indefinitely — readiness kept it out of rotation, so the service was down, and nothing ever recovered it.
- **Why now:** the note's hard precondition was met — m80 closed 2026-08-10 with the `startupProbe` owning boot, so liveness cannot kill a slow-booting pod (kubelet suspends it while startup runs).
- **Render parity task included** (t004): the change is tenant-observable — instances now restart — and m80's lesson stands: every surface that *describes* health checks (MCP tool descriptions, dashboard copy, locale strings) is part of the contract an agent or user reads, so a stale description is a live defect. It earned its keep — see Outcome.

## Out of scope

- **Changing readiness/startup semantics.** m80 settled those; this milestone added the third probe and touched the existing two only where a test proved an interaction (none did).
- **Per-service liveness control in the API** (a new `healthCheckPath`-style field). t001's chosen contract needs no new field.
- **Backfilling a decision for existing App CRs beyond what the Deployment projection naturally rolls out.** The projection rebuilds the pod template each reconcile, so the probe appears on the next roll; no migration job.
