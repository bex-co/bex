# shellcheck shell=bash
# kratos_oidc_fragment — the Kratos `oidc.yaml` config fragment: a SECOND
# `--config` file the kratos Deployment loads (deployment.extraArgs), carried
# by the out-of-band `kratos` Secret because it holds client secrets and a
# bearer (docs/ADR012-auth.md §10, §9b). scripts/auth-secrets.sh writes it;
# scripts/kratos-oidc-fragment.test.sh pins it. It prints YAML and never reads
# a file.
#
# Social sign-in providers, Render's four, each on its own (w4/003, w7/m465,
# w7/m468):
#   BEX_GITHUB_OIDC_CLIENT_ID + BEX_GITHUB_OIDC_CLIENT_SECRET        → `github`
#   BEX_GOOGLE_OIDC_CLIENT_ID + BEX_GOOGLE_OIDC_CLIENT_SECRET        → `google`
#   BEX_GITLAB_OIDC_CLIENT_ID + BEX_GITLAB_OIDC_CLIENT_SECRET        → `gitlab`
#   BEX_BITBUCKET_OIDC_CLIENT_ID + BEX_BITBUCKET_OIDC_CLIENT_SECRET  → `bitbucket`
# No pair set → `oidc.enabled: false`, a valid no-op, so the mounted file
# always exists and Kratos never crashloops on a missing --config target.
#
# Bitbucket speaks OAuth 2.0 but not OpenID Connect, and Kratos has no
# bitbucket provider, so Kratos reaches it through Dex's bitbucket-cloud
# connector (ADR012 §10): `bitbucket` is a generic provider at Dex's issuer
# (DEX_ISSUER_URL, default https://auth.bex.co/dex) that signs in as Dex's
# static client `kratos` with DEX_KRATOS_CLIENT_SECRET, which auth-secrets.sh
# mints and keeps. The Bitbucket pair itself is Dex's (dex_bitbucket_config
# below); without DEX_KRATOS_CLIENT_SECRET the provider stays off.
#
# The workspace Google-login settings guard (w7/m465, docs/ADR012-auth.md §9b):
#   KRATOS_SETTINGS_GUARD_URL + KRATOS_SETTINGS_GUARD_TOKEN → a flow-interrupting
#   web_hook on the settings flow's profile and oidc methods, calling bex-api's
#   internal /internal/kratos/settings-guard. Kratos runs it before it saves the
#   change and fails the flow if the hook cannot answer.
#
# Test only (scripts/auth-oidc-e2e.sh's Dex stand-ins): KRATOS_OIDC_GOOGLE_ISSUER
# and KRATOS_OIDC_GITLAB_ISSUER register `google` and `gitlab` as generic OIDC
# providers at that issuer, because Kratos's built-in google provider always
# talks to accounts.google.com and its gitlab provider calls GitLab's own
# /oauth/authorize, /oauth/token and /oauth/userinfo, which Dex does not serve.
# Bitbucket needs no override: the e2e sets DEX_ISSUER_URL to its stand-in Dex.

# The Dex static client Kratos signs in as.
DEX_KRATOS_CLIENT_ID=kratos

# kratos_oidc_pair NAME — BEX_<NAME>_OIDC_CLIENT_ID and _SECRET are both set.
kratos_oidc_pair() {
  local id="BEX_${1}_OIDC_CLIENT_ID" secret="BEX_${1}_OIDC_CLIENT_SECRET"
  [ -n "${!id:-}" ] && [ -n "${!secret:-}" ]
}

