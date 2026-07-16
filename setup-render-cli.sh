#!/usr/bin/env bash

# Source this file so its Render CLI exports remain in the current shell:
#   source ./setup-render-cli.sh

if ! (return 0 2>/dev/null); then
  printf '%s\n' 'Run: source ./setup-render-cli.sh' >&2
  exit 1
fi

_bex_setup_fail() {
  printf 'setup-render-cli: %s\n' "$1" >&2
  return 1
}

for _bex_setup_command in curl jq render; do
  command -v "$_bex_setup_command" >/dev/null 2>&1 ||
    _bex_setup_fail "missing required command: $_bex_setup_command" || return 1
done
unset _bex_setup_command

if [ -z "${BEX_API_KEY_ID:-}" ]; then
  printf 'bex API key ID: ' >&2
  IFS= read -r BEX_API_KEY_ID || return 1
fi

if [ -z "${BEX_API_KEY_SECRET:-}" ]; then
  printf 'bex API key secret: ' >&2
  _bex_setup_stty="$(stty -g 2>/dev/null || true)"
  [ -z "$_bex_setup_stty" ] || stty -echo
  IFS= read -r BEX_API_KEY_SECRET
  _bex_setup_read_status=$?
  [ -z "$_bex_setup_stty" ] || stty "$_bex_setup_stty"
  printf '\n' >&2
  unset _bex_setup_stty
  [ "$_bex_setup_read_status" -eq 0 ] || return "$_bex_setup_read_status"
  unset _bex_setup_read_status
fi

[ -n "$BEX_API_KEY_ID" ] || _bex_setup_fail 'API key ID cannot be empty' || return 1
[ -n "$BEX_API_KEY_SECRET" ] || _bex_setup_fail 'API key secret cannot be empty' || return 1

export BEX_API_KEY_ID
export RENDER_HOST="${RENDER_HOST:-https://api.bex.co/v1/}"
_bex_setup_token="$(
  curl --fail --silent --show-error \
    --request POST 'https://oauth.bex.co/oauth2/token' \
    --data-urlencode 'grant_type=client_credentials' \
    --data-urlencode "client_id=$BEX_API_KEY_ID" \
    --data-urlencode "client_secret=$BEX_API_KEY_SECRET" |
    jq --exit-status --raw-output '.access_token'
)" || return 1
export RENDER_API_KEY="$_bex_setup_token"
unset _bex_setup_token

unset BEX_API_KEY_SECRET

if [ -z "${RENDER_WORKSPACE:-}" ]; then
  render workspaces || return 1
  printf 'workspace ID (tea-…): ' >&2
  IFS= read -r RENDER_WORKSPACE || return 1
  [ -n "$RENDER_WORKSPACE" ] || _bex_setup_fail 'workspace ID cannot be empty' || return 1
  export RENDER_WORKSPACE
fi

render whoami || return 1
printf '%s\n' 'Render CLI configured for this shell. Re-source before the 15-minute access token expires.' >&2

unset -f _bex_setup_fail
