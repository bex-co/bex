#!/usr/bin/env bash
# Pins scripts/lib/kratos-oidc-fragment.sh, the Kratos oidc.yaml config
# fragment scripts/auth-secrets.sh writes (docs/ADR012-auth.md §10, §9b;
# w7/m465, w7/m468): each of the four social providers is enabled on its own
# pair of credentials, none leaves oidc disabled, the login/registration hooks
# mirror the password flow whenever a provider is on, and the settings guard
# hooks the profile and oidc settings methods with a parsed, authenticated
# web_hook. Bitbucket is a generic provider at Dex's issuer, and Dex's config
# (dex_bitbucket_config) is the other half of that bridge.
# Requires: bash, yq (mikefarah v4), base64.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/lib/kratos-oidc-fragment.sh

fail=0
check() { # description, command...
  local what="$1"
  shift
  if "$@"; then echo "ok   $what"; else echo "FAIL $what" >&2; fail=1; fi
}

lib() { # FUNCTION, env assignments... → FUNCTION's output in a clean environment
  local fn="$1"
  shift
  env -i PATH="$PATH" "$@" bash -c '. scripts/lib/kratos-oidc-fragment.sh; "$0"' "$fn"
}
fragment() { lib kratos_oidc_fragment "$@"; }

q() { yq -r "$2" <<<"$1"; }

