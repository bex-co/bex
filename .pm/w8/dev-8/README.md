# dev-8 — workstream w8's isolated local dev environment

A dedicated Kratos + Hydra + Mailpit stack (namespace `dev-8-auth`) plus a
dedicated app namespace (`dev-8`), layered onto the **shared** local
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

`dev-8` sidesteps this by giving workstream w8 its own Kratos/Hydra with
its own ports, instead of fighting over the shared one or another
workstream's. `dev-1` .. `dev-9` (`.pm/w1/dev-1` .. `.pm/w9/dev-9`) are
nine identical siblings — one per workstream — differing only in namespace
suffix and port block, all derived from the workstream number $N (see
`ports.env`'s header comment for the exact formula) so any subset can run
concurrently.

## Ports (see `ports.env`)

| service | port |
| --- | --- |
| dashboard | `50080` |
| kratos-public (port-forward) | `51080` |
| hydra-admin (port-forward) | `52080` |
| mailpit UI (port-forward) | `53080` |
| bex-api (local process) | `54080` |
| bex-db (port-forward) | `55080` |
| bex-api control-plane API (BEX_CP_ADDR, host-only) | `56080` |
| kratos-admin (port-forward, BEX_KRATOS_ADMIN_URL) | `57080` |

## Start / status / stop

```sh
bash scripts/dev-env.sh 8 up      # idempotent: creates namespaces, CNPG Clusters,
                               # mailpit, kratos+hydra (helm), port-forwards,
                               # and a local bex-api
bash scripts/dev-env.sh 8 status  # pod status + http health checks
bash scripts/dev-env.sh 8 down    # kills local processes, deletes both namespaces
```

Then start the dashboard separately (kept out of `dev-env.sh 8 up` so it stays in the
foreground / under your own process control — `dev-env.sh 8 up` prints the exact
command):

```sh
cd dashboard && VITE_API_URL=http://localhost:54080/graphql \
  VITE_KRATOS_PUBLIC_URL=http://localhost:51080 yarn dev --port 50080
```

## Recovering after a restart

`dev-env.sh 8 up` is idempotent — re-running it after a reboot (or after the shared
`bex` kind cluster was recreated) regenerates the kubeconfig from `kind`,
re-applies the CNPG Clusters / mailpit / Kratos / Hydra (helm
`upgrade --install`), and re-forks the port-forwards + bex-api. Existing
KeyValue/App/Database CRs in the `dev-8` namespace survive as long as the
underlying `bex` cluster (and its PVs) survive.

## Tenant-namespace isolation between concurrent dev-N stacks

`dev-env.sh 8 up` sets **`BEX_CP_IDENTITY=dev-8`**, so every tenant namespace this
control plane provisions is stamped `app.bex.co/control-plane: dev-8` and its
orphan prune deletes only namespaces carrying that value (w6/m39, ADR043 D9).

This matters because namespace *provisioning* is namespaced but the orphan
*prune* is **cluster-scoped**: before m39 every bex-api holding a
`BEX_CP_DB_URI` deleted every managed `tea-*` namespace absent from **its own**
database, so two `dev-N` stacks on the shared cluster wiped each other's
tenants within one 60-second resync (observed live in both directions,
`.pm/w3/017.md`). **Running several `dev-N` control planes against the shared
cluster at once is now safe.**

`dev-env.sh 8 status` prints every managed tenant namespace with its owner — if a row
shows an owner other than `dev-8` it belongs to another harness and this one
will leave it alone; if *this* harness's rows show anything but `dev-8`, its
`BEX_CP_IDENTITY` is mis-set and it is pruning outside its lane.

## What's deliberately NOT duplicated

- **The bex operator** — already cluster-scoped in the shared cluster,
  reconciles CRs in every namespace including `dev-8`. No need for a second
  instance per workstream.
- **OpenFGA** — left unset for this environment's local bex-api, same as the
  shared cluster's default (unset ⇒ authz allow-all). Not needed to exercise
  the datastore CRUD flows this environment is for.
- **cert-manager / zot / kpack** — not needed for the KeyValue/Postgres CRUD
  flows this environment targets; only relevant to App builds/custom domains.

## Where the scripts went (w1/m72)

`up.sh` / `down.sh` / `status.sh` and the templated manifests (`db/`, `mailpit/`,
`values/`, `rbac-dev-8.yaml`) used to live in this directory, copied ten times.
They are now one implementation — **`scripts/dev-env.sh 8 {up|down|status|clean|env}`**
— with the manifests templated under `scripts/dev-env/`. What stays here is only
what is genuinely per-workstream: `ports.env` (a generated record of the
derivation, checked by `scripts/dev-env.test.sh`), this README, and `.gitignore`.

Local runtime state (`bin/`, `logs/`, `.pids/`, `.kubeconfig`) is untouched by
that move and still lives here. **`logs/` is truncated on every `up`** so a
long-lived environment cannot accumulate without bound — copy anything you need
to keep before re-running. `scripts/dev-env.sh 8 clean` reclaims `logs/` and
`bin/`, and refuses while the environment is up.

Something genuinely per-workstream can go in an optional `override.env` here; it
is sourced after the derivation. It may not change `DEV_NS`/`DEV_AUTH_NS` — the
script refuses, because cross-N isolation depends on them.
