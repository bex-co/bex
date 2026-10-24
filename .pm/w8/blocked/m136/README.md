# w8 · m136 — Environment network isolation also closes its Postgres and Key Value to other environments, as on Render

**Worker:** worker8 **Goal:** With `networkIsolationEnabled: true`, a Key Value or Postgres in that environment refuses private-network connections from services in any other environment, as on Render ("blocks private network traffic from crossing its boundary"). Its external URL (allowlist-governed) and same-environment traffic keep working. **Status:** blocked — t001, t002, t003, t004, t005, t006 done 2026-10-23; release pipeline deploys bex-api, the operator and GitOps (new pod-label admission policy); QA runs t007: probe in staging vs a Free Key Value in production — TCPOK with isolation off, TCPFAIL within 60s after PATCH networkIsolationEnabled:true, same-environment probe still TCPOK, external allowlisted PING→PONG, TCPOK again after off; Postgres matrix when a slot is free; delete fixtures by ID with 404 proofs

## Tasks (in order)

| id   | title                                                                          | est | depends_on                 |
| ---- | ------------------------------------------------------------------------------ | --- | -------------------------- |
| t001 — **DONE** | Policy: isolated-environment datastores admit only same-environment peers       | 60m | —                          |
| t002 — **DONE** | Toggle, membership moves and environment delete converge promptly               | 30m | w8/m136/t001               |
| t003 — **DONE** | Keep external access, backups, metrics and platform users working               | 30m | w8/m136/t001               |
| t004 — **DONE** | Render parity: ADR032/ADR018 and the dashboard isolation copy                   | 15m | w8/m136/t002, w8/m136/t003 |
| t005 — **DONE** | Simplify the changed code                                                      | 15m | w8/m136/t004               |
| t006 — **DONE** | Test coverage: isolated vs not, cross-env vs same-env, KV and Postgres          | 40m | w8/m136/t004               |
| t007 | Close out the milestone                                                        | 30m | w8/m136/t005, w8/m136/t006 |

## Definition of done

Live on production after the release pipeline deploys the operator (and GitOps, if t001 uses a platform policy), with owned Free fixtures deleted by ID with 404 proofs:

- Project with environments `production` and `staging`; a Free Key Value in `production`, a Free busybox probe web service in `staging` looping `nc -z -w 3 <kv internal host> 6379`.
- Isolation off: `TCPOK`. `PATCH /v1/environments/<production> {"networkIsolationEnabled":true}` → within 60 s the probe logs only `TCPFAIL`. A probe in `production` still gets `TCPOK`. Isolation off again → `TCPOK`.
- The store's external endpoint with an allowlist still answers `bex kv-cli <id> -- PING` → `PONG` while isolated.
- The same matrix for Postgres (`nc -z <pg internal host> 5432`) when a free Postgres slot is available, or on a paid fixture in a dev environment.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep b1c3, 2026-10-23 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, platform pin `29d63e1f7bea`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device login. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Render [projects/environments](https://render.com/docs/projects): with private network isolation, "services _within_ the environment can still communicate", private traffic is blocked from crossing the boundary, and "Render Postgres and Key Value instances in the environment can still receive traffic at their external URL", implying their private endpoints are inside the boundary. [ADR032](../../../docs/ADR032-environments.md) and [ADR041](../../../docs/ADR041-service-addresses.md):14 ("project-environment network isolation narrows it further").
- **Expected outcome:** a team isolating `production` actually keeps `staging` services away from production data stores, the main reason to turn isolation on.
- **Why now:** major. A security boundary the UI and API report as on (`networkIsolationEnabled: true`) does not cover the most sensitive resources. Render parity task included: dashboard copy and ADRs describe the boundary.

## Evidence (sweep b1c3)

```text
POST /v1/projects {"name":"qa-20261023-b1c3-proj","ownerId":"tea-daif693dqjvc73e7as3g","environments":[{"name":"production"},{"name":"staging"}]}
→ 201 prj-dbduagj17lic73al6o10, environmentIds evm-dbduagj17lic73al6o1g (production), evm-dbduagj17lic73al6o20 (staging)
$ bex kv create --name qa-20261023-b1c3-kv --plan free --environment evm-dbduagj17lic73al6o1g --confirm -o json   → red-dbdualj17lic73al6o2g (available)
$ bex services create --type web_service --name qa-20261023-b1c3-probe --image docker.io/library/busybox:1.37 --plan free \
    --env-var PORT=10000 --environment-id evm-dbduagj17lic73al6o20 --confirm -o json                          → srv-dbdualhf0c2s738fsesg
  (Docker command via PATCH: loop `nc -z -w 3 red-dbdualj17lic73al6o2g.tea-daif693dqjvc73e7as3g.svc 6379` every 10 s)
isolation off:                          6× TCPOK
PATCH /v1/environments/evm-dbduagj17lic73al6o1g {"networkIsolationEnabled":true} → 200, body networkIsolationEnabled:true, redisIds:["red-dbdualj17lic73al6o2g"]
22:55:03–22:56:03 UTC (first 60 s):    9× TCPOK
22:57:00–22:58:15 UTC (2–3 min later):  7× TCPOK
```

Every probe is a new TCP connection, so Render's "existing connections persist until restart" note does not apply.

## Root cause

- Isolation is projected only onto **App** CRs (`core.LabelNetworkIsolation`, ADR032:84). Their per-App policy then narrows App ingress (w4/m185).
- Datastores get `reconcileEnvironmentPeerPolicy` (`lego/operator/internal/controller/environment_network_policy.go:38-71`; callers `keyvalue_controller.go:1040`, `database_controller.go:836`). That is an **additional** ingress *allow* from pods labelled with this environment's isolation id, so isolated Apps can still reach their own datastores. Kubernetes NetworkPolicies are a union of allows, so it restricts nothing: the store's existing same-workspace reachability (pods carry `app.bex.co/workspace`, `keyvalue_controller.go:727-735`) still admits every workspace pod from any environment.

## Fix (target)

- t001: when a datastore's environment is isolated, admit private ingress only from that environment's members (and platform components), refusing other workspace pods. With vanilla NetworkPolicy that means the datastore must not also be selected by a broader allow. Either select it out of the workspace-wide allow while isolated, or use a Cilium `ingressDeny` from workspace endpoints whose environment label differs (precedent: the clusterwide free-plan policies, `deploy/gitops/base/tenant-node-egress.yaml`). Carry the isolation state onto Database/KeyValue CRs (today only `LabelEnvironment` membership is there).
- t003: the external SNI proxy path, backup/purge Jobs, metrics exporters and the platform ACL user must stay allowed.

## Blast radius

Every Postgres and Key Value in an isolated environment, every workspace. Apps' isolation is unchanged. Turning isolation on will start refusing cross-environment datastore clients that work today, so the release note names it.

## Unverified

- Postgres was not exercised live (the workspace's one free Postgres slot is held by a pre-existing database). The same function and union reasoning apply (`database_controller.go:836`).
- Outbound is already correct (sweep c8d1, same pin): a busybox probe in an **isolated** `production` (`srv-dbdusp1f0c2s738fsfe0`) looping `nc -z -w 3` to a Free Key Value in non-isolated `staging` (`red-dbdusor17lic73al6o90`) logged 4× `TCPFAIL`, while `wget https://example.com/` logged 4× `NETOK`. Only the inbound direction into an isolated environment's datastores is open.

## Dedupe

Searched `.pm` open and done, ADR032/ADR018/ADR041 for isolation + Postgres/Key Value/datastore: w6/m19 (isolation for services), w4/m185 (prompt label enforcement for Apps), w6/m20 (datastore environment membership label). No item covers datastores under isolation.