GH=(BEX_GITHUB_OIDC_CLIENT_ID=gh-id BEX_GITHUB_OIDC_CLIENT_SECRET=gh-secret)
GOOGLE=(BEX_GOOGLE_OIDC_CLIENT_ID=google-id BEX_GOOGLE_OIDC_CLIENT_SECRET=google-secret)
GITLAB=(BEX_GITLAB_OIDC_CLIENT_ID=gitlab-id BEX_GITLAB_OIDC_CLIENT_SECRET=gitlab-secret)
# Bitbucket's pair is Dex's; Kratos signs in to Dex with the static client
# secret auth-secrets.sh mints.
BITBUCKET=(BEX_BITBUCKET_OIDC_CLIENT_ID=bb-id BEX_BITBUCKET_OIDC_CLIENT_SECRET=bb-secret DEX_KRATOS_CLIENT_SECRET=dex-kratos-secret)
GUARD=(KRATOS_SETTINGS_GUARD_URL=http://bex-api.bex-system.svc.cluster.local:8091/ KRATOS_SETTINGS_GUARD_TOKEN=guard-token)

none="$(fragment)"
github="$(fragment "${GH[@]}")"
google="$(fragment "${GOOGLE[@]}")"
gitlab="$(fragment "${GITLAB[@]}")"
bitbucket="$(fragment "${BITBUCKET[@]}")"
both="$(fragment "${GH[@]}" "${GOOGLE[@]}")"
all="$(fragment "${BITBUCKET[@]}" "${GITLAB[@]}" "${GOOGLE[@]}" "${GH[@]}")"
half="$(fragment BEX_GOOGLE_OIDC_CLIENT_ID=google-id)" # a client id without its secret enables nothing
gitlab_half="$(fragment BEX_GITLAB_OIDC_CLIENT_SECRET=gitlab-secret)"
bitbucket_half="$(fragment BEX_BITBUCKET_OIDC_CLIENT_ID=bb-id DEX_KRATOS_CLIENT_SECRET=dex-kratos-secret)"
# The Bitbucket pair without Dex's client secret: Kratos could not sign in to Dex.
bitbucket_no_dex="$(fragment BEX_BITBUCKET_OIDC_CLIENT_ID=bb-id BEX_BITBUCKET_OIDC_CLIENT_SECRET=bb-secret)"

for name in none github google gitlab bitbucket both all half gitlab_half bitbucket_half bitbucket_no_dex; do
  check "$name: parses as YAML" bash -c 'yq -e . >/dev/null <<<"$1"' _ "${!name}"
done

ids() { q "$1" '[.selfservice.methods.oidc.config.providers[].id] | join(",")'; }
provider() { # fragment, provider id → that provider's entry as YAML
  q "$1" ".selfservice.methods.oidc.config.providers[] | select(.id == \"$2\")"
}

check "no pair: oidc disabled, no flows" \
  test "$(q "$none" '.selfservice.methods.oidc.enabled')/$(q "$none" '.selfservice.flows')" = "false/null"
for name in half gitlab_half bitbucket_half bitbucket_no_dex; do
  check "$name: oidc disabled, no flows" \
    test "$(q "${!name}" '.selfservice.methods.oidc.enabled')/$(q "${!name}" '.selfservice.flows')" = "false/null"
done
check "GitHub pair only: only github" test "$(ids "$github")" = "github"
check "Google pair only: only google" test "$(ids "$google")" = "google"
check "GitLab pair only: only gitlab" test "$(ids "$gitlab")" = "gitlab"
check "Bitbucket pair only: only bitbucket" test "$(ids "$bitbucket")" = "bitbucket"
check "both pairs: github and google" test "$(ids "$both")" = "github,google"
check "every pair: all four, in a stable order" test "$(ids "$all")" = "github,google,gitlab,bitbucket"
check "google is Kratos's built-in google provider with openid email profile" \
  test "$(q "$google" '.selfservice.methods.oidc.config.providers[0] | .provider + " " + (.scope | join(" ")) + " " + .client_id + " " + .client_secret')" \
  = "google openid email profile google-id google-secret"
check "gitlab is Kratos's built-in gitlab provider (gitlab.com) with userinfo's scopes" \
  test "$(provider "$gitlab" gitlab | yq -r '.provider + " " + (.issuer_url // "gitlab.com") + " " + (.scope | join(" ")) + " " + .client_id + " " + .client_secret')" \
  = "gitlab gitlab.com openid email profile gitlab-id gitlab-secret"
check "bitbucket is a generic provider at Dex's issuer, signing in as Dex's kratos client" \
  test "$(provider "$bitbucket" bitbucket | yq -r '.provider + " " + .issuer_url + " " + (.scope | join(" ")) + " " + .client_id + " " + .client_secret')" \
  = "generic https://auth.bex.co/dex openid email profile kratos dex-kratos-secret"
check "bitbucket's fragment never carries the Bitbucket pair (that is Dex's)" \
  bash -c '! grep -Eq "bb-id|bb-secret" <<<"$1"' _ "$bitbucket"
check "DEX_ISSUER_URL moves bitbucket's issuer (trailing slash dropped)" \
  test "$(provider "$(fragment "${BITBUCKET[@]}" DEX_ISSUER_URL=https://auth.example.com/dex/)" bitbucket | yq -r .issuer_url)" \
  = "https://auth.example.com/dex"
check "each provider names itself for the button" \
  test "$(q "$all" '[.selfservice.methods.oidc.config.providers[].label] | join(",")')" = "GitHub,Google,GitLab,Bitbucket"
check "github keeps its provider and scope" \
  test "$(q "$github" '.selfservice.methods.oidc.config.providers[0] | .provider + " " + (.scope | join(" "))')" = "github user:email"

for name in github google gitlab bitbucket both all; do
  check "$name: login hook mirrors the password flow" \
    test "$(q "${!name}" '[.selfservice.flows.login.after.oidc.hooks[].hook] | join(",")')" = "require_verified_address"
  check "$name: registration hooks mirror the password flow" \
    test "$(q "${!name}" '[.selfservice.flows.registration.after.oidc.hooks[].hook] | join(",")')" = "show_verification_ui,session"
done

mapper() { # fragment, provider id → the decoded Jsonnet mapper
  provider "$1" "$2" | yq -r .mapper_url | sed 's|^base64://||' | base64 --decode
}
check "github's mapper is unchanged: email and name" \
  test "$(mapper "$both" github)" = "local claims = std.extVar('claims'); { identity: { traits: { email: claims.email } + (if std.objectHas(claims, 'name') && claims.name != null then { name: claims.name } else {}) } }"
verified_mapper="local claims = std.extVar('claims'); { identity: { traits: { email: claims.email } + (if std.objectHas(claims, 'name') && claims.name != null then { name: claims.name } else {}) } + (if std.objectHas(claims, 'email_verified') && claims.email_verified == true then { verified_addresses: [{ via: 'email', value: claims.email }] } else {}) }"
for id in google gitlab bitbucket; do
  check "$id's mapper marks a provider-verified email verified" test "$(mapper "$all" "$id")" = "$verified_mapper"
done

# The test-only issuer overrides register google and gitlab as generic providers.
stand_in="$(fragment "${GOOGLE[@]}" "${GITLAB[@]}" KRATOS_OIDC_GOOGLE_ISSUER=http://localhost:5556/dex KRATOS_OIDC_GITLAB_ISSUER=http://localhost:5556/dex)"
for id in google gitlab; do
  check "the e2e stand-in: $id is a generic provider at the issuer" \
    test "$(provider "$stand_in" "$id" | yq -r '.id + " " + .provider + " " + .issuer_url')" = "$id generic http://localhost:5556/dex"
done
check "the e2e stand-in keeps gitlab's label, scopes and mapper" \
  test "$(provider "$stand_in" gitlab | yq -r '.label + " " + (.scope | join(" ")) + " " + .mapper_url')" \
  = "$(provider "$gitlab" gitlab | yq -r '.label + " " + (.scope | join(" ")) + " " + .mapper_url')"

# Dex's config: the other half of the Bitbucket bridge.
dex="$(lib dex_bitbucket_config "${BITBUCKET[@]}")"
check "dex config parses as YAML" bash -c 'yq -e . >/dev/null <<<"$1"' _ "$dex"
check "dex: issuer is auth.bex.co/dex, memory storage, no approval screen" \
  test "$(q "$dex" '.issuer + " " + .storage.type + " " + (.oauth2.skipApprovalScreen | tostring) + " " + (.oauth2.responseTypes | join(","))')" \
  = "https://auth.bex.co/dex memory true code"
check "dex: one bitbucket-cloud connector on the Bitbucket pair, calling back to the issuer" \
  test "$(q "$dex" '[.connectors[] | .type + " " + .id + " " + .config.clientID + " " + .config.clientSecret + " " + .config.redirectURI] | join(";")')" \
  = "bitbucket-cloud bitbucket bb-id bb-secret https://auth.bex.co/dex/callback"
check "dex: one static client, Kratos's, redirecting to its bitbucket callback" \
  test "$(q "$dex" '[.staticClients[] | .id + " " + .secret + " " + (.redirectURIs | join(","))] | join(";")')" \
  = "kratos dex-kratos-secret https://auth.bex.co/self-service/methods/oidc/callback/bitbucket"
check "dex: no password database, no other connector" \
  test "$(q "$dex" '(.enablePasswordDB // false | tostring) + " " + (.connectors | length | tostring)')" = "false 1"
check "dex: the client the fragment signs in as is the one Dex serves" \
  test "$(q "$dex" '.staticClients[0].id + " " + .staticClients[0].secret + " " + .issuer')" \
  = "$(provider "$bitbucket" bitbucket | yq -r '.client_id + " " + .client_secret + " " + .issuer_url')"
dex_moved="$(lib dex_bitbucket_config "${BITBUCKET[@]}" DEX_ISSUER_URL=http://dex.auth.svc.cluster.local:5556/dex KRATOS_PUBLIC_URL=http://localhost:4433/)"
check "dex: DEX_ISSUER_URL and KRATOS_PUBLIC_URL move the issuer and both callbacks" \
  test "$(q "$dex_moved" '.issuer + " " + .connectors[0].config.redirectURI + " " + .staticClients[0].redirectURIs[0]')" \
  = "http://dex.auth.svc.cluster.local:5556/dex http://dex.auth.svc.cluster.local:5556/dex/callback http://localhost:4433/self-service/methods/oidc/callback/bitbucket"

# The settings guard.
guarded="$(fragment "${GOOGLE[@]}" "${GUARD[@]}")"
guard_only="$(fragment "${GUARD[@]}")"
check "no guard without its URL and token" test "$(q "$google" '.selfservice.flows.settings')" = "null"
check "a guard without providers leaves oidc disabled" \
  test "$(q "$guard_only" '.selfservice.methods.oidc.enabled')" = "false"
for name in guarded guard_only; do
  for method in profile oidc; do
    hook=".selfservice.flows.settings.after.$method.hooks[0]"
    check "$name: $method runs the settings guard" \
      test "$(q "${!name}" "$hook.hook + \" \" + $hook.config.method + \" \" + $hook.config.url")" \
      = "web_hook POST http://bex-api.bex-system.svc.cluster.local:8091/internal/kratos/settings-guard?method=$method"
    check "$name: $method's hook interrupts the flow and sends the bearer" \
      test "$(q "${!name}" "($hook.config.response.parse | tostring) + \" \" + $hook.config.auth.type + \" \" + $hook.config.auth.config.name + \" \" + $hook.config.auth.config.value + \" \" + $hook.config.auth.config.in")" \
      = "true api_key Authorization Bearer guard-token header"
    check "$name: $method sends the identity, its email, the request URL and the unlink the client names" \
      grep -q "identity_id: ctx.identity.id, email: .*request_url: ctx.request_url, unlink: .*ctx.flow.transient_payload.unlink else ''" \
      <<<"$(q "${!name}" "$hook.config.body" | sed 's|^base64://||' | base64 --decode)"
  done
  check "$name: the password method is not guarded" test "$(q "${!name}" '.selfservice.flows.settings.after.password')" = "null"
