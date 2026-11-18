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
#   KRATOS_COURIER_SMTP_URI   kratos courier SMTP relay URI  (smtp:// or smtps://)
#     Prod (SendGrid): smtp://apikey:<sendgrid-api-key>@smtp.sendgrid.net:587
#       Port 587 + STARTTLS, NOT 465 — Hetzner blocks outbound 25/465; 587/2525 are
#       open. With :465 the courier loops on `dial tcp: i/o timeout` and mail never
#       lands, while the flow still reports "sent_email" (docs/ADR012-auth.md §11).
#     Local (Mailpit): smtp://mailpit.auth.svc:1025/?disable_starttls=true  (no TLS)
#     Written into the kratos Secret under the chart key `smtpConnectionURI`; the
#     courier is enabled in the values, so the key must exist (docs/ADR012-auth.md §11).
#
# Optional (social sign-in via Kratos oidc — docs/ADR012-auth.md §10). Each pair
# enables its provider in the kratos Secret's `oidc.yaml` fragment on its own;
# neither set ⇒ the fragment is written disabled (a valid no-op), so flipping
# social login on is purely `.env` + a re-run, no git change:
#   BEX_GITHUB_OIDC_CLIENT_ID     GitHub OAuth app client id
#   BEX_GITHUB_OIDC_CLIENT_SECRET GitHub OAuth app client secret
#   BEX_GOOGLE_OIDC_CLIENT_ID     Google OAuth client id (w7/m465)
#   BEX_GOOGLE_OIDC_CLIENT_SECRET Google OAuth client secret
#   BEX_GITLAB_OIDC_CLIENT_ID     GitLab application id (w7/m468)
#   BEX_GITLAB_OIDC_CLIENT_SECRET GitLab application secret
#   BEX_BITBUCKET_OIDC_CLIENT_ID     Bitbucket OAuth consumer key (w7/m468)
#   BEX_BITBUCKET_OIDC_CLIENT_SECRET Bitbucket OAuth consumer secret
#
# Bitbucket is not OpenID Connect, so Kratos reaches it through Dex (the `dex`
# Application, deploy/gitops/charts/dex; docs/ADR012-auth.md §10). With the
# Bitbucket pair set, this writes Dex's whole config into auth/dex (key
# config.yaml: the bitbucket-cloud connector and Kratos's static client),
# scales deploy/dex to one replica (restarting it when the config changed) and
# waits for it before Kratos gets the `bitbucket` button. Kratos's Dex client
# secret is minted once and kept in auth/dex (key kratosClientSecret; ROTATE=1
# mints a new one). With the pair unset, deploy/dex is scaled to zero and
# auth/dex deleted, so nothing of Bitbucket runs. DEX_ISSUER_URL (default
# https://auth.bex.co/dex, the path the dex Ingress routes) and KRATOS_PUBLIC_URL
# (default https://auth.bex.co) override where Dex and Kratos answer.
#
# The workspace Google-login settings guard (w7/m465, docs/ADR012-auth.md §9b):
# the fragment also wires Kratos's settings flow to bex-api's internal settings
# guard. Its bearer is minted here once and kept in bex-system/
# bex-kratos-settings-guard (BEX_KRATOS_SETTINGS_GUARD_TOKEN, read back on every
# run; ROTATE=1 mints a new one), which bex-api's Deployment reads. Nothing to
# set in .env. KRATOS_SETTINGS_GUARD_URL overrides where Kratos calls bex-api
# (default: the in-cluster bex-api internal listener).
#
# The DSNs are composed from the CNPG-generated DB credentials (Secrets
# kratos-db-app / hydra-db-app / openfga-db-app, created by the Clusters in
# deploy/gitops/charts/auth-dbs/) — DB passwords never live in .env.
#
# Usage: scripts/auth-secrets.sh             # create/update the Secrets (idempotent)
#        DRY_RUN=1 scripts/auth-secrets.sh   # print what would be applied (names only)
# Requires: kubectl (respects $KUBECONFIG).
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$(dirname "$0")/.."
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"
# shellcheck source=lib/kratos-oidc-fragment.sh
. "$script_dir/lib/kratos-oidc-fragment.sh"

NS=auth

# Load .env when present (local use); in CI the keys arrive as environment
# variables instead (deploy.yml exports them from GitHub Actions secrets).
if [ -f .env ]; then
  . scripts/lib/env-file.sh && bex_load_env ./.env || exit 1
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

