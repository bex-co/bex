#!/usr/bin/env bash
# Pins scripts/lib/kratos-oidc-fragment.sh, the Kratos oidc.yaml config
# fragment scripts/auth-secrets.sh writes (docs/ADR012-auth.md §10, §9b;
# w7/m465): each social provider is enabled on its own pair of credentials,
# neither leaves oidc disabled, the login/registration hooks mirror the
# password flow whenever a provider is on, and the settings guard hooks the
# profile and oidc settings methods with a parsed, authenticated web_hook.
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

fragment() { # env assignments... → the fragment on stdout
  env -i PATH="$PATH" "$@" bash -c '. scripts/lib/kratos-oidc-fragment.sh; kratos_oidc_fragment'
}

q() { yq -r "$2" <<<"$1"; }

GH=(BEX_GITHUB_OIDC_CLIENT_ID=gh-id BEX_GITHUB_OIDC_CLIENT_SECRET=gh-secret)
GOOGLE=(BEX_GOOGLE_OIDC_CLIENT_ID=google-id BEX_GOOGLE_OIDC_CLIENT_SECRET=google-secret)
GUARD=(KRATOS_SETTINGS_GUARD_URL=http://bex-api.bex-system.svc.cluster.local:8091/ KRATOS_SETTINGS_GUARD_TOKEN=guard-token)

none="$(fragment)"
github="$(fragment "${GH[@]}")"
google="$(fragment "${GOOGLE[@]}")"
both="$(fragment "${GH[@]}" "${GOOGLE[@]}")"
half="$(fragment BEX_GOOGLE_OIDC_CLIENT_ID=google-id)" # a client id without its secret enables nothing

for name in none github google both half; do
  check "$name: parses as YAML" bash -c 'yq -e . >/dev/null <<<"$1"' _ "${!name}"
done

check "neither pair: oidc disabled, no flows" \
  test "$(q "$none" '.selfservice.methods.oidc.enabled')/$(q "$none" '.selfservice.flows')" = "false/null"
check "a client id alone: oidc disabled" test "$(q "$half" '.selfservice.methods.oidc.enabled')" = "false"
check "GitHub pair only: only github" \
  test "$(q "$github" '[.selfservice.methods.oidc.config.providers[].id] | join(",")')" = "github"
check "Google pair only: only google" \
  test "$(q "$google" '[.selfservice.methods.oidc.config.providers[].id] | join(",")')" = "google"
check "both pairs: github and google" \
  test "$(q "$both" '[.selfservice.methods.oidc.config.providers[].id] | join(",")')" = "github,google"
check "google is Kratos's built-in google provider with openid email profile" \
  test "$(q "$google" '.selfservice.methods.oidc.config.providers[0] | .provider + " " + (.scope | join(" ")) + " " + .client_id + " " + .client_secret')" \
  = "google openid email profile google-id google-secret"
check "each provider names itself for the button" \
  test "$(q "$both" '[.selfservice.methods.oidc.config.providers[].label] | join(",")')" = "GitHub,Google"
check "github keeps its provider and scope" \
  test "$(q "$github" '.selfservice.methods.oidc.config.providers[0] | .provider + " " + (.scope | join(" "))')" = "github user:email"

for name in github google both; do
  check "$name: login hook mirrors the password flow" \
    test "$(q "${!name}" '[.selfservice.flows.login.after.oidc.hooks[].hook] | join(",")')" = "require_verified_address"
  check "$name: registration hooks mirror the password flow" \
    test "$(q "${!name}" '[.selfservice.flows.registration.after.oidc.hooks[].hook] | join(",")')" = "show_verification_ui,session"
done

mapper() { # fragment, provider id → the decoded Jsonnet mapper
  q "$1" ".selfservice.methods.oidc.config.providers[] | select(.id == \"$2\") | .mapper_url" | sed 's|^base64://||' | base64 --decode
}
check "github's mapper is unchanged: email and name" \
  test "$(mapper "$both" github)" = "local claims = std.extVar('claims'); { identity: { traits: { email: claims.email } + (if std.objectHas(claims, 'name') && claims.name != null then { name: claims.name } else {}) } }"
check "google's mapper marks a Google-verified email verified" \
  grep -q "verified_addresses: \[{ via: 'email', value: claims.email }\]" <<<"$(mapper "$both" google)"

# The test-only issuer override registers google as a generic provider.
stand_in="$(fragment "${GOOGLE[@]}" KRATOS_OIDC_GOOGLE_ISSUER=http://localhost:5556/dex)"
check "the e2e stand-in: google is a generic provider at the issuer" \
  test "$(q "$stand_in" '.selfservice.methods.oidc.config.providers[0] | .id + " " + .provider + " " + .issuer_url')" = "google generic http://localhost:5556/dex"

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
    check "$name: $method sends the identity, its email and the request URL" \
      grep -q "identity_id: ctx.identity.id, email: .*request_url: ctx.request_url" \
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
dry="$(env -i PATH="$PATH" HOME="$HOME" DRY_RUN=1 "${GOOGLE[@]}" \
  KRATOS_SECRETS_DEFAULT="$long" KRATOS_SECRETS_COOKIE="$long" KRATOS_SECRETS_CIPHER="$cipher" \
  HYDRA_SECRETS_SYSTEM="$long" HYDRA_SECRETS_COOKIE="$long" HYDRA_OIDC_PAIRWISE_SALT="$long" \
  OPENFGA_PRESHARED_KEY="$long" KRATOS_COURIER_SMTP_URI=smtp://relay:587 bash scripts/auth-secrets.sh)"
check "auth-secrets.sh dry run names the enabled providers" grep -Eq "providers ENABLED:( github)? google$" <<<"$dry"
check "auth-secrets.sh dry run names the settings guard" grep -q "settings guard -> http://bex-api.bex-system.svc.cluster.local:8091/internal/kratos/settings-guard" <<<"$dry"
check "auth-secrets.sh dry run prints no secret" bash -c '! grep -q "google-secret" <<<"$1"' _ "$dry"
check "auth-secrets.sh calls the library" grep -q 'oidc.yaml "$(kratos_oidc_fragment)"' scripts/auth-secrets.sh

exit "$fail"
