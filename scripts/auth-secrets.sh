#!/usr/bin/env bash
# Create the Kubernetes Secrets Ory Kratos + Hydra + OpenFGA consume, out-of-band
# of GitOps (no secret material in git or Argo-managed manifests — repo rule;
# prod-grade committed secrets are w1/m7 SOPS/sealed-secrets territory).
#
# Reads the repo-local .env (gitignored — never commit or print it). Required keys
# (names only; values are never echoed):
#   KRATOS_SECRETS_DEFAULT    kratos "default" secret        (>= 16 chars)
#   KRATOS_SECRETS_COOKIE     kratos cookie secret           (>= 16 chars)
#   KRATOS_SECRETS_CIPHER     kratos cipher secret           (exactly 32 chars)
#   HYDRA_SECRETS_SYSTEM      hydra system secret            (>= 16 chars)
#   HYDRA_SECRETS_COOKIE      hydra cookie secret            (>= 16 chars)
#   HYDRA_OIDC_PAIRWISE_SALT  hydra pairwise subject salt    (>= 8 chars)
#   OPENFGA_PRESHARED_KEY     openfga API preshared key      (>= 16 chars)
#
# Optional (Sign in with GitHub via Kratos oidc — docs/auth.md § Social login).
# When BOTH are set, the kratos Secret gains an `oidc.yaml` fragment enabling the
# GitHub provider; unset ⇒ the fragment is written disabled (a valid no-op), so
# flipping social login on is purely `.env` + a re-run, no git change:
#   BEX_GITHUB_OIDC_CLIENT_ID     GitHub OAuth app client id
#   BEX_GITHUB_OIDC_CLIENT_SECRET GitHub OAuth app client secret
#
# The DSNs are composed from the CNPG-generated DB credentials (Secrets
# kratos-db-app / hydra-db-app / openfga-db-app, created by the Clusters in
# deploy/gitops/charts/auth-dbs/) — DB passwords never live in .env.
#
# Usage: scripts/auth-secrets.sh             # create/update the Secrets (idempotent)
#        DRY_RUN=1 scripts/auth-secrets.sh   # print what would be applied (names only)
# Requires: kubectl (respects $KUBECONFIG).
set -euo pipefail
cd "$(dirname "$0")/.."

NS=auth

# Load .env when present (local use); in CI the keys arrive as environment
# variables instead (deploy.yml exports them from GitHub Actions secrets).
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

# require NAME LEN [exact] — assert the .env key exists and is at least (or,
# with "exact", exactly) LEN characters. Never prints the value.
require() {
  local name="$1" len="$2" val="${!1:-}"
  [ -n "$val" ] || { echo "error: $name is missing or empty (.env or environment)" >&2; exit 1; }
  if [ "${3:-}" = "exact" ]; then
    [ "${#val}" -eq "$len" ] || { echo "error: $name must be exactly $len characters (got ${#val})" >&2; exit 1; }
  else
    [ "${#val}" -ge "$len" ] || { echo "error: $name must be at least $len characters (got ${#val})" >&2; exit 1; }
  fi
}

require KRATOS_SECRETS_DEFAULT 16
require KRATOS_SECRETS_COOKIE 16
require KRATOS_SECRETS_CIPHER 32 exact
require HYDRA_SECRETS_SYSTEM 16
require HYDRA_SECRETS_COOKIE 16
require HYDRA_OIDC_PAIRWISE_SALT 8
require OPENFGA_PRESHARED_KEY 16

# Kratos OIDC provider fragment (a SECOND `--config` file the kratos Deployment
# loads via deployment.extraArgs; the client_secret must not live in git, so it
# rides this out-of-band Secret). Always emitted — enabled with the GitHub
# provider when both BEX_GITHUB_OIDC_* are set, else a valid `enabled: false` no-op
# so the mounted file always exists and Kratos never crashloops on a missing
# --config target. The Jsonnet claims→traits mapper is inlined via base64:// so
# no extra file mount is needed (docs/auth.md § Social login).
oidc_fragment() {
  if [ -n "${BEX_GITHUB_OIDC_CLIENT_ID:-}" ] && [ -n "${BEX_GITHUB_OIDC_CLIENT_SECRET:-}" ]; then
    local mapper
    mapper="$(printf '%s' \
      'local claims = std.extVar('"'"'claims'"'"'); { identity: { traits: { email: claims.email } } }' \
      | base64 | tr -d '\n')"
    cat <<YAML
selfservice:
  methods:
    oidc:
      enabled: true
      config:
        providers:
          - id: github
            provider: github
            client_id: "${BEX_GITHUB_OIDC_CLIENT_ID}"
            client_secret: "${BEX_GITHUB_OIDC_CLIENT_SECRET}"
            mapper_url: "base64://${mapper}"
            scope:
              - user:email
  flows:
    registration:
      # Kratos doesn't issue a session after OIDC registration unless told to —
      # mirror the password flow's session hook (base kratos.values.yaml) so a
      # first-time GitHub sign-in lands authenticated, not back on the login page.
      after:
        oidc:
          hooks:
            - hook: session
YAML
  else
    cat <<'YAML'
selfservice:
  methods:
    oidc:
      enabled: false
YAML
  fi
}

