# w4 · m123 — Every tenant container gets 174 Kubernetes service-link env vars nobody asked for, naming every sibling resource in the workspace

**Worker:** worker4 **Goal:** a tenant process sees the environment bex says it will see — the variables the user set, plus `PORT` — and not 174 legacy Docker-link variables Kubernetes injects for every Service in the namespace, including the platform's own. **Status:** todo

## Tasks (in order)

| id   | title                                                                                | est | depends_on   |
| ---- | ------------------------------------------------------------------------------------ | --- | ------------ |
| t001 | Set `enableServiceLinks: false` on the App Deployment projection — in the projection, not the server-defaults helper | 40m | —            |
| t002 | Decide and execute the one-time pod roll this causes, and say so where users will see it | 35m | w4/m123/t001 |
| t003 | Place the other 10 PodSpec sites: builds, pre-deploy, cron runs, backups, exports, publish, Key Value | 45m | w4/m123/t001 |
| t004 | Record the environment contract: what bex injects, what it does not, and how a service reaches a sibling | 30m | w4/m123/t003 |
| t005 | Render parity — compare the delivered environment against a render.com service                | 30m | w4/m123/t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed                                   | 20m | w4/m123/t005 |
| t007 | Test coverage — the injected set is exactly what the contract says                            | 40m | w4/m123/t005 |
| t008 | Closeout — close the milestone once the definition of done actually holds                     | 15m | w4/m123/t007 |

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
