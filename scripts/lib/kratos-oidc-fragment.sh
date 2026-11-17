# shellcheck shell=bash
# kratos_oidc_fragment — the Kratos `oidc.yaml` config fragment: a SECOND
# `--config` file the kratos Deployment loads (deployment.extraArgs), carried
# by the out-of-band `kratos` Secret because it holds client secrets and a
# bearer (docs/ADR012-auth.md §10, §9b). scripts/auth-secrets.sh writes it;
# scripts/kratos-oidc-fragment.test.sh pins it. It prints YAML and never reads
# a file.
#
# Social sign-in providers, each on its own (w4/003, w7/m465):
#   BEX_GITHUB_OIDC_CLIENT_ID + BEX_GITHUB_OIDC_CLIENT_SECRET  → provider `github`
#   BEX_GOOGLE_OIDC_CLIENT_ID + BEX_GOOGLE_OIDC_CLIENT_SECRET  → provider `google`
# Neither pair set → `oidc.enabled: false`, a valid no-op, so the mounted file
# always exists and Kratos never crashloops on a missing --config target.
#
# The workspace Google-login settings guard (w7/m465, docs/ADR012-auth.md §9b):
#   KRATOS_SETTINGS_GUARD_URL + KRATOS_SETTINGS_GUARD_TOKEN → a flow-interrupting
#   web_hook on the settings flow's profile and oidc methods, calling bex-api's
#   internal /internal/kratos/settings-guard. Kratos runs it before it saves the
#   change and fails the flow if the hook cannot answer.
#
# Test only: KRATOS_OIDC_GOOGLE_ISSUER registers `google` as a generic OIDC
# provider at that issuer (scripts/auth-oidc-e2e.sh's Dex stand-in), because
# Kratos's built-in google provider always talks to accounts.google.com.

kratos_oidc_fragment() {
  local github=0 google=0 guard=0
  [ -n "${BEX_GITHUB_OIDC_CLIENT_ID:-}" ] && [ -n "${BEX_GITHUB_OIDC_CLIENT_SECRET:-}" ] && github=1
  [ -n "${BEX_GOOGLE_OIDC_CLIENT_ID:-}" ] && [ -n "${BEX_GOOGLE_OIDC_CLIENT_SECRET:-}" ] && google=1
  [ -n "${KRATOS_SETTINGS_GUARD_URL:-}" ] && [ -n "${KRATOS_SETTINGS_GUARD_TOKEN:-}" ] && guard=1

  echo "selfservice:"
  echo "  methods:"
  echo "    oidc:"
  if [ "$github" = 0 ] && [ "$google" = 0 ]; then
    echo "      enabled: false"
  else
    echo "      enabled: true"
    echo "      config:"
    echo "        providers:"
    [ "$github" = 1 ] && kratos_oidc_github_provider
    [ "$google" = 1 ] && kratos_oidc_google_provider
  fi
  if [ "$github" = 0 ] && [ "$google" = 0 ] && [ "$guard" = 0 ]; then
    return 0
  fi
  echo "  flows:"
  if [ "$github" = 1 ] || [ "$google" = 1 ]; then
    cat <<'YAML'
    login:
      # ADR075 D8 (w6/m42, revised 2026-08-20): mirror the base password flow's
      # BACKSTOP — a returning unverified OIDC identity gets no session at
      # login. A provider-verified address passes this hook untouched; an
      # unverified one completes the same emailed-code verification flow.
      after:
        oidc:
          hooks:
            - hook: require_verified_address
    registration:
      # Mirror the base password flow (keep in lockstep): show_verification_ui
      # routes a fresh signup with an UNVERIFIED address into the verification
      # flow (it emits nothing when the provider verified the address), and
      # session signs the new identity in.
      after:
        oidc:
          hooks:
            - hook: show_verification_ui
            - hook: session
YAML
  fi
  if [ "$guard" = 1 ]; then
    echo "    settings:"
    echo "      after:"
    kratos_settings_guard_hook profile
    kratos_settings_guard_hook oidc
  fi
}

# kratos_oidc_mapper EXPR — a base64 Jsonnet claims→traits mapper: the email
# always, the display name when the provider supplies one (w4/m25), and, when
# EXPR is "verified", the email marked verified if the provider says it is.
kratos_oidc_mapper() {
  local q="'" verified=""
  if [ "${1:-}" = verified ]; then
    verified=" + (if std.objectHas(claims, ${q}email_verified${q}) && claims.email_verified == true then { verified_addresses: [{ via: ${q}email${q}, value: claims.email }] } else {})"
  fi
  printf '%s' "local claims = std.extVar(${q}claims${q}); { identity: { traits: { email: claims.email } + (if std.objectHas(claims, ${q}name${q}) && claims.name != null then { name: claims.name } else {}) }${verified} }" \
    | base64 | tr -d '\n'
}

kratos_oidc_github_provider() {
  cat <<YAML
          - id: github
            provider: github
            label: GitHub
            client_id: "${BEX_GITHUB_OIDC_CLIENT_ID}"
            client_secret: "${BEX_GITHUB_OIDC_CLIENT_SECRET}"
            mapper_url: "base64://$(kratos_oidc_mapper)"
            scope:
              - user:email
YAML
}

# Google's ID token carries email_verified for the account's address, so the
# mapper marks it verified and a Google sign-up needs no emailed code.
kratos_oidc_google_provider() {
  if [ -n "${KRATOS_OIDC_GOOGLE_ISSUER:-}" ]; then
    cat <<YAML
          - id: google
            provider: generic
            issuer_url: "${KRATOS_OIDC_GOOGLE_ISSUER}"
YAML
  else
    cat <<YAML
          - id: google
            provider: google
YAML
  fi
  cat <<YAML
            label: Google
            client_id: "${BEX_GOOGLE_OIDC_CLIENT_ID}"
            client_secret: "${BEX_GOOGLE_OIDC_CLIENT_SECRET}"
            mapper_url: "base64://$(kratos_oidc_mapper verified)"
            scope:
              - openid
              - email
              - profile
YAML
}

# kratos_settings_guard_hook METHOD — the settings guard's web_hook for one
# settings method. The Jsonnet body sends the identity as it would be saved
# and the URL of the request changing it (an oidc link runs on the provider
# callback, an unlink on the settings POST).
kratos_settings_guard_hook() {
  local q="'" body
  body="$(printf '%s' \
    "function(ctx) { identity_id: ctx.identity.id, email: if std.objectHas(ctx.identity.traits, ${q}email${q}) then ctx.identity.traits.email else ${q}${q}, request_url: ctx.request_url }" \
    | base64 | tr -d '\n')"
  cat <<YAML
        $1:
          hooks:
            - hook: web_hook
              config:
                url: "${KRATOS_SETTINGS_GUARD_URL%/}/internal/kratos/settings-guard?method=$1"
                method: POST
                body: "base64://${body}"
                response:
                  parse: true
                auth:
                  type: api_key
                  config:
                    name: Authorization
                    value: "Bearer ${KRATOS_SETTINGS_GUARD_TOKEN}"
                    in: header
YAML
}