if [ "${DRY_RUN:-}" = "1" ]; then
  if [ -n "${BEX_GITHUB_OIDC_CLIENT_ID:-}" ] && [ -n "${BEX_GITHUB_OIDC_CLIENT_SECRET:-}" ]; then
    echo "would apply secret $NS/kratos oidc.yaml key: GitHub provider ENABLED"
  else
    echo "would apply secret $NS/kratos oidc.yaml key: oidc disabled (BEX_GITHUB_OIDC_* unset)"
  fi
  echo "would ensure namespace $NS"
  echo "would apply secret $NS/kratos (keys: dsn secretsDefault secretsCookie secretsCipher oidc.yaml)"
  echo "would apply secret $NS/hydra (keys: dsn secretsSystem secretsCookie oidcPairwiseSalt)"
  echo "would apply secret $NS/openfga (keys: uri keys)"
  exit 0
fi

# dsn CLUSTER DB — postgres:// DSN for a CNPG cluster's app user, from the
# credentials Secret CNPG generated (<cluster>-app). Passwords stay in the pipe.
dsn() {
  local cluster="$1" db="$2" user pass
  { read -r user; read -r pass; } < <(kubectl -n "$NS" get secret "$cluster-app" \
    -o go-template='{{.data.username | base64decode}}{{"\n"}}{{.data.password | base64decode}}')
  [ -n "$user" ] && [ -n "$pass" ] || { echo "error: Secret $NS/$cluster-app has no username/password — is the CNPG Cluster $cluster up?" >&2; return 1; }
  printf 'postgres://%s:%s@%s-rw.%s.svc.cluster.local:5432/%s?sslmode=require' \
    "$user" "$pass" "$cluster" "$NS" "$db"
}

kubectl get namespace "$NS" >/dev/null 2>&1 || kubectl create namespace "$NS" >/dev/null

kubectl create secret generic kratos -n "$NS" \
  --from-literal=dsn="$(dsn kratos-db kratos)" \
  --from-literal=secretsDefault="$KRATOS_SECRETS_DEFAULT" \
  --from-literal=secretsCookie="$KRATOS_SECRETS_COOKIE" \
  --from-literal=secretsCipher="$KRATOS_SECRETS_CIPHER" \
  --from-literal=oidc.yaml="$(oidc_fragment)" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic hydra -n "$NS" \
  --from-literal=dsn="$(dsn hydra-db hydra)" \
  --from-literal=secretsSystem="$HYDRA_SECRETS_SYSTEM" \
  --from-literal=secretsCookie="$HYDRA_SECRETS_COOKIE" \
  --from-literal=oidcPairwiseSalt="$HYDRA_OIDC_PAIRWISE_SALT" \
  --dry-run=client -o yaml | kubectl apply -f -

# Key names `uri`/`keys` are what the openfga chart's templates hardcode
# (datastore.uriSecret / authn.preshared.keysSecret).
kubectl create secret generic openfga -n "$NS" \
  --from-literal=uri="$(dsn openfga-db openfga)" \
  --from-literal=keys="$OPENFGA_PRESHARED_KEY" \
  --dry-run=client -o yaml | kubectl apply -f -

# bex-api (ns bex-system) presents the same preshared key to OpenFGA — it can't
# mount the auth-namespace Secret, so it gets its own copy.
kubectl create secret generic bex-openfga -n bex-system \
  --from-literal=token="$OPENFGA_PRESHARED_KEY" \
  --dry-run=client -o yaml | kubectl apply -f -
