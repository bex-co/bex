#!/usr/bin/env bash
# canary-api.sh — shared credential + HTTP plumbing for the production canary
# probes (w3/m83: scripts/tenant-view-liveness.sh, scripts/deploy-canary.sh).
#
# Sourced, never executed. It exists because both probes need the same three
# things and neither may ever leak the credential:
#
#   1. A bearer token from BEX_CANARY_API_KEY. A bex API key is NOT a static
#      Render-style `rnd_…` string: it is a Hydra client_credentials pair
#      (docs/ADR012-auth.md), so the key has to be exchanged at the issuer's
#      /oauth2/token for a short-lived access token before any REST call. The
#      secret is written to a 0600 file and passed with curl's --data @file so
#      it never appears in argv (visible to every process on the runner).
#   2. curl with the Authorization header supplied through a header FILE (-K),
#      for the same argv reason — the pattern scripts/static-delete-timing-verify.sh
#      established.
#   3. A workspace guard, so a mis-set secret pointing at a REAL tenant refuses
#      to run instead of creating/deleting resources in someone's workspace.
#
# Nothing here prints a credential. The only value ever echoed is a workspace
# or resource id.
#
# Env read here (documented once, referenced by both callers):
#   BEX_API_URL              bex-api origin, default https://api.bex.co
#   BEX_OAUTH_ISSUER         Hydra public issuer, default https://oauth.bex.co
#   BEX_CANARY_API_KEY       `<key-id>:<key-secret>` for the canary workspace's
#                            API key. A value with no colon is treated as an
#                            already-exchanged bearer token (local dry runs).
#   BEX_CANARY_WORKSPACE_ID  the canary workspace (`tea-…`); required by the
#                            write guard, optional for read-only probes.

# canary_api_url / canary_issuer — normalized origins (no trailing slash).
canary_api_url() { printf '%s' "${BEX_API_URL:-https://api.bex.co}" | sed 's#/*$##'; }
canary_issuer() { printf '%s' "${BEX_OAUTH_ISSUER:-https://oauth.bex.co}" | sed 's#/*$##'; }

# canary_require_tools — every canary probe needs exactly these.
canary_require_tools() {
  local tool
  for tool in curl jq; do
    command -v "$tool" >/dev/null || {
      echo "error: $tool is required" >&2
      return 2
    }
  done
}

# canary_login <workdir> — exchange BEX_CANARY_API_KEY and stage the curl
# header file. Sets CANARY_HDR_FILE for acurl. <workdir> must already exist and
# be private (mktemp -d); the caller's trap removes it.
canary_login() {
  local dir="$1" key="${BEX_CANARY_API_KEY:-}" key_id key_secret form token
  [ -d "$dir" ] || {
    echo "error: canary_login needs an existing private working directory" >&2
    return 2
  }
  [ -n "$key" ] || {
    echo "error: BEX_CANARY_API_KEY is unset (expected <key-id>:<key-secret> for the canary workspace)" >&2
    return 2
  }
  umask 077
  CANARY_HDR_FILE="$dir/canary.hdr"
  if [ "${key#*:}" = "$key" ]; then
    # No colon: an already-exchanged bearer token. Supported for local runs so
    # an operator can probe with a token they already hold.
    token="$key"
  else
    key_id="${key%%:*}"
    key_secret="${key#*:}"
    form="$dir/canary.form"
    printf 'grant_type=client_credentials&client_id=%s&client_secret=%s' \
      "$key_id" "$key_secret" >"$form"
    token="$(curl -sS --connect-timeout 5 --max-time 30 \
      -X POST "$(canary_issuer)/oauth2/token" \
      -H 'Content-Type: application/x-www-form-urlencoded' \
      --data "@$form" | jq -r '.access_token // empty')" || token=""
    rm -f -- "$form"
    [ -n "$token" ] || {
      echo "error: client_credentials exchange at $(canary_issuer)/oauth2/token returned no access token" >&2
      echo "       (the key id/secret pair, the issuer, or the key's grant type is wrong)" >&2
      return 2
    }
  fi
  printf 'header = "Authorization: Bearer %s"\n' "$token" >"$CANARY_HDR_FILE"
  unset token
}

# acurl <curl-args…> — authenticated curl. The token rides in -K, not argv.
acurl() {
  curl -sS -K "$CANARY_HDR_FILE" --connect-timeout 5 --max-time 60 "$@"
}

# canary_bearer — the exchanged token, for the one case that needs the value
# rather than the header file: handing it to a child script that takes a BEARER
# env var (scripts/static-delete-timing-verify.sh). Environment is per-process
# and owner-readable only, unlike argv; never echo this into a log.
canary_bearer() {
  sed -n 's/^header = "Authorization: Bearer \(.*\)"$/\1/p' "$CANARY_HDR_FILE"
}

# canary_assert_workspace — refuse to continue unless BEX_CANARY_WORKSPACE_ID is
# set AND the key can actually see it. This is the guard that keeps a mis-set
# secret from writing into a real tenant: deploy-canary.sh calls it before its
# first create.
canary_assert_workspace() {
  local want="${BEX_CANARY_WORKSPACE_ID:-}" owners
  case "$want" in
    tea-*) ;;
    *)
      echo "error: BEX_CANARY_WORKSPACE_ID must be the canary workspace id (tea-…); refusing to write" >&2
      return 2
      ;;
  esac
  owners="$(acurl "$(canary_api_url)/v1/owners?limit=100")" || {
    echo "error: could not list the key's workspaces (GET /v1/owners failed)" >&2
    return 2
  }
  echo "$owners" | jq -e --arg id "$want" '[.[] | (.owner // .).id] | index($id)' >/dev/null || {
    echo "error: this API key cannot see $want — it belongs to a different workspace." >&2
    echo "       Refusing to run: a canary that writes must only ever touch the canary workspace." >&2
    return 2
  }
  echo "  guard: key is scoped to the canary workspace $want"
}
