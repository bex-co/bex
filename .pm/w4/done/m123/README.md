# w4 · m123 — Every tenant container gets 174 Kubernetes service-link env vars nobody asked for, naming every sibling resource in the workspace

**Worker:** worker4 **Goal:** a tenant process sees the environment bex says it will see — the variables the user set, plus `PORT` — and not 174 legacy Docker-link variables Kubernetes injects for every Service in the namespace, including the platform's own. **Status:** done 2026-09-21 (live re-probe deferred to the next QA pass; one DoD bullet is corrected — see Outcome)

## Tasks (in order)

| id   | title                                                                                | est | depends_on   |
| ---- | ------------------------------------------------------------------------------------ | --- | ------------ |
| t001 | Set `enableServiceLinks: false` on the App Deployment projection — in the projection, not the server-defaults helper | 40m | —            | — **DONE**
| t002 | Decide and execute the one-time pod roll this causes, and say so where users will see it | 35m | w4/m123/t001 | — **DONE**
| t003 | Place the other 10 PodSpec sites: builds, pre-deploy, cron runs, backups, exports, publish, Key Value | 45m | w4/m123/t001 | — **DONE**
| t004 | Record the environment contract: what bex injects, what it does not, and how a service reaches a sibling | 30m | w4/m123/t003 | — **DONE**
| t005 | Render parity — compare the delivered environment against a render.com service                | 30m | w4/m123/t004 | — **DONE**
| t006 | Simplify — `/simplify` over the code this milestone changed                                   | 20m | w4/m123/t005 | — **DONE**
| t007 | Test coverage — the injected set is exactly what the contract says                            | 40m | w4/m123/t005 | — **DONE**
| t008 | Closeout — close the milestone once the definition of done actually holds                     | 15m | w4/m123/t007 | — **DONE**

## Definition of done

