# w4 · m185 — Apply environment isolation when its label changes

**Worker:** worker4 **Goal:** saved environment boundaries promptly control new private connections without waiting for an idle timer or manual deploy **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Admit App isolation-label transitions without broadening metadata events — **DONE** | 40m | — |
| t002 | Converge operational network state on serving and held releases — **DONE** | 60m | t001 |
| t003 | Audit the shared callers and original ACL enforcement guarantees — **DONE** | 70m | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout | 15m | t005, t006 |

## Definition of done

- Recreate the finding's two owned Free BusyBox web fixtures with unchanged default environment IP rules. After both are Live, enable A isolation and move B outside: within ordinary asynchronous convergence new CGI connections return `BLOCKED` without manual deploy, restart, idle expiry or another spec edit. The label-only watch transition, owned policy and current serving labels converge together; REST/GraphQL/UI retain A=true and the correct membership.
- Disable A isolation without changing IP rules: new CGI connections return A's marker without forcing reconcile, the App/serving template/current pods shed isolation and the owned policy is removed. A fresh desktop and narrow-mobile settings reload shows false.
- With both services in isolated A after enforcement settles, new private connections still return A's marker; moving the client outside blocks them. Both public root URLs keep returning their own markers and HTTP200 in settled states. API scope/id conversions and the selected runtime/release remain correct. No user deploy is minted for a boundary change and staged Save-only configuration never starts serving.
- Re-run the original w6/m19 protection and Postgres/KeyValue IP-layer guarantees in the existing suites, documenting their live-unverified status and coordinating w4/m176's separate guard work. These were not live claims in this sweep. Other resource families and held/suspended/hibernated/failed paths are verified work from t003, not evidence already obtained here.
- Watch-driven manager coverage passes with a long Free idle deadline; removing label admission makes it fail. Appropriate checks pass, and live replay removes every owned resource/artifact and revokes only its session before closeout.

## Source + Goal linkage

- **Source:** [cycle-15 finding](finding.md), infinite `$qa-find-bugs w4` on 2026-10-08 using privately supplied muse.env credentials. This is a regression of w6/done/m19's network enforcement guarantee; its complete DoD review and unverified siblings are in the finding. No open duplicate or anti-goal applies; main f34b4bed1 still filters the event.
- **Goal linkage:** ADR008 reliable hosting; ADR032 environment ACL enforcement; ADR043 namespace isolation. Placement in w4 is the user's scheduling instruction.
- **Expected outcome:** a saved boundary controls new private connections while same-environment/private and public access remain correct, independently of Free service idle scheduling.
- **Why now:** production reported isolation on while new cross-environment connections succeeded for minutes; off could retain denial. Persisting the desired flag is not enforcement, and tests that directly invoke Reconcile concealed the dropped event.
- **Scope / sizing:** seven tasks, 4h40m; App-specific watch and operational network convergence plus shared-caller/old-DoD verification. Filing only; this hunt implemented no product fix. No paid fixtures, foreign workspace changes, generic metadata admission or API/auth changes.
- **Render parity:** included because this repairs a tenant-facing control across the dashboard and shared REST/GraphQL/MCP semantics, with Render's new-connection versus already-open-connection distinction retained.

## Progress (2026-10-08)

t001–t006 done:

- **t001:** `generationOrDeletionPredicate.Update` also admits a change to the App's `app.bex.co/network-isolation` label (add, remove, A→B). Status-only, unrelated label/annotation and Save-only notification updates stay filtered; `saved_configuration_test.go:568` still passes.
- **t002:** `reconcileNetworkPolicy` moved into `convergeSharedChildren`, so the owned protected policy converges on every pass, held and prior-release passes included; the duplicate ordinary-path call was removed. `convergeServingRoute` syncs the served template's isolation label (`syncIsolationLabel`) in the same patch as its replica count, so a held release's pods pick it up without applying pending config. Pods are relabeled by a readiness-gated rollout, not patched in place: the operator's RBAC has no pod `patch`, and granting it would also allow image rewrites. No App generation, user deploy or release identity is minted.
- **t003 audit:** cron pods lacked the isolation label entirely, so an isolated cron could not reach its own environment's services. `reconcileCronJob` now adds it. `restoreServedTemplate` returns early on the revision label, so the label sync cannot flap against a restored record. The five `applyAppEnvironmentLabels` callers all write only this label and now reach the operator. Database/KeyValue IP layers are separate and unchanged.
- **t004:** ADR032 records the rule.
- **t005/t006 tests:** a manager-driven envtest settles an App to Running, then edits only the label. The policy and template converge within 10 s and are removed on clear, with the App generation unchanged. It **fails with label admission disabled** (times out). `TestIsolatedCronPodsCarryTheIsolationLabel` covers the cron label. Operator `make test` and `make lint` pass.

Deployed 2026-10-08: deploy run for `0a754a260` succeeded, and production `bex-controller-manager` runs the pinned `bex-operator@sha256:0482219b…`. The t007 live replay could not start: `bash scripts/qa-login.sh` exited 2 (`QA_EMAIL`/`QA_PASSWORD` unset in both `.env` and `muse.env`).
