# dev-3 — workstream w3's isolated local dev environment

A dedicated Kratos + Hydra + Mailpit stack (namespace `dev-3-auth`) plus a
dedicated app namespace (`dev-3`), layered onto the **shared** local
kind/CAPD cluster (`scripts/mock-cluster.sh`'s `bex` cluster) without
touching its `auth`/`default` namespaces. It reuses that cluster's CNPG
operator, cert-manager, and the already-running bex operator (cluster-scoped,
watches every namespace) — only the identity stack is duplicated.

## Why this exists

The shared cluster's Kratos (`auth` namespace) hard-codes its `ui_url` /
`allowed_return_urls` / CORS origin to `http://localhost:5173` (see
`deploy/gitops/overlays/local/values/kratos.values.yaml`) — those are
single-valued, so only one dashboard dev server can be "the" configured
origin at a time. Two agent sessions working different workstreams
(`.pm/wN`) against the shared cluster at once will have their Kratos error
flows silently redirect into whichever session's dev server already owns
`:5173` (confirmed empirically: an unlisted `return_to` 400s, and Kratos's
hard-coded error `ui_url` sends the browser to `localhost:5173/auth/error`
regardless of which origin actually made the request).

`dev-3` sidesteps this by giving workstream w3 its own Kratos/Hydra with
its own ports, instead of fighting over the shared one or another
workstream's. `dev-1` .. `dev-9` (`.pm/w1/dev-1` .. `.pm/w9/dev-9`) are
nine identical siblings — one per workstream — differing only in namespace
suffix and port block, all derived from the workstream number $N (see
`ports.env`'s header comment for the exact formula) so any subset can run
concurrently.

## Ports (see `ports.env`)

| service | port |
| --- | --- |
| dashboard | `50030` |
| kratos-public (port-forward) | `51030` |
| hydra-admin (port-forward) | `52030` |
| mailpit UI (port-forward) | `53030` |
| bex-api (local process) | `54030` |
| bex-db (port-forward) | `55030` |
| bex-api control-plane API (BEX_CP_ADDR, host-only) | `56030` |
| kratos-admin (port-forward, BEX_KRATOS_ADMIN_URL) | `57030` |
| hydra-public / issuer (port-forward, BEX_OAUTH_ISSUER) | `58030` |
| loki (port-forward, BEX_LOKI_URL) | `59030` |
| mailpit SMTP (port-forward, BEX_SMTP_ADDR) | `60030` |
| OpenSandbox lifecycle server (`agent-up`) | `61030` |
| ssh-gateway agent attach SSE (`agent-up`) | `62030` |
| OpenFGA (`agent-up`) | `63030` |
| OpenBao (`agent-up`) | `64030` |
| ssh-gateway sandbox-exec (`agent-up`) | `65030` |

## Start / status / stop

```sh
bash scripts/dev-env.sh 3 up      # idempotent: creates namespaces, CNPG Clusters,
                               # mailpit, kratos+hydra (helm), port-forwards,
                               # and a local bex-api
bash scripts/dev-env.sh 3 status  # pod status + http health checks
bash scripts/dev-env.sh 3 down    # kills local processes, deletes both namespaces
```

Then start the dashboard separately (kept out of `dev-env.sh 3 up` so it stays in the
foreground / under your own process control — `dev-env.sh 3 up` prints the exact
command):

```sh
cd dashboard && VITE_API_URL=http://localhost:54030/graphql \
  VITE_KRATOS_PUBLIC_URL=http://localhost:51030 yarn dev --port 50030
```

## Recovering after a restart

`dev-env.sh 3 up` is idempotent — re-running it after a reboot (or after the shared
`bex` kind cluster was recreated) regenerates the kubeconfig from `kind`,
re-applies the CNPG Clusters / mailpit / Kratos / Hydra (helm
`upgrade --install`), and re-forks the port-forwards + bex-api. Existing
KeyValue/App/Database CRs in the `dev-3` namespace survive as long as the
underlying `bex` cluster (and its PVs) survive.

## Tenant-namespace isolation between concurrent dev-N stacks

`dev-env.sh 3 up` sets **`BEX_CP_IDENTITY=dev-3`**, so every tenant namespace this
control plane provisions is stamped `app.bex.co/control-plane: dev-3` and its
orphan prune deletes only namespaces carrying that value (w6/m39, ADR043 D9).

This matters because namespace *provisioning* is namespaced but the orphan
*prune* is **cluster-scoped**: before m39 every bex-api holding a
`BEX_CP_DB_URI` deleted every managed `tea-*` namespace absent from **its own**
database, so two `dev-N` stacks on the shared cluster wiped each other's
tenants within one 60-second resync (observed live in both directions,
`.pm/w3/017.md`). **Running several `dev-N` control planes against the shared
cluster at once is now safe.**

`dev-env.sh 3 status` prints every managed tenant namespace with its owner — if a row
shows an owner other than `dev-3` it belongs to another harness and this one
will leave it alone; if *this* harness's rows show anything but `dev-3`, its
`BEX_CP_IDENTITY` is mis-set and it is pruning outside its lane.

## Agent sessions (opt-in: `agent-up`)

The base `up` above cannot run an ADR047 cloud coding-agent session — it ships
none of that machinery. `bash scripts/dev-env.sh 3 agent-up` layers it on:

| added | why the base stack lacks it |
| --- | --- |
| OpenSandbox controller + lifecycle server (`opensandbox-system`) | one sandbox Pod per session; `deploy/gitops/overlays/local` deletes the hosted server because the mock has no gVisor |
| in-cluster **ssh-gateway** (`bex-system`) | it resolves the sandbox Pod IP and dials `podIP:8787`; Calico Pod IPs are not routable from macOS, so this is the one dev-N component that cannot be a host process |
| **OpenFGA** (`dev-3-auth`) | `cmd/ssh-gateway` calls `requiredEnv("BEX_OPENFGA_URL")` and has no allow-all hatch; bex-api must share the store or every attach is denied |
| **OpenBao** (`dev-3-auth`, dev mode) | the BYO model key (ADR062) the gateway mints per exchange |
| local sandbox NetworkPolicies | production grants the sandbox's three network paths with cluster-wide **Cilium** policies; this cluster runs Calico |
| CiliumNetworkPolicy CRD shim | bex-api mints a per-session egress policy; without the CRD **every** session fails at dispatch with `no matches for kind "CiliumNetworkPolicy"`. Registered as an inert schema — it is NOT enforced on this Calico cluster |

`agent-up` is **exclusive across the whole cluster**, not per-workstream. Three of
those workloads have names the product hard-codes — `namespaces.go` binds each
`<ws>-sandbox` namespace's RoleBindings to `opensandbox-server` /
`opensandbox-controller-manager` in `opensandbox-system` and to `bex-ssh-gateway`
in `bex-system`, and the sandbox's Git remote defaults to that gateway's FQDN.
They cannot be suffixed with N without stripping the sandbox RBAC. So the leg is
stamped `app.bex.co/dev-env: dev-3` and refuses to take it from another
workstream; `agent-down` releases it.

It also flips two local-dev shortcuts off: with the gateway running, bex-api runs
with a real `BEX_CP_TOKEN` and real OpenFGA instead of `BEX_CP_INSECURE=1` /
`BEX_ALLOW_INSECURE_AUTHZ=1`.

A session comes in two shapes, and only one of them needs GitHub:

- **repo-less (chat-only)** — omit `repo` and the agent just runs against the
  prompt in an empty sandbox (`validateCreate` sets `BEX_AGENT_DELIVER=0`; no
  clone, no branch, no PR). This needs **no GitHub App at all** and is the
  quickest way to prove the whole path locally.
- **delivery** — with `repo` + a `bex-agent/*` branch, the driver clones through
  the gateway's Git proxy and pushes a draft PR. This needs a GitHub App that can
  mint **installation** tokens; the throwaway identity `up` generates cannot.

Two things stay manual, because they are per-workspace rather than
per-environment:

```sh
# 1. per sandbox namespace, the local stand-in for the Cilium allows
bash scripts/dev-env.sh 3 agent-netpol <workspaceId>

# 2. the BYO model key (note the `default` tenant root — the agent-session
#    read does not scope the OpenBao path by workspace)
bao kv put tenants/default/agent-sessions/<workspaceId>/model-key \
  BEX_AGENT_MODEL_API_KEY=<key>

# (delivery sessions only) stage real GitHub App credentials in
# .agent/github-app.env + .agent/github-app.pem, then re-run agent-up.
```

### Completing a turn without a provider key (`agent-stub`)

`bash scripts/dev-env.sh 3 agent-stub` stands up a local Anthropic Messages API
stub and points the gateway's upstream hop at it, so a **repo-less session runs a
real turn end to end with no credential at all**:

```
POST /v1/agent-sessions  ->  OpenFGA tuple + durable row
  -> per-session egress CR -> OpenSandbox -> sandbox Pod (agent image)
  -> driver -> claude-code-acp over stdio (ACP handshake)
  -> gateway model proxy :8084 -> Pod verification -> mint via bex-api -> OpenBao
  -> credential injected on the upstream TLS hop -> response
  -> ACP updates mapped to AI SDK parts -> durable transcript
  -> POST /attach-ticket -> gateway SSE replay (single-use ticket)
```

⚠️ **The stub is a test double, not a model.** Replies are canned. It proves
bex's transport, credential mint, transcript and attach path; it proves nothing
about the real provider — that is what the operator-run
`scripts/agent-session-verify.sh` covers against a live deployment. Undo with
`agent-stub-off`, which restores the gateway's normal trust store so the same
environment can be pointed at a real key.

It has to interpose at the gateway's upstream hop because bex pins each agent
profile to its registered provider endpoint (`RegisteredModelEndpoint` rejects any
other `agentConfig.modelEndpoint`) — a deliberate product rule the stub must not
weaken. So it serves TLS as `api.anthropic.com` behind a `hostAlias`, and the
gateway trusts it through a CA bundle (stub CA appended to a real root store, so
every other TLS destination still verifies) via Go's `SSL_CERT_FILE`.

### Driving the agent-session UI (`VITE_AGENT_STREAM_URL`)

The dashboard's `/agents` conversation view will not load in dev-3 on the
default env — it silently shows "The conversation stream is unavailable" forever.
Start the dashboard with the stream origin pointed at the gateway:

```sh
cd dashboard && HYDRA_ADMIN_URL=http://localhost:52030 \
  HYDRA_PUBLIC_URL=http://localhost:58030 \
  VITE_API_URL=http://localhost:54030/graphql \
  VITE_KRATOS_PUBLIC_URL=http://localhost:51030 \
  VITE_KRATOS_SSR_URL=http://localhost:51030 \
  VITE_AGENT_STREAM_URL=http://localhost:62030 \
  yarn dev --port 50030
```

**Why it is needed.** `agentSessionStreamUrl()` builds the SSE endpoint from
`config.agentStreamBaseUrl`, which defaults to the **bex-api** origin — correct in
production, where Traefik path-routes `api.bex.co/v1/agent-sessions/{{id}}/stream`
to the gateway, so the two share one origin. Locally they are separate ports and
bex-api does not proxy that path, so the browser hits bex-api and is rejected at
the preflight:

```
Access to fetch at 'http://localhost:54030/v1/agent-sessions/<id>/stream'
  blocked by CORS policy: Request header field x-bex-agent-ticket is not
  allowed by Access-Control-Allow-Headers in preflight response.
```

`VITE_AGENT_STREAM_URL` overrides that origin (`dashboard/src/config/config.ts`).
Pointed at `62030` the request reaches the gateway's attach listener directly,
which does allow the ticket header and echoes CORS for the dev-3 dashboard
origin (its `BEX_API_CORS_ORIGIN`).

**`/agents` is also GrowthBook-gated** to one beta workspace
(`dashboard/src/config/growthbook.ts`), so a local workspace is redirected to `/`
until that file has a rule matching it.

Readiness is observable: `GET /v1/agent-sessions/capabilities?ownerId=<workspaceId>`
returns `enabled`, `github.connected`, `modelKeyReady` and an overall `ready`.

`.agent/` holds this environment's generated secrets (control-plane token, the
HMAC keys bex-api and the gateway must share byte-for-byte, the OpenBao dev
token, the gateway host key). It is gitignored — never commit it.

**Not available locally:** sandbox pause/resume. The snapshot path needs
containerd-CRI + runsc `--overlay2=none` (ADR042 D5/D6); create/exec/delete work.

## What's deliberately NOT duplicated

- **The bex operator** — already cluster-scoped in the shared cluster,
  reconciles CRs in every namespace including `dev-3`. No need for a second
  instance per workstream.
- **OpenFGA** — left unset by `up` for this environment's local bex-api, same as the
  shared cluster's default (unset ⇒ authz allow-all). Not needed to exercise
  the datastore CRUD flows this environment is for.
- **cert-manager / zot / kpack** — not needed for the KeyValue/Postgres CRUD
  flows this environment targets; only relevant to App builds/custom domains.

## Where the scripts went (w1/m72)

`up.sh` / `down.sh` / `status.sh` and the templated manifests (`db/`, `mailpit/`,
`values/`, `rbac-dev-3.yaml`) used to live in this directory, copied ten times.
They are now one implementation — **`scripts/dev-env.sh 3 {up|down|status|clean|env}`**
— with the manifests templated under `scripts/dev-env/`. What stays here is only
what is genuinely per-workstream: `ports.env` (a generated record of the
derivation, checked by `scripts/dev-env.test.sh`), this README, and `.gitignore`.

Local runtime state (`bin/`, `logs/`, `.pids/`, `.kubeconfig`) is untouched by
that move and still lives here. **`logs/` is truncated on every `up`** so a
long-lived environment cannot accumulate without bound — copy anything you need
to keep before re-running. `scripts/dev-env.sh 3 clean` reclaims `logs/` and
`bin/`, and refuses while the environment is up.

Something genuinely per-workstream can go in an optional `override.env` here; it
is sourced after the derivation. It may not change `DEV_NS`/`DEV_AUTH_NS` — the
script refuses, because cross-N isolation depends on them.
