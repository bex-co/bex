# w1/m72 t001 — the ten harnesses reconciled

Measured at `ce37c0a5` by normalizing every copy (all digit runs → `#`, which removes the per-N substitution without hiding structural differences) and hashing:

| script | distinct variants | groups |
| --- | --- | --- |
| `up.sh` | **8** | `w2`/`w7`/`w10` agree; `w1`, `w3`, `w4`, `w5`, `w6`, `w8`, `w9` each differ |
| `status.sh` | **4** | `w1`/`w2`/`w4`/`w6`/`w7`/`w8`/`w10` agree; `w3`, `w5`, `w9` differ |
| `down.sh` | **1** | all ten identical — the proof convergence was achievable |

133 tracked files, 7,129 tracked LOC.

## The union, and what each variant contributed

| capability | who had it | disposition |
| --- | --- | --- |
| kubeconfig from `kind get kubeconfig` | all | **keep** (fallback) |
| prefer `infra/local/bex.kubeconfig` (CAPD), copied so a shared refresh cannot strand a running env | `w3` | **keep** — the CAPD mock is the current substrate |
| normalize a `0.0.0.0` apiserver URL to loopback (cert only covers `127.0.0.1`) | `w4`, `w9` (two spellings) | **keep**, one spelling |
| `bex.co/pool=platform` label on the control-plane node | `w3` | **keep** — the local Ory overlays select it |
| `pod-security…/enforce=privileged` on the auth namespace | `w3` | **keep** |
| preflight: required tools, `kind` cluster present, cluster reachable, **default StorageClass** | `w5` | **keep** — each replaces an opaque mid-run failure |
| `ensure_cnpg`: install the operator if absent, wait for ready, pin it to the control-plane node, tolerate "operator down but Clusters exist" | `w5` | **keep** |
| CNPG Clusters (kratos/hydra/bex), Mailpit, out-of-band Kratos/Hydra secrets, Ory helm installs | all | **keep** |
| Loki + Alloy log-shipper in `monitoring`, `BEX_LOKI_URL` | `w5` | **keep**, default on, `DEV_ENV_OBSERVABILITY=0` to skip |
| `hydra-public` port-forward | `w3`, `w9` | **keep** — without it the Hydra issuer points at nothing |
| `auth-bootstrap-client.sh` (platform OAuth2 clients) | `w3`, `w9` | **keep** |
| Mailpit **SMTP** forward + `BEX_SMTP_ADDR`/`BEX_SMTP_FROM` | `w1` | **keep** — the drift this milestone was filed over |
| throwaway GitHub-App identity (local RSA key, fake id/slug) | `w8` | **keep** — enables anonymous public-repo blueprint fetches |
| self-healing port-forward supervisor | all | **keep**, with backoff + give-up added (see below) |
| `bex-api` 5-attempt start retry | all | **keep** |
| verification inventory (pass/fail assertions incl. a Loki push→query round-trip) | `w5`'s `status.sh` | **keep** |
| `hydra-public` HTTP check in `status.sh` | `w3`, `w9` | **keep** |
| `bootstrap-key.sh` (mints a real tenant-bound API key for CLI-compat work) | `w9` | **keep where it is** — a per-workstream extra, already parameterized by `BEX_DEV_ENV_DIR`, not part of up/down/status |

### The full `bex-api` environment union

| variable | set by, before | now |
| --- | --- | --- |
| `BEX_API_ADDR`, `BEX_CP_ADDR`, `BEX_API_NAMESPACE`, `BEX_API_CORS_ORIGIN`, `BEX_KRATOS_URL`, `BEX_KRATOS_ADMIN_URL`, `BEX_HYDRA_ADMIN_URL`, `BEX_CP_DB_URI`, `BEX_CP_APPS_NAMESPACE`, `BEX_CP_IDENTITY`, `BEX_BASE_DOMAIN` | all ten | all |
| `BEX_ALLOW_INSECURE_AUTHZ` | `w5`, `w8` only | **all** |
| `BEX_CP_INSECURE` | `w1`, `w5`, `w8`, `w9` | all |
| `BEX_DASHBOARD_URL` | `w1`, `w4`, `w6`, `w9` | all |
| `BEX_REGION` | `w4`, `w6`, `w9` | all |
| `BEX_OAUTH_ISSUER` | `w3`, `w9` | all |
| `BEX_BUILD_NAMESPACE` | `w3` | all |
| `BEX_LOKI_URL` | `w5` | all |
| `BEX_SMTP_ADDR`, `BEX_SMTP_FROM`, `BEX_REQUIRE_VERIFIED_INVITE_EMAIL` | `w1` | all |
| `BEX_GITHUB_APP_ID`, `_PRIVATE_KEY`, `_SLUG` | `w8` | all |

## Three defects the reconciliation found

**1. Five of the ten harnesses could not start `bex-api` at all.** `cmd/api` refuses to start when `BEX_CP_DB_URI` is set and `BEX_OPENFGA_URL` is not, unless `BEX_ALLOW_INSECURE_AUTHZ=1` (w1/m65 F16, fail-closed authorization). Only `w5` and `w8` set it. `w1`/`w3`/`w9` set `BEX_CP_INSECURE` — a different variable — and `w2`/`w4`/`w6`/`w7`/`w10` set neither, so their `up.sh` would exhaust its five start attempts and exit 1. The shared harness sets it for every N.

**2. The Hydra issuer pointed at a port nothing served, in eight of ten.** `values/hydra.values.yaml` set `issuer: http://localhost:57000+N*10` — the **Kratos-admin** slot. Only `w3` and `w9`, the two that also forwarded `hydra-public`, used `58000+N*10` and were right. The fold takes the working spelling and always forwards `hydra-public`.

**3. Two port families collided on `58000+N*10`.** `w5` assigned it to Loki; `w3`/`w9` to `hydra-public`; `w1`'s Mailpit-SMTP forward hard-coded `58010` — a third claim on the same slot. The converged map gives each its own family (`hydra-public` 58000, Loki 59000, Mailpit-SMTP 60000), so `w3`/`w9` keep the value they already used and only `w5`'s Loki port moves (58050 → 59050).

## Dropped, with reasons

- **Per-copy `db/*.yaml`, `mailpit/`, `values/`, `rbac-dev-N.yaml`** (100 files) — identical modulo N; now templates under `scripts/dev-env/` rendered at apply time.
- **`ports.env` as an input** — it is regenerated as a *record* of the derivation and checked against it by `scripts/dev-env.test.sh`. It was already stale in `w5` (the Loki collision), which is exactly the failure a second source of truth produces.
- **Nothing else.** Every behavioural difference above is kept.

## Override mechanism

An optional `.pm/wN/dev-N/override.env`, sourced after the derivation. It may set knobs (`DEV_ENV_OBSERVABILITY=0` on a constrained machine, `BEX_BOOTSTRAP_CLIENT_SECRET`, …) but **may not** change `DEV_NS`/`DEV_AUTH_NS`: the script checks and refuses, because cross-N isolation and the cluster-scoped prune scope both depend on them. No workstream needed one at migration time, so none was created.

## One fix beyond reconciliation

The `while true; kubectl port-forward; sleep 1` supervisor every copy shared has no backoff and no give-up. When the CAPD cluster underneath `dev-5` died, six of its port-forwards wrote an error per second until they filled **3.9 GB** (six logs of ~677 MB, all timestamped after the cluster went away). The shared `forward()` keeps the self-healing behaviour, backs off to 10s after five immediate failures, and gives up after 60 with a message naming the fix. Truncate-on-`up` (t004) bounds the rest.
