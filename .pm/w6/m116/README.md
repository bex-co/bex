# w6 · m116 — A free service never auto-sleeps as created: `idleTTLSeconds: 0` disables auto-sleep while eight surfaces call it "the platform default"

**Worker:** worker6 **Goal:** the free tier's advertised behavior and its actual behavior are the same one — either free services sleep by default (ADR003's `sleep = free`) or the product stops telling users they do, on every surface that says it today **Status:** todo

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Decide what `idleTTLSeconds: 0` means, and write it into ADR003/ADR007                 | 30m | —          |
| t002 | Implement the decision in the operator                                                 | 45m | t001       |
| t003 | Correct every surface that states the contract — 8 sites, one of them the CRD schema   | 45m | t001       |
| t004 | Render parity                                                                          | 20m | t002, t003 |
| t005 | Simplify                                                                               | 20m | t004       |
| t006 | Test coverage                                                                          | 30m | t004       |
| t007 | Closeout                                                                               | 10m | t005, t006 |

## Definition of done

- **A free service created through the product's own create path behaves the way its Settings page says it does.** Create a free web service, leave it idle past the shortest window the Settings select offers (300s), and the observed `phase` matches the selected option's meaning. Today it does not: the select reads **"Platform default"** and the service is still `Running` after **12m30s** (capture below).
- **The word on the control is true.** Whatever t001 decides, the Settings select's label for `0` and the hint under it agree with the operator. If `0` stays "off", the hint "Free services sleep after this idle window, then wake on the next request." must not render as an unconditional statement of fact next to it.
- **`kubectl explain app.spec.idleTTLSeconds` and the MCP tool schema say the same thing as the operator.** Both currently say "0 = controller default" / "0 restores the controller default" against an operator that treats 0 as disabled. `w7/m80` already settled the principle for this exact shape: "an MCP description promising 'restore the platform default /' is the contract an agent reads, so a stale one is a live defect, not a typo."
- **The nonzero path is untouched and still works**, re-verified rather than assumed: setting `idleTTLSeconds: 60` hibernates the service within ~60s, and the next request wakes it with the documented interstitial. Both legs were re-measured live this run (captures below) and must still hold after the change.
- `go test ./lego/operator/...` covers `idleTTLSeconds: 0` explicitly — asserting the decided behavior, not merely that some nonzero value sleeps. The existing tests all use nonzero values, which is why the 0 case has no coverage today.
- If t001 chooses a real default, the effect on **already-running free services** is stated and deliberate: `eden-dash-v3` (free, `Running`, 25 days old, `idleTTLSeconds: 0`) would begin sleeping. That is a live behavior change for existing tenants, not a no-op.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 40th run, 2026-08-27, journey 15 (free-tier sleep → wake). Workspace `tea-d98210cbbpdc73dcrkvg`.

  Every service in the workspace — including both free-plan ones — reads `idleTTLSeconds: 0`, and so does a **freshly created** one, so this is not legacy data:

  ```
  createService(name:"qa-20260827-sleep", plan:"free", …) -> srv-da81t88ueu1c7395ink0
  GET /v1/services/srv-da81t88ueu1c7395ink0  ->  {"idleTTLSeconds":0,"plan":"free"}   (11:25:30Z, phase Building)
  ```

  What the Settings tab shows for that service, read verbatim from the live accessibility tree (`page.locator('main').ariaSnapshot()`):

  ```
  Idle timeout
    Free services sleep after this idle window, then wake on the next request.
    combobox: Platform default
  ```

  **Failing case** — served once at `11:28:14Z` (`curl -sSk` → `200`), then left completely untouched (dashboard/API reads do not stamp `last-active`; only data-plane traffic does):

  ```
  11:29:11Z  phase=Running  idleTTL=0  replicas=1
  11:31:07Z  phase=Running  idleTTL=0  replicas=1
  11:33:02Z  phase=Running  idleTTL=0  replicas=1
  11:34:57Z  phase=Running  idleTTL=0  replicas=1
  11:36:52Z  phase=Running  idleTTL=0  replicas=1
  11:38:48Z  phase=Running  idleTTL=0  replicas=1
  11:40:44Z  phase=Running  idleTTL=0  replicas=1     <- 12m30s idle, 2.5x the shortest offered preset
  ```

  **Control, on the same service, changing only that one field** — via `setIdleTimeout`, the same mutation the Settings control calls:

  ```
  11:40:58Z  mutation setIdleTimeout(idleTTLSeconds:60) -> {"idleTTLSeconds":60}
  11:41:28Z  phase=Running     idleTTL=60
  11:41:59Z  phase=Hibernated  idleTTL=60             <- 61 seconds
  ```

  **Wake, immediately after** (journey 15's own promise, and proof the whole path is healthy):

  ```
  t+1s   503  {"error":"service hibernated","retryAfter":5}
  t+6s   503  {"error":"service hibernated","retryAfter":5}
  t+10s  503  {"error":"service hibernated","retryAfter":5}
  t+15s  200  OK — pushed v3
  ```

  So the mechanism is fully functional and the only thing standing between a free service and the free tier is the value the product ships it with. Fixture deleted; `GET` → 404 and gone from the list.

- **Root cause:** `lego/operator/internal/controller/app_controller.go:1638` — `autoSleepEligible` is `!app.Spec.Suspended && app.Spec.IdleTTLSeconds > 0 && isFreeApp(app)`. There is no controller default: a grep for a default constant across `lego/` returns only `defaultIdleTimeout` in `cmd/pg-sni-proxy/main.go:76` and `cmd/kv-sni-proxy/main.go:62`, both TCP-proxy connection timeouts unrelated to App hibernation, and the CRD field carries `+optional` with **no** `+kubebuilder:default` (`lego/types/v1alpha1/app_types.go:613-615`). The same predicate gates two more behaviors, so `0` does more than skip the timer: `:2098` decides whether the `app.bex.co/last-active` annotation is ever stamped, and `:2031` decides whether the activator is the App's Ingress backend at all — a `0` App is not merely never asleep, it is never on the wake path.
- **The eight sites that assert a default that does not exist:**
  1. `lego/types/v1alpha1/app_types.go:613` — CRD field doc, "0 = controller default"
  2. `lego/operator/config/crd/bases/app.bex.co_apps.yaml:525-526` — the generated schema, so `kubectl explain` repeats it to cluster operators
  3. `lego/backend/internal/apps/service.go:435-436` — `AppView.IdleTTLSeconds`, "0 = the controller default"
  4. `lego/backend/internal/apps/service.go:3042` — `MaxIdleTTLSeconds`, "0 means the controller default"
  5. `lego/backend/internal/apps/service.go:3047` — `SetIdleTTL`, "0 restores the controller default"
  6. `lego/backend/internal/apps/mcp.go:96` — MCP jsonschema, **agent-facing**: "0 restores the controller default"
  7. `lego/backend/internal/apps/render.go:116` — REST field comment, "0 = default"
  8. `dashboard/src/features/services/lib/idle-timeout.ts:6-8` — "0 is the platform default (the operator's own idle window)", plus `idle-timeout-row.tsx:21,24` and the user-visible label `services.idleTimeoutDefault` = **"Platform default"** in `locales/en.ts` (the `zh` catalog carries the same key and needs the same treatment)
- **Goal linkage:** [docs/ADR003-control-plane.md](../../docs/ADR003-control-plane.md):80 makes this load-bearing, not cosmetic — "Idle free apps **hibernate** (`sleep = free`) and **wake on the next request** via the gateway **activator**… a sleeping pod occupies nothing, so the cluster **overcommits well beyond Σ** and Free approaches \$0." With every free App at `0`, `Σ(running pods)` includes every free App forever and that overcommit assumption does not hold. Also [ADR007](../../docs/ADR007-restart-suspend-and-resume.md) (the auto-hibernate/wake design) and [ADR018](../../docs/ADR018-render-parity.md).
- **Expected outcome:** a free service's Settings page and its actual behavior agree, and whichever way t001 decides, no surface claims a default that the operator does not implement.
- **Why now:** the whole mechanism is already built, tested and live — `w6/m47` fixed the Ingress-backend bug that left sleeping free services on Traefik's 404, `w6/m94` fixed both hibernate/wake routing races, and this run re-measured the full nonzero path working end to end (hibernate in 61s, wake in 15s with the documented interstitial). The activator, the `Sleeping` badge, `services.statusSleepingHint`, and the `service_hibernated`/wake event types all exist and are all unreachable for a service created the normal way. The cheapest moment to settle what `0` means is before more surfaces copy the phrase from a neighbour.
- **Render parity:** included (t004). Render's free tier spins down after a **fixed** idle window with no user knob, and it is always on — so bex diverges twice: it adds a knob (a deliberate, documented extension), and it ships that knob defaulted to a value that means "never", where Render's equivalent is "always". The knob is fine; the default is the divergence, and t004 must record which half is deliberate.
- **Blast radius:** `autoSleepEligible` has exactly 3 callers, all in `app_controller.go` — `shouldAutoHibernate:1642`, the Ingress-backend choice `:2031`, and the `last-active` stamp `:2098` — so a change to what `0` means moves all three together and each needs its own regression test, including `:2031`, which is correct today only because `0` never reaches it. Only App CRs carry `idleTTLSeconds`; Postgres and Key Value have `suspended` but no idle window. `isFreeApp` (`:1211`) is `Tier == "" || Tier == tierFree`, so untiered bare-CR Apps are in scope too — matching the dashboard's `planSleeps(plan === null || plan === "free")`.
- **Checked and NOT a sibling defect:** a free `background_worker` cannot be woken by a request, so an eligible worker would sleep forever. `desiredReplicas` (`:1984`) already excludes it — `autoHibernating = !worker && …` — with the reason stated in the function's own comment ("A worker never auto-hibernates — it has no Ingress, so a request could never wake it"). Verified by reading the call site, not assumed from the predicate.
- **Adjacent classes:** n/a — this is a default value, not an error taxonomy. The neighbouring **state** that must not move is explicit suspension: `spec.suspended` is a separate path (`hibernated()` is called from `:1867` for auto and `:2773` for suspend), and ADR007's suspend/auto-sleep precedence must survive t002 unchanged.
- **Unverified this run:** (1) whether `eden-dash-v3` (the workspace's other free service, `Running` 25 days at `idleTTLSeconds: 0`) has ever hibernated — its history was not queried, and it was deliberately not touched, being a non-QA production service; (2) whether any bare-CR (`Tier == ""`) App exists in production, so the untiered branch is reasoned from `isFreeApp` rather than observed; (3) the `zh` locale string, which carries the same key but was not rendered; (4) whether `BEX_ACTIVATOR_SERVICE` is set on the production operator — it is set in `lego/operator/config/manager/manager.yaml:248` and the activator Deployment exists at `config/activator/deployment.yaml`, and the 61s hibernate + 15s wake prove the activator is live and serving, but the deployed env var itself was not read.