# The courier is enabled in the values (docs/ADR012-auth.md §11), so the Kratos chart
# injects COURIER_SMTP_CONNECTION_URI from this key as a NON-optional secretKeyRef
# — the pod won't start without it. Require presence + an smtp(s):// shape (never
# echo the value; it carries the SendGrid API key in prod).
require KRATOS_COURIER_SMTP_URI 1
case "$KRATOS_COURIER_SMTP_URI" in
  smtp://* | smtps://*) : ;;
  *) echo "error: KRATOS_COURIER_SMTP_URI must be an smtp:// or smtps:// URI" >&2; exit 1 ;;
esac

# The Kratos OIDC fragment (a SECOND `--config` file the kratos Deployment loads
# via deployment.extraArgs; its client secrets and bearer must not live in git,
# so it rides this out-of-band Secret) is scripts/lib/kratos-oidc-fragment.sh's
# kratos_oidc_fragment, pinned by scripts/kratos-oidc-fragment.test.sh.
KRATOS_SETTINGS_GUARD_URL="${KRATOS_SETTINGS_GUARD_URL:-http://bex-api.bex-system.svc.cluster.local:8091}"

if [ "${DRY_RUN:-}" = "1" ]; then
  providers=""
  for provider in github google gitlab bitbucket; do
    kratos_oidc_pair "$(tr '[:lower:]' '[:upper:]' <<<"$provider")" && providers="$providers $provider"
  done
  if [ -n "$providers" ]; then
    echo "would apply secret $NS/kratos oidc.yaml key: providers ENABLED:$providers"
  else
    echo "would apply secret $NS/kratos oidc.yaml key: oidc disabled (no BEX_*_OIDC_* pair set)"
  fi
  echo "would apply secret $NS/kratos oidc.yaml key: settings guard -> $KRATOS_SETTINGS_GUARD_URL/internal/kratos/settings-guard"
  if kratos_oidc_pair BITBUCKET; then
    echo "would apply secret $NS/dex (keys: config.yaml kratosClientSecret): Bitbucket's bridge, issuer $(dex_issuer_url)"
    echo "would scale $NS/deploy/dex to 1 replica (restarted when its config changed)"
  else
    echo "would scale $NS/deploy/dex to 0 replicas and delete secret $NS/dex (BEX_BITBUCKET_OIDC_* unset)"
  fi
  echo "would ensure namespace $NS"
  echo "would apply secret $NS/kratos (keys: dsn secretsDefault secretsCookie secretsCipher smtpConnectionURI oidc.yaml)"
  echo "would apply secret bex-system/bex-kratos-settings-guard (keys: BEX_KRATOS_SETTINGS_GUARD_TOKEN)"
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

# The settings guard's bearer: kept once minted, so Kratos and bex-api agree
# across runs. Never printed.
KRATOS_SETTINGS_GUARD_TOKEN=""
if [ "${ROTATE:-0}" != "1" ]; then
  KRATOS_SETTINGS_GUARD_TOKEN="$(kubectl -n bex-system get secret bex-kratos-settings-guard \
    -o jsonpath='{.data.BEX_KRATOS_SETTINGS_GUARD_TOKEN}' 2>/dev/null | base64 --decode 2>/dev/null || true)"
fi
if [ -z "$KRATOS_SETTINGS_GUARD_TOKEN" ]; then
  command -v openssl >/dev/null || { echo "error: openssl is required to mint the settings guard bearer" >&2; exit 1; }
  KRATOS_SETTINGS_GUARD_TOKEN="$(openssl rand -hex 32)"
  echo "minted a new Kratos settings guard bearer"
fi
kubectl get namespace bex-system >/dev/null 2>&1 || kubectl create namespace bex-system >/dev/null
apply_secret bex-system bex-kratos-settings-guard Opaque \
  BEX_KRATOS_SETTINGS_GUARD_TOKEN "$KRATOS_SETTINGS_GUARD_TOKEN"
export KRATOS_SETTINGS_GUARD_URL KRATOS_SETTINGS_GUARD_TOKEN

