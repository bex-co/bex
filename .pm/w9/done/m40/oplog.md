# w9/m40 op log — Key Value `spec.name` backfill rollout (2026-07-17, UTC)

All identity evidence is kind/name/UID tuples only — no Secret values, credentials,
`.env` contents, or kubeconfig contents were captured. Scratch artifacts (fetched
prod kubeconfig, throwaway CLI key env) lived in the session scratchpad and were
destroyed after the run.

## t001 — dev fleet (shared local kind/CAPD `bex` cluster)

`scripts/keyvalue-name-migrate.sh` ran dry-run **and** apply per namespace
`dev-1` … `dev-10`, then `--apply --all-namespaces` (22:49Z). Every run reported
`key-value display-name backfill: already complete` — the shared CAPD app cluster
was recreated on 2026-07-16 (see memory note `bare-bex-cluster-restore`) and holds
**zero** KeyValue CRs in any namespace, so no store predates the m6 identity split.

Because an all-empty sweep proves little about the apply path, a synthetic
legacy-shaped store (`metadata.name=legacy-kv-m40`, no `spec.name`) was created in
`dev-9` and walked through the full lifecycle:

- before: `uid=31782c09-a070-4f06-98bb-c85048612417`, `spec.name=[]`
- dry-run planned exactly `dev-9/legacy-kv-m40 spec.name <- legacy-kv-m40`
- apply backfilled it; rerun reported `already complete` (idempotence)
- after: **same UID**, `spec.name=[legacy-kv-m40]`;
  `keyvalue-rename-verify.sh compare` → "kept every recorded object identity"
- throwaway deleted afterwards; `dev-9` left with zero KeyValues (as found)

## t002 — production (Hetzner `bex` cluster)

Access per the m3/m39/m43 runbook: kubeconfig SSH-fetched via
`scripts/fetch-app-kubeconfig.sh` (never committed/printed); API at
`https://api.bex.co`, token exchange at `https://oauth.bex.co`, unmodified
official Render CLI (render-oss/cli, the repo-pinned build; `render vdev`).

**Backfill (22:49:39Z):** production inventory contained **zero** KeyValue CRs
(`kubectl get keyvalues.app.bex.co -A` → No resources found — nothing predates
m6's CRD upgrade, which is live: `spec.name` served by the keyvalues CRD).
Dry-run, apply, and idempotence rerun with `--all-namespaces` each reported
`already complete in all namespaces`.

**Official-CLI rename smoke (self-cleaning, namespace `default`):**

- `keyvalues create --name kv-m40-smoke --plan free` → minted
  `red-d9db3fp07a5s73dj32i0` (region fsn1); projected CR
  `uid=8037e678-f56e-4b03-a440-acfd869f3c49`, `spec.name=[kv-m40-smoke]`
- first `keyvalue-rename-cli-smoke.sh` run (22:51Z): the CLI rename
  `kv-m40-smoke → kv-m40-renamed` succeeded and routed by the `red-` id, but the
  identity compare **falsely failed** — see the script-defect section below
- rerun with the hardened verify script (22:52:53Z), rename
  `kv-m40-renamed → kv-m40-final`: captured 5 identities (KeyValue, StatefulSet,
  PVC, Secret, Service), CLI `keyvalues update --name` routed by
  `red-d9db3fp07a5s73dj32i0`, `keyvalues get kv-m40-final` re-resolved to the
  same id, compare → **"verified: default/red-d9db3fp07a5s73dj32i0 kept every
  recorded object identity"** (store status `available`, phase Ready)
- `keyvalues delete red-d9db3fp07a5s73dj32i0 --confirm` → `meta.deleted: true`;
  all `red-d9db3fp…` Kubernetes objects (KeyValue/StatefulSet/Service/Secret/PVC)
  confirmed garbage-collected

**Throwaway-credential cleanup (m3-precedent):** both scratch API keys revoked
via `DELETE /v1/api-keys` (204, which retires their Hydra clients), both scratch
Kratos identities deleted via the port-forwarded admin API (204), the two empty
workspace rows (`tea-d9db37p07a5s73dj32gg`, `tea-d9db3d5gr2bc73akhnf0`) removed
from the control-plane DB in one transaction (2 `tenant_members` + 2 `tenants`
rows; zero apps/usage/invite rows existed), and their two OpenFGA
`workspace … admin` tuples deleted. Production carries no residue from this run.

## Script defect the rollout surfaced (fixed here)

The first prod compare reported `Service … 41e40210-0db8-4e30-bb6f-59619b37434d`
"missing or replaced" — yet the live Service still existed with **that exact
UID** and a pre-rename creationTimestamp. Root cause: `snapshot()` in
`keyvalue-rename-verify.sh` (and its Postgres mirror) ran
`kubectl get … 2>/dev/null || true`, so a transient list failure (laptop →
Hetzner over the internet) silently dropped an entire resource from the "after"
snapshot and surfaced as identity churn. Fix (both scripts, kept byte-identical):
only an uninstalled resource type (`doesn't have a resource type` /
`could not find the requested resource`) may be skipped; any other list failure
exits loudly naming the resource. Regression coverage:
`scripts/rename-verify.test.sh` (clusterless kubectl fake; 20 assertions over
both scripts), wired into `.github/workflows/scripts.yml` together with the
previously unwired `keyvalue-name-migrate.test.sh`.

## t003 — Simplify (script changes only)

The one candidate simplification — capturing stderr inline via `2>&1` instead of
a temp file — was reviewed and rejected: on success kubectl may emit warnings to
stderr, which would corrupt the JSON handed to jq. The temp-file + RETURN-trap
form is the minimal correct shape; both siblings carry the identical hunk. No
other change to apply.

## t004 — Test coverage

`scripts/rename-verify.test.sh` (above) + CI wiring; `keyvalue-name-migrate.test.sh`
and `postgres-name-migrate.test.sh` re-run green alongside it.