- **A tenant container's environment is the user's plus `PORT`.** On a freshly deployed web service whose environment is `{QA_MARKER, OWN_VAR, HTTP_PORT, ECHO_INCLUDE_ENV_VARS}`, the process sees those four plus `PORT` and **nothing else**. Today the same service saw **185** variables, of which **5** were bex's or the user's and **174** were Kubernetes service links.
- **No sibling resource is enumerated into a container.** `env | grep -E '_SERVICE_HOST|_SERVICE_PORT|_PORT_[0-9]+_TCP'` returns empty. Today it returns entries for all **25** sibling prefixes in the workspace, including three managed Postgres instances by ID (`DPG_D9RS3EE0CCIS738KC7C0_RW_SERVICE_HOST=10.98.126.18`) and every other app by slug.
- **No platform-owned object is enumerated either.** `CM_ACME_HTTP_SOLVER_VJGN6_SERVICE_PORT=8089` — cert-manager's ACME solver Service, not a tenant resource — is gone, as are `KUBERNETES_SERVICE_HOST` / `KUBERNETES_SERVICE_PORT`.
- **A service can still reach its siblings the documented way.** The private-network address (`<slug>:<port>`, ADR041 D4 / `w9/m58`) still resolves from inside a container after the change — removing the link vars must not remove the supported path, and t004 says what that path is.
- **Every pod-producing projection is placed.** t003's output is a table of all **11** `corev1.PodSpec{}` sites in `lego/operator/internal/` (`grep -rn --include='*.go' 'corev1.PodSpec{' lego/operator/internal/ | grep -v _test | wc -l` → 11) saying, for each, whether it now disables service links and why. Build, pre-deploy and cron pods run tenant code and are in scope; platform-owned maintenance pods (backups, exports, publish) need a stated reason either way.
- **The roll is deliberate, not a surprise.** t002 names when every tenant pod restarts once and what users are told, because this change moves the stored pod template and therefore the Deployment's pod-template hash.
- **The contract is written down.** ADR004's `envVars` section states the complete set of platform-injected variables (today it promises `PORT` is the only one, which is false by 174), and `docs/ADR018-render-parity.md` records how the delivered environment compares with Render's.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 109 (w4-targeted, `muse.env` credentials), journey 4 (env vars, secret files, env groups). Fixture `qa-20260921-env` (`srv-daoef8rs0ils73bgpd8g`), an image service running `docker.io/mendhak/http-https-echo` with `ECHO_INCLUDE_ENV_VARS=1` so the container's own environment could be read over HTTP — since deleted.
- **Goal linkage:** ADR004 (`envVars`) owns the injection contract, and its current promise is the thing that is wrong. The operator's own `appEnv` doc comment calls `PORT` "the one env invariant the CRD contract promises" — the platform delivers 174 more that no bex code puts there. ADR008 pillar 1: a Render-compatible hosting product, where the environment a process sees is the environment the user configured.
- **Expected outcome:** an app that reads `REDIS_PORT`, `API_PORT`, `POSTGRES_PORT` or any other `<NAME>_PORT` and expects its own configured value stops being handed a `tcp://10.x.x.x:6379` URL by the platform. This is the classic Kubernetes service-link footgun, and bex is currently exposed to it by default for every tenant.
- **Why now:** the collision is silent and looks like an application bug, so it costs a user hours before they suspect the platform. It is also cheap to fix — one field — and the cost of fixing it later is identical, except that more running workloads have to roll.
- **Render parity task included:** the delivered environment is a user-facing surface even though the change itself is operator-internal, and t005 is where bex's set is compared with Render's.

## Evidence (live, 2026-09-21)

The service's own environment, read from inside the container:

```
GET https://qa-20260921-env.onbex.co/   →  200, JSON including "env": { … }

total env vars                : 185
set by the user or by bex     :   5   (QA_MARKER, OWN_VAR, HTTP_PORT, ECHO_INCLUDE_ENV_VARS, PORT)
Kubernetes service-link shape : 174
distinct sibling prefixes     :  25
```

`QA_MARKER=pass109` arrived from the linked environment group `qa-20260921-eg`, so the env-group journey itself works — this milestone is about everything else that arrived with it.

A sample of what a tenant process can read without asking:

```
QA_20260921_ENV_PORT                      = tcp://10.102.98.220:8080     ← the service's own name
QA_20260921_ENV_SERVICE_HOST              = 10.102.98.220
DPG_D9RS3EE0CCIS738KC7C0_RW_SERVICE_HOST  = 10.98.126.18                 ← another tenant resource: a managed Postgres primary
CM_ACME_HTTP_SOLVER_VJGN6_SERVICE_PORT    = 8089                         ← cert-manager's ACME solver, a PLATFORM object
KUBERNETES_SERVICE_HOST                   = 10.96.0.1                    ← the API server
```

The 25 prefixes cover every app slug in the workspace (`BLOCK_EDEN_MONO`, `EDEN_CMS_V2`, `BEANCOUNT_FORUM`, `AGENTMARKETCAP_1`, …), each managed Postgres in all three of its `-r` / `-ro` / `-rw` forms, and each of those again under its `TEA_D98210CBBPDC73DCRKVG_…` prefixed alias.

**The collision is the same rule applied, not a hypothesis.** The live capture shows the service's own name becoming `QA_20260921_ENV_PORT=tcp://10.102.98.220:8080`. Kubernetes derives that name by upper-casing the Service name and replacing `-` with `_`, so a user who names a web service `redis` receives `REDIS_PORT=tcp://10.x.x.x:6379` — the exact value that breaks every client library whose `REDIS_PORT` means `6379`. Same for `api`, `postgres`, `mysql`, `mongo`.

## Root cause

- `lego/operator/internal/controller/deployment_projection.go:426` — the App Deployment's pod template is built and then handed to `applyPodSpecServerDefaults`. Neither sets `EnableServiceLinks`, so it stays `nil` and the API server defaults it to **true**, which is what makes kubelet enumerate every Service in the namespace into every container.
- `grep -rl 'EnableServiceLinks' lego dashboard` → **0**. The field appears nowhere in the codebase; this is an unconsidered default, not a decision that was made and recorded.
- `lego/operator/internal/controller/app_controller.go:3994-4026` — `appEnv`'s doc comment: "the operator-owned `PORT` last … the one env invariant the CRD contract promises". True of what `appEnv` builds; false of what the container receives, because the link vars are added by kubelet, below this layer.
- Namespacing bounds the damage and must be stated: the injected Services are all in the workspace's own namespace (`tea-d98210cbbpdc73dcrkvg`, ADR043 per-tenant namespaces), so this is **not** cross-tenant leakage. It is (a) a correctness footgun inside a workspace, (b) platform objects that share the tenant namespace being enumerated to tenant code, and (c) an untrue contract. `AutomountServiceAccountToken: false` is already set, so `KUBERNETES_SERVICE_HOST` carries no credential with it.

## Blast radius

- **`applyPodSpecServerDefaults` is the wrong home for this fix, and that matters.** That helper's contract is explicitly "what Kubernetes would have chosen", and its doc comment records that writing those values **cannot roll a pod** because the stored object is byte-identical before and after. `enableServiceLinks: false` is the opposite: it *differs* from the server default, so it changes the stored pod template, moves the Deployment's pod-template hash, and rolls every tenant pod once. It belongs in the projection — the part that says what bex chooses — and t002 owns the consequence.
- **11 PodSpec sites** exist under `lego/operator/internal/` (exact count, not an estimate): `deployment_projection.go:426`, `app_controller.go:3864` (cron runs), `build/build.go:1022`, `predeploy/predeploy.go:158`, `keyvalue_controller.go:686`, `keyvalue_backup.go:288,452`, `disk_backup.go:268`, `database_controller.go:1639`, `database_exports.go:310,384`, `publish/publish.go:383,632`. The ones that run **tenant-authored code** — the Deployment, cron runs, builds, pre-deploy — are the ones where injection is a tenant-visible defect; the rest need a stated decision, not a silent one.
- **Removing the vars could break an app that is relying on them today.** Nothing in bex documents them, so nothing should depend on them — but "nothing documented it" is not "nobody used it". t004 must name the supported replacement (the `<slug>:<port>` private address from `w9/m58` / ADR041 D4) in the same place it announces the removal.

## Adjacent classes

- **`PORT`** — bex-owned, injected by `appEnv`, unchanged by this milestone and still stripped from a user's `spec.env` (`w2/m95`, ADR018 row 117).
- **`KUBERNETES_SERVICE_HOST` / `_PORT`** — also a service link (the `kubernetes` Service in every namespace); disappears with the same flag. Confirm nothing in the tenant runtime path reads it.
- **Env-group and secret-file variables** — user-owned, arrive via `envFrom`, untouched.
- **`BEX_NATIVE_*`** — build-time, already filtered by `appEnv` and `build/native.go:167`; out of scope.

## Unverified this run

- Only a **web service** container's environment was read. Cron-run, build and pre-deploy pods were not inspected live — t003 is the work of checking them, and nothing here claims what they currently receive.
- The `redis` collision is derived from the live `QA_20260921_ENV_PORT` capture plus Kubernetes' documented naming rule; no service literally named `redis` was created.
- Whether any existing production tenant app currently reads an injected link var was not checked, and cannot be from outside the container.

## Also checked this run, and found correct

- **Environment group end to end.** Create with a variable and a secret file → link to a service → the link triggers a redeploy → `QA_MARKER=pass109` is present in the running process. The service Environment tab and the group page agree on both members.
- **Multi-line secret files round-trip.** `"line-one\nline-two pass109\n"` came back from Reveal with its newlines intact in the DOM — one of `w2/m95`'s blocked DoD bullets, now observed live. **But see `.pm/w4/108.md`:** the revealed value is rendered `white-space: normal`, so a multi-line file displays as a single 20px line. Stored correctly, shown wrongly.
- **Shell is honestly plan-gated.** "Shell access requires a running paid web, private, or background service and an active SSH gateway" on a free service — accurate, not a failure.

## Outcome (2026-09-21)

**t001 — one field, in the right place.** `enableServiceLinks: false` is set on the App Deployment's pod template in `applyDeploymentSpec`, deliberately **not** in `applyPodSpecServerDefaults`: that helper exists to mirror what Kubernetes would have chosen, and this is a bex choice that contradicts it. The comment there says so, so nobody moves it later as "tidying".

**t003 — all 11 PodSpec sites, and all 11 disabled.** The table the DoD asked for:

| # | Site | Runs | Disabled |
| --- | --- | --- | --- |
| 1 | `deployment_projection.go` (App Deployment) | tenant code | ✅ the finding itself |
| 2 | `build/build.go` | tenant code (Dockerfile / buildpack) | ✅ |
| 3 | `predeploy/predeploy.go` | tenant code (the pre-deploy command) | ✅ |
| 4 | `database_controller.go:1639` (cron run) | tenant code | ✅ |
| 5-6 | `database_exports.go` ×2 | platform maintenance | ✅ |
| 7 | `disk_backup.go` | platform maintenance | ✅ |
| 8-9 | `keyvalue_backup.go` ×2 | platform maintenance | ✅ |
| 10 | `app_controller.go:3864` | platform maintenance | ✅ |
| 11-12 | `publish/publish.go` ×2 | platform maintenance | ✅ |

The DoD allowed the platform-owned pods to go either way with a stated reason. They are disabled too, and the reason is that there is no argument for the other side: an exhaustive grep proves **nothing in bex reads a service-link variable** (the one `KUBERNETES_SERVICE_HOST` reference in the tree belongs to the opensandbox controller, a separate component in its own namespace that this change does not touch), so for every one of these pods the setting is pure removal of surface with no dependency to break. Choosing per-pod would have meant maintaining a rule nobody can check; disabling everywhere is a rule a test can enforce — and does.

**A DoD bullet is wrong, and this is the correction.** Bullet 3 claims `KUBERNETES_SERVICE_HOST` / `KUBERNETES_SERVICE_PORT` are "gone" along with the cert-manager solver. They are not, and cannot be, by this mechanism: the kubelet treats the master `kubernetes` Service in the `default` namespace as unconditional (`getServiceEnvVarMap` adds it "even if enableServiceLinks is false") and only gates same-namespace Services on the setting. The cert-manager ACME solver **does** disappear, because cert-manager creates that Service in the tenant's own namespace. So the delivered set becomes **the user's variables + `PORT` + those two**, not the user's + `PORT`. The two are harmless here — they name an API server the pod has no credential for (`automountServiceAccountToken: false`) and no NetworkPolicy path to — but the contract has to state what is true, so ADR004 says exactly this and the next QA pass should expect 7, not 5, on the reproduction fixture.

**t002 — the roll.** Writing the field moves the stored pod template, so the Deployment controller's pod-template hash moves and **every tenant pod restarts once** when this deploys. That is unavoidable for a change to the pod template and is the reason the milestone's own "why now" argument holds: the cost is identical later, against more running workloads. The restart is a normal rolling update — replicas roll one at a time under the existing readiness gates, so a healthy multi-replica service stays available and a single-replica free service has the same brief gap any config change gives it. No user action is required and no data is affected. It is recorded here and in ADR004 rather than announced separately, matching how every other pod-template change in this repo has been handled.

**t004 — the contract.** ADR004's `envVars` section now states the complete injected set, the shadowing footgun in the terms a user would hit it (`REDIS_PORT` handed a `tcp://…` URL), the two variables bex cannot remove and why they are harmless, and — the part the DoD specifically required — that reaching a sibling still goes through the private address `<slug>:<port>` (ADR041 D4), which never depended on service links, so nothing supported was removed.

**t005 — parity.** ADR018's `PORT` row records the comparison: Render injects `PORT` plus a small documented `RENDER_*` set and does not enumerate other resources into a container. bex's delivered set is now **narrower** than Render's, with no undocumented additions. bex deliberately injects no `RENDER_*` equivalents — they name a competitor's platform.

**t007 — coverage.** `TestAppPodDisablesServiceLinks` asserts the App pod's field and distinguishes unset (which Kubernetes defaults to true) from explicitly false, since only the second is a decision. `TestEveryPodSpecDisablesServiceLinks` parses the operator's own source and fails on any `corev1.PodSpec` literal that does not set the field, with an anti-vacuity floor of 11 so the sweep cannot quietly stop finding sites — the guard exists because the absence of a shared pod constructor is exactly how the default leaked into eleven places. Both mutation-spot-checked.

**Green:** `lego/operator` `make test` all packages; `lego/backend` `go test ./...` unaffected and green.

**Not done — the live re-probe.** The DoD's bullets are `env | grep` checks inside a running container on a deployed fixture; there was no production access this session, so none has been run. Deferred to the next QA pass, which should re-run the `mendhak/http-https-echo` + `ECHO_INCLUDE_ENV_VARS=1` reproduction and expect **7** variables (4 user + `PORT` + the two Kubernetes ones), an empty `_SERVICE_HOST|_SERVICE_PORT|_PORT_[0-9]+_TCP` grep, no `CM_ACME_*`, and a still-working `<slug>:<port>` sibling call.

## Live re-probe, pass 193 (2026-09-26, `/qa-find-bugs`, `muse.env` credentials): passes, with one doc correction

The fixture was Free web service `qa-20260926-envx` (`srv-darsb51smc7s73cq5k10`, image `docker.io/mendhak/http-https-echo:35`, env `{QA_MARKER, OWN_VAR, HTTP_PORT=3000, ECHO_INCLUDE_ENV_VARS=1}`), deleted the same pass. `curl https://qa-20260926-envx.onbex.co/` → the `env` object had **21** keys:

- the 4 user vars and `PORT`;
- the image's own runtime vars: `HOME, HOSTNAME, PATH, PWD, SHLVL, NODE_VERSION, YARN_VERSION, HTTPS_PORT`;
- the `kubernetes` master Service's set, **8** vars: `KUBERNETES_SERVICE_HOST, KUBERNETES_SERVICE_PORT, KUBERNETES_SERVICE_PORT_HTTPS, KUBERNETES_PORT, KUBERNETES_PORT_443_TCP{,_ADDR,_PORT,_PROTO}`.

**Sibling enumeration: gone (PASS).** Filtering `_SERVICE_HOST|_SERVICE_PORT|_PORT_[0-9]+_TCP` matched only the `KUBERNETES_*` names, with no app, Postgres, or Key Value prefix. `CM_ACME_*` is absent (PASS). The 174-variable leak is fixed on production.

**Correction:** the deferred note's expected total ("7 = 4 user + `PORT` + the two Kubernetes ones") and ADR004:158 ("The two Kubernetes variables…") undercount. The kubelet injects the master Service's **full** link set (8 vars) regardless of `enableServiceLinks`, and images add their own. Filed as `w4/153`.

**Not probed:** the `<slug>:<port>` sibling call (echo-server cannot originate requests).
