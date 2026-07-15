# dev-alpha — an isolated local dev environment

A dedicated Kratos + Hydra + Mailpit stack (namespace `dev-alpha-auth`) plus a
dedicated app namespace (`dev-alpha`), layered onto the **shared** local
kind/CAPD cluster (`scripts/mock-cluster.sh`'s `bex` cluster) without touching
its `auth`/`default` namespaces. It reuses that cluster's CNPG operator,
cert-manager, and the already-running bex operator (cluster-scoped, watches
every namespace) — only the identity stack is duplicated.

## Why this exists

The shared cluster's Kratos (`auth` namespace) hard-codes its `ui_url` /
`allowed_return_urls` / CORS origin to `http://localhost:5173` (see
`deploy/gitops/overlays/local/values/kratos.values.yaml`) — those are
single-valued, so only one dashboard dev server can be "the" configured
origin at a time. If another Claude Code session (or another `bexN`
worktree) is already running its dashboard on `:5173` against the shared
cluster, a second dashboard on a different port will have its Kratos error
flows silently redirect into *that* session's dev server instead of its own
(confirmed empirically: an unlisted `return_to` 400s, and Kratos's
hard-coded error `ui_url` sends the browser to `localhost:5173/auth/error`
regardless of which origin actually made the request).

dev-alpha sidesteps this by giving itself its own Kratos/Hydra with its own
ports, instead of fighting over the shared one.

## Ports (see `ports.env`)

| service | port |
| --- | --- |
| dashboard | `5273` |
| kratos-public (port-forward) | `15433` |
| hydra-admin (port-forward) | `15445` |
| mailpit UI (port-forward) | `15825` |
| bex-api (local process) | `18095` |

Chosen to avoid the standard `5173`/`4433`/`4445`/`8090` local-dev ports so
dev-alpha can run alongside a concurrent session using those.

## Start / status / stop

```sh
bash env/dev-alpha/up.sh      # idempotent: creates namespaces, CNPG Clusters,
                               # mailpit, kratos+hydra (helm), port-forwards,
                               # and a local bex-api
bash env/dev-alpha/status.sh  # pod status + http health checks
bash env/dev-alpha/down.sh    # kills local processes, deletes both namespaces
```

Then start the dashboard separately (kept out of `up.sh` so it stays in the
foreground / under your own process control — `up.sh` prints the exact
command):

```sh
cd dashboard && VITE_API_URL=http://localhost:18095/graphql \
  VITE_KRATOS_PUBLIC_URL=http://localhost:15433 yarn dev --port 5273
```

## Recovering after a restart

`up.sh` is idempotent — re-running it after a reboot (or after the shared
`bex` kind cluster was recreated) regenerates the kubeconfig from `kind`,
re-applies the CNPG Clusters / mailpit / Kratos / Hydra (helm
`upgrade --install`), and re-forks the port-forwards + bex-api. Existing
KeyValue/App/Database CRs in the `dev-alpha` namespace survive as long as the
underlying `bex` cluster (and its PVs) survive.

## What's deliberately NOT duplicated

- **The bex operator** — already cluster-scoped in the shared cluster,
  reconciles CRs in every namespace including `dev-alpha`. No need for a
  second instance.
- **OpenFGA** — left unset for dev-alpha's local bex-api, same as the shared
  cluster's default (unset ⇒ authz allow-all). Not needed to exercise the
  datastore CRUD flows this environment is for.
- **cert-manager / zot / kpack** — not needed for the KeyValue/Postgres CRUD
  flows this environment targets; only relevant to App builds/custom domains.
