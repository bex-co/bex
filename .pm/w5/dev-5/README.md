# dev-5 — workstream w5's isolated local dev environment

A dedicated Kratos + Hydra + Mailpit stack (namespace `dev-5-auth`) plus a
dedicated app namespace (`dev-5`), layered onto the **shared** local
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

`dev-5` sidesteps this by giving workstream w5 its own Kratos/Hydra with
its own ports, instead of fighting over the shared one or another
workstream's. `dev-1` .. `dev-9` (`.pm/w1/dev-1` .. `.pm/w9/dev-9`) are
nine identical siblings — one per workstream — differing only in namespace
suffix and port block, all derived from the workstream number $N (see
`ports.env`'s header comment for the exact formula) so any subset can run
concurrently.

## Ports (see `ports.env`)

| service | port |
| --- | --- |
| dashboard | `50050` |
| kratos-public (port-forward) | `51050` |
| hydra-admin (port-forward) | `52050` |
| mailpit UI (port-forward) | `53050` |
| bex-api (local process) | `54050` |
| bex-db (port-forward) | `55050` |
| bex-api control-plane API (BEX_CP_ADDR, host-only) | `56050` |

## Start / status / stop

```sh
bash .pm/w5/dev-5/up.sh      # idempotent: creates namespaces, CNPG Clusters,
                               # mailpit, kratos+hydra (helm), port-forwards,
                               # and a local bex-api
bash .pm/w5/dev-5/status.sh  # pod status + http health checks
bash .pm/w5/dev-5/down.sh    # kills local processes, deletes both namespaces
```

Then start the dashboard separately (kept out of `up.sh` so it stays in the
foreground / under your own process control — `up.sh` prints the exact
command):

```sh
cd dashboard && VITE_API_URL=http://localhost:54050/graphql \
  VITE_KRATOS_PUBLIC_URL=http://localhost:51050 yarn dev --port 50050
```

## Recovering after a restart

`up.sh` is idempotent — re-running it after a reboot (or after the shared
`bex` kind cluster was recreated) regenerates the kubeconfig from `kind`,
re-applies the CNPG Clusters / mailpit / Kratos / Hydra (helm
`upgrade --install`), and re-forks the port-forwards + bex-api. Existing
KeyValue/App/Database CRs in the `dev-5` namespace survive as long as the
underlying `bex` cluster (and its PVs) survive.

## What's deliberately NOT duplicated

- **The bex operator** — already cluster-scoped in the shared cluster,
  reconciles CRs in every namespace including `dev-5`. No need for a second
  instance per workstream.
- **OpenFGA** — left unset for this environment's local bex-api, same as the
  shared cluster's default (unset ⇒ authz allow-all). Not needed to exercise
  the datastore CRUD flows this environment is for.
- **cert-manager / zot / kpack** — not needed for the KeyValue/Postgres CRUD
  flows this environment targets; only relevant to App builds/custom domains.
