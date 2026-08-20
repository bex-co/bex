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
# Optional (Sign in with GitHub via Kratos oidc — docs/ADR012-auth.md § Social login).
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
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$(dirname "$0")/.."
# shellcheck source=lib/secret-install.sh
. "$script_dir/lib/secret-install.sh"

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

# The courier is enabled in the values (docs/ADR012-auth.md §11), so the Kratos chart
# injects COURIER_SMTP_CONNECTION_URI from this key as a NON-optional secretKeyRef
# — the pod won't start without it. Require presence + an smtp(s):// shape (never
# echo the value; it carries the SendGrid API key in prod).
require KRATOS_COURIER_SMTP_URI 1
case "$KRATOS_COURIER_SMTP_URI" in
  smtp://* | smtps://*) : ;;
  *) echo "error: KRATOS_COURIER_SMTP_URI must be an smtp:// or smtps:// URI" >&2; exit 1 ;;
esac

# Kratos OIDC provider fragment (a SECOND `--config` file the kratos Deployment
# loads via deployment.extraArgs; the client_secret must not live in git, so it
# rides this out-of-band Secret). Always emitted — enabled with the GitHub
# provider when both BEX_GITHUB_OIDC_* are set, else a valid `enabled: false` no-op
# so the mounted file always exists and Kratos never crashloops on a missing
# --config target. The Jsonnet claims→traits mapper is inlined via base64:// so
# no extra file mount is needed (docs/ADR012-auth.md § Social login).
oidc_fragment() {
  if [ -n "${BEX_GITHUB_OIDC_CLIENT_ID:-}" ] && [ -n "${BEX_GITHUB_OIDC_CLIENT_SECRET:-}" ]; then
    local mapper
    # Maps email always and the display name when GitHub supplies one (w4/m25:
    # the optional `name` identity trait) — social-login users get the same
    # populated name as password registrations, from one ingestion point.
    mapper="$(printf '%s' \
      'local claims = std.extVar('"'"'claims'"'"'); { identity: { traits: { email: claims.email } + (if std.objectHas(claims, '"'"'name'"'"') && claims.name != null then { name: claims.name } else {}) } }' \
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
    login:
      # ADR075 D8 (w6/m42, revised 2026-08-20): mirror the base password flow's
      # BACKSTOP — a returning unverified OIDC identity gets no session at
      # login. The Jsonnet mapper above maps only traits and marks nothing
      # verified; whether a GitHub address arrives verified is Kratos's provider
      # claims handling (email_verified). A provider-verified address passes
      # this hook untouched; an unverified one completes the same emailed-code
      # verification flow.
      after:
        oidc:
          hooks:
            - hook: require_verified_address
    registration:
      # Mirror the base password flow (keep in lockstep): show_verification_ui
      # routes a fresh signup with an UNVERIFIED address into the verification
      # flow (it emits nothing when the provider-verified address created no
      # verification flow — the common GitHub case), and session signs the new
      # identity in so verification is a seamless in-product step, not a
      # re-login.
      after:
        oidc:
          hooks:
            - hook: show_verification_ui
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
  echo "would apply secret $NS/kratos (keys: dsn secretsDefault secretsCookie secretsCipher smtpConnectionURI oidc.yaml)"
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

apply_secret "$NS" kratos Opaque \
  dsn "$(dsn kratos-db kratos)" \
  secretsDefault "$KRATOS_SECRETS_DEFAULT" \
  secretsCookie "$KRATOS_SECRETS_COOKIE" \
  secretsCipher "$KRATOS_SECRETS_CIPHER" \
  smtpConnectionURI "$KRATOS_COURIER_SMTP_URI" \
  oidc.yaml "$(oidc_fragment)"

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