kratos_oidc_fragment() {
  local github=0 google=0 gitlab=0 bitbucket=0 guard=0
  kratos_oidc_pair GITHUB && github=1
  kratos_oidc_pair GOOGLE && google=1
  kratos_oidc_pair GITLAB && gitlab=1
  # Bitbucket also needs Dex's static client secret to sign in with.
  kratos_oidc_pair BITBUCKET && [ -n "${DEX_KRATOS_CLIENT_SECRET:-}" ] && bitbucket=1
  [ -n "${KRATOS_SETTINGS_GUARD_URL:-}" ] && [ -n "${KRATOS_SETTINGS_GUARD_TOKEN:-}" ] && guard=1
  local any=$((github || google || gitlab || bitbucket))

  echo "selfservice:"
  echo "  methods:"
  echo "    oidc:"
  if [ "$any" = 0 ]; then
    echo "      enabled: false"
  else
    echo "      enabled: true"
    echo "      config:"
    echo "        providers:"
    [ "$github" = 1 ] && kratos_oidc_provider github github "" GitHub \
      "$BEX_GITHUB_OIDC_CLIENT_ID" "$BEX_GITHUB_OIDC_CLIENT_SECRET" unverified user:email
    # Google's ID token carries email_verified for the account's address, so
    # a Google sign-up needs no emailed code.
    [ "$google" = 1 ] && kratos_oidc_provider google google "${KRATOS_OIDC_GOOGLE_ISSUER:-}" Google \
      "$BEX_GOOGLE_OIDC_CLIENT_ID" "$BEX_GOOGLE_OIDC_CLIENT_SECRET" verified openid email profile
    # GitLab (gitlab.com): Kratos's built-in provider reads only GitLab's
    # /oauth/userinfo, which needs `openid` and carries email_verified with
    # `email` and the name with `profile`. Ory's GitLab guide also asks for
    # `read_user` (the REST API), which this provider never calls.
    [ "$gitlab" = 1 ] && kratos_oidc_provider gitlab gitlab "${KRATOS_OIDC_GITLAB_ISSUER:-}" GitLab \
      "$BEX_GITLAB_OIDC_CLIENT_ID" "$BEX_GITLAB_OIDC_CLIENT_SECRET" verified openid email profile
    # Bitbucket through Dex, signing in as Dex's static client: Dex's
    # bitbucket-cloud connector takes the account's primary, confirmed email
    # and always reports it verified; its `name` claim is the username.
    [ "$bitbucket" = 1 ] && kratos_oidc_provider bitbucket generic "$(dex_issuer_url)" Bitbucket \
      "$DEX_KRATOS_CLIENT_ID" "$DEX_KRATOS_CLIENT_SECRET" verified openid email profile
  fi
  if [ "$any" = 0 ] && [ "$guard" = 0 ]; then
    return 0
  fi
  echo "  flows:"
  if [ "$any" = 1 ]; then
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

# kratos_oidc_provider ID PROVIDER ISSUER LABEL CLIENT_ID CLIENT_SECRET MAPPER SCOPE...
# — one provider entry: Kratos's built-in PROVIDER, or, given ISSUER, a generic
# OIDC provider at that issuer under the same id. MAPPER is kratos_oidc_mapper's
# EXPR.
kratos_oidc_provider() {
  local id="$1" provider="$2" issuer="$3" label="$4" client_id="$5" client_secret="$6" mapper="$7"
  shift 7
  if [ -n "$issuer" ]; then
    printf '          - id: %s\n            provider: generic\n            issuer_url: "%s"\n' "$id" "$issuer"
  else
    printf '          - id: %s\n            provider: %s\n' "$id" "$provider"
  fi
  printf '            label: %s\n            client_id: "%s"\n            client_secret: "%s"\n' \
    "$label" "$client_id" "$client_secret"
  printf '            mapper_url: "base64://%s"\n            scope:\n' "$(kratos_oidc_mapper "$mapper")"
  printf '              - %s\n' "$@"
}

# dex_issuer_url — Dex's public issuer: the browser reaches its /auth and
# /callback there, and Kratos its discovery, token and keys endpoints.
dex_issuer_url() {
  local issuer="${DEX_ISSUER_URL:-https://auth.bex.co/dex}"
  printf '%s' "${issuer%/}"
}

# dex_bitbucket_config — Dex's whole config file, the Bitbucket bridge
# (ADR012 §10): one bitbucket-cloud connector on the Bitbucket pair and one
# static client for Kratos, whose redirect URI is Kratos's callback for the
# `bitbucket` provider (KRATOS_PUBLIC_URL, default https://auth.bex.co). It
# holds both secrets, so it rides the out-of-band `dex` Secret auth-secrets.sh
# writes. Storage is memory: a restart loses only logins in flight and
# rotates the signing keys, which Kratos re-fetches. With one connector and
# no password database, Dex sends the browser straight to Bitbucket. The
# connector id is part of every Dex subject Kratos stores: renaming it would
# orphan every linked Bitbucket account.
dex_bitbucket_config() {
  local issuer kratos
  issuer="$(dex_issuer_url)"
  kratos="${KRATOS_PUBLIC_URL:-https://auth.bex.co}"
  cat <<YAML
issuer: ${issuer}
storage:
  type: memory
web:
  http: 0.0.0.0:5556
telemetry:
  http: 0.0.0.0:5558
logger:
  level: info
  format: json
oauth2:
  responseTypes: [code]
  skipApprovalScreen: true
connectors:
  - type: bitbucket-cloud
    id: bitbucket
    name: Bitbucket
    config:
      clientID: "${BEX_BITBUCKET_OIDC_CLIENT_ID}"
      clientSecret: "${BEX_BITBUCKET_OIDC_CLIENT_SECRET}"
      redirectURI: ${issuer}/callback
staticClients:
  - id: ${DEX_KRATOS_CLIENT_ID}
    name: bex
    secret: "${DEX_KRATOS_CLIENT_SECRET}"
    redirectURIs:
      - ${kratos%/}/self-service/methods/oidc/callback/bitbucket
YAML
}

# kratos_settings_guard_hook METHOD — the settings guard's web_hook for one
# settings method. The Jsonnet body sends the identity as it would be saved,
# the URL of the request changing it (an oidc link runs on the provider
# callback, an unlink on the settings POST), and the provider the client says
# an unlink removes: the flow's transient_payload.unlink, which the dashboard
# sets because Kratos gives the hook neither the request body nor the
# credentials (w7/085). Empty when the client named none.
kratos_settings_guard_hook() {
  local q="'" body tp="ctx.flow.transient_payload"
  body="$(printf '%s' \
    "function(ctx) { identity_id: ctx.identity.id, email: if std.objectHas(ctx.identity.traits, ${q}email${q}) then ctx.identity.traits.email else ${q}${q}, request_url: ctx.request_url, unlink: if std.objectHas(ctx.flow, ${q}transient_payload${q}) && std.isObject(${tp}) && std.objectHas(${tp}, ${q}unlink${q}) && std.isString(${tp}.unlink) then ${tp}.unlink else ${q}${q} }" \
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