# Bitbucket's bridge (docs/ADR012-auth.md §10): Dex comes up before Kratos
# offers the button, and goes away after it. Argo ignores deploy/dex's replica
# count (deploy/gitops/charts/dex/deployment.yaml), so this scale holds.
dex_scale() { # REPLICAS
  kubectl -n "$NS" scale deploy/dex --replicas="$1" >/dev/null
}
if kratos_oidc_pair BITBUCKET; then
  # Kratos's client secret (kept unless ROTATE=1) and the config Dex runs.
  { read -r DEX_KRATOS_CLIENT_SECRET || true; read -r dex_old || true; } < <(kubectl -n "$NS" get secret dex --ignore-not-found \
    -o go-template='{{with .data.kratosClientSecret}}{{. | base64decode}}{{end}}{{"\n"}}{{index .data "config.yaml"}}{{"\n"}}' 2>/dev/null || true)
  if [ "${ROTATE:-0}" = "1" ] || [ -z "${DEX_KRATOS_CLIENT_SECRET:-}" ]; then
    command -v openssl >/dev/null || { echo "error: openssl is required to mint Kratos's Dex client secret" >&2; exit 1; }
    DEX_KRATOS_CLIENT_SECRET="$(openssl rand -hex 32)"
    echo "minted a new Dex client secret for Kratos"
  fi
  export DEX_KRATOS_CLIENT_SECRET
  dex_config="$(dex_bitbucket_config)"
  apply_secret "$NS" dex Opaque \
    config.yaml "$dex_config" \
    kratosClientSecret "$DEX_KRATOS_CLIENT_SECRET"
  # A first install can run before Argo has synced the dex Application:
  # give it about five minutes.
  dex_replicas=""
  for _ in $(seq 1 60); do
    dex_replicas="$(kubectl -n "$NS" get deploy dex -o jsonpath='{.spec.replicas}' 2>/dev/null)" && break
    sleep 5
  done
  [ -n "$dex_replicas" ] \
    || { echo "error: deploy/dex never appeared in $NS — is the dex Argo Application synced? (kubectl -n argocd get application dex)" >&2; exit 1; }
  if [ "$dex_replicas" = 0 ]; then
    dex_scale 1
  elif [ "${dex_old:-}" != "$(secret_b64 "$dex_config")" ]; then
    kubectl -n "$NS" rollout restart deploy/dex >/dev/null # Dex reads its config only at start
  fi
  kubectl -n "$NS" rollout status deploy/dex --timeout=180s >/dev/null \
    || { echo "error: deploy/dex did not become ready — kubectl -n $NS logs deploy/dex" >&2; exit 1; }
  echo "Bitbucket's bridge is up: Dex at $(dex_issuer_url)"
else
  unset DEX_KRATOS_CLIENT_SECRET
fi

apply_secret "$NS" kratos Opaque \
  dsn "$(dsn kratos-db kratos)" \
  secretsDefault "$KRATOS_SECRETS_DEFAULT" \
  secretsCookie "$KRATOS_SECRETS_COOKIE" \
  secretsCipher "$KRATOS_SECRETS_CIPHER" \
  smtpConnectionURI "$KRATOS_COURIER_SMTP_URI" \
  oidc.yaml "$(kratos_oidc_fragment)"

# Without the pair, Dex goes away once Kratos no longer offers the button.
# One read on the usual deploy, where Dex is already off and its Secret gone.
if ! kratos_oidc_pair BITBUCKET; then
  dex_left="$(kubectl -n "$NS" get deploy/dex secret/dex --ignore-not-found \
    -o jsonpath='{range .items[*]}{.kind}:{.spec.replicas}{"\n"}{end}')"
  if grep -Eq '^Deployment:[1-9]' <<<"$dex_left"; then dex_scale 0; fi
  if grep -q '^Secret:' <<<"$dex_left"; then kubectl -n "$NS" delete secret dex >/dev/null; fi
fi

apply_secret "$NS" hydra Opaque \
  dsn "$(dsn hydra-db hydra)" \
  secretsSystem "$HYDRA_SECRETS_SYSTEM" \
  secretsCookie "$HYDRA_SECRETS_COOKIE" \
  oidcPairwiseSalt "$HYDRA_OIDC_PAIRWISE_SALT"

# Key names `uri`/`keys` are what the openfga chart's templates hardcode
# (datastore.uriSecret / authn.preshared.keysSecret).
apply_secret "$NS" openfga Opaque \
  uri "$(dsn openfga-db openfga)" \
  keys "$OPENFGA_PRESHARED_KEY"

# bex-api (ns bex-system) presents the same preshared key to OpenFGA — it can't
# mount the auth-namespace Secret, so it gets its own copy.
apply_secret bex-system bex-openfga Opaque token "$OPENFGA_PRESHARED_KEY"

# bex-api's invite mailer (docs/ADR012-auth.md §11, w4/m12) shares the courier's relay.
# ADDR/FROM are non-secret and baked into the bex-api Deployment; only the
# credentials ride a Secret (referenced optional:true, so absent ⇒ mailer nil ⇒
# invites recorded but not emailed). Create bex-smtp only when a credential is set.
if [ -n "${BEX_SMTP_USERNAME:-}" ] || [ -n "${BEX_SMTP_PASSWORD:-}" ]; then
  apply_secret bex-system bex-smtp Opaque \
    username "${BEX_SMTP_USERNAME:-}" password "${BEX_SMTP_PASSWORD:-}"
fi