done

# auth-secrets.sh writes the fragment from this library, and its dry run names
# the providers and the guard without printing a secret. (It also reads a
# developer's .env when one exists, which may enable github too.)
# Placeholders of the lengths auth-secrets.sh requires, never credential-shaped.
long="$(printf 'x%.0s' {1..16})"
cipher="$(printf 'x%.0s' {1..32})"
dry_run() { # env assignments... → auth-secrets.sh's dry run
  env -i PATH="$PATH" HOME="$HOME" DRY_RUN=1 "$@" \
    KRATOS_SECRETS_DEFAULT="$long" KRATOS_SECRETS_COOKIE="$long" KRATOS_SECRETS_CIPHER="$cipher" \
    HYDRA_SECRETS_SYSTEM="$long" HYDRA_SECRETS_COOKIE="$long" HYDRA_OIDC_PAIRWISE_SALT="$long" \
    OPENFGA_PRESHARED_KEY="$long" KRATOS_COURIER_SMTP_URI=smtp://relay:587 bash scripts/auth-secrets.sh
}
dry="$(dry_run "${GOOGLE[@]}" "${GITLAB[@]}" BEX_BITBUCKET_OIDC_CLIENT_ID=bb-id BEX_BITBUCKET_OIDC_CLIENT_SECRET=bb-secret)"
check "auth-secrets.sh dry run names the enabled providers" grep -Eq "providers ENABLED:( github)? google gitlab bitbucket$" <<<"$dry"
check "auth-secrets.sh dry run names the settings guard" grep -q "settings guard -> http://bex-api.bex-system.svc.cluster.local:8091/internal/kratos/settings-guard" <<<"$dry"
check "auth-secrets.sh dry run brings Bitbucket's Dex up" \
  bash -c 'grep -q "secret auth/dex (keys: config.yaml kratosClientSecret): Bitbucket.s bridge, issuer https://auth.bex.co/dex" <<<"$1" && grep -q "scale auth/deploy/dex to 1 replica" <<<"$1"' _ "$dry"
check "auth-secrets.sh dry run prints no secret" bash -c '! grep -Eq "google-secret|gitlab-secret|bb-secret" <<<"$1"' _ "$dry"
dry_off="$(dry_run "${GOOGLE[@]}")"
check "auth-secrets.sh dry run without the Bitbucket pair takes Dex down" \
  grep -q "scale auth/deploy/dex to 0 replicas and delete secret auth/dex" <<<"$dry_off"
check "auth-secrets.sh calls the library" grep -q 'oidc.yaml "$(kratos_oidc_fragment)"' scripts/auth-secrets.sh
check "auth-secrets.sh writes Dex's config from the library" grep -q 'dex_config="$(dex_bitbucket_config)"' scripts/auth-secrets.sh

exit "$fail"
