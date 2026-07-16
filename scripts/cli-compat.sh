#!/usr/bin/env bash
# Run render-oss/cli (github.com/render-oss/cli, checked out at ./cli) against
# a live bex-api instance instead of api.render.com. bex never forks/patches
# the CLI (.pm/DO_NOT_DO.md) — this wraps its two documented overrides:
#
#   RENDER_HOST     — base URL (cli/pkg/cfg/cfg.go GetHost); bex-api's REST
#                      surface, e.g. http://localhost:54090/v1/
#   RENDER_API_KEY  — sent verbatim as `Authorization: Bearer` (cfg.go
#                      GetAPIKey + client.go AddHeaders). Render's own API
#                      keys are static (rnd_...) and used directly; bex's API
#                      keys are Hydra client_credentials pairs (id+secret,
#                      docs/ADR012-auth.md), so this script exchanges them at
#                      Hydra's public /oauth2/token for a short-lived (15m,
#                      hydra.values.yaml ttl.access_token) bearer token first
#                      — the same exchange docs/ADR025-connect-an-agent.md
#                      documents for any headless MCP client. That's the one
#                      finding this harness itself produces: the CLI has no
#                      notion of a token-exchange step, so a real bex
#                      deployment would need a thin wrapper (this script, or
#                      a `render` shell alias) in front of it — see the
#                      checklist's harness header.
#
# Usage:
#   scripts/cli-compat.sh <render-cli-args...>
#   scripts/cli-compat.sh verify        # re-run the ✅ rows, exit non-zero on regression (t006)
#   scripts/cli-compat.sh registry-credential-verify
#
# Env (all have dev-9 defaults so this runs with zero setup once dev-9 is up):
#   BEX_API_URL       bex-api base, default http://localhost:54090
#   HYDRA_PUBLIC_URL  Hydra's public endpoint (token exchange), default http://localhost:59090
#   CLI_KEY_ENV       path to a KEY_ID/KEY_SECRET env file, default .pm/w9/dev-9/.cli-key.env
#     (bash .pm/w9/dev-9/bootstrap-key.sh mints one — see docs/cli-compatibility-checklist.md)
#   RENDER_BIN        path to the built render binary, default .pm/w9/dev-9/bin/render
set -euo pipefail
cd "$(dirname "$0")/.."

BEX_API_URL="${BEX_API_URL:-http://localhost:54090}"
HYDRA_PUBLIC_URL="${HYDRA_PUBLIC_URL:-http://localhost:59090}"
CLI_KEY_ENV="${CLI_KEY_ENV:-.pm/w9/dev-9/.cli-key.env}"
export RENDER_BIN="${RENDER_BIN:-.pm/w9/dev-9/bin/render}"

[ -f "$CLI_KEY_ENV" ] || { echo "error: $CLI_KEY_ENV missing — run: bash .pm/w9/dev-9/bootstrap-key.sh" >&2; exit 1; }
[ -x "$RENDER_BIN" ] || { echo "error: $RENDER_BIN missing — run: (cd cli && go build -o ../$RENDER_BIN .)" >&2; exit 1; }
# shellcheck disable=SC1090
source "$CLI_KEY_ENV"

TOKEN=$(curl -sf -X POST "$HYDRA_PUBLIC_URL/oauth2/token" \
  -d "grant_type=client_credentials&client_id=$CLI_COMPAT_KEY_ID&client_secret=$CLI_COMPAT_KEY_SECRET" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])')
[ -n "$TOKEN" ] || { echo "error: token exchange against $HYDRA_PUBLIC_URL failed" >&2; exit 1; }

export RENDER_HOST="$BEX_API_URL/v1/"
export RENDER_API_KEY="$TOKEN"
export HYDRA_PUBLIC_URL # verify.sh's logout leg re-exchanges a throwaway key here
export RENDER_WORKSPACE="${CLI_COMPAT_TENANT_ID:-}"
export CLI_COMPAT_EMAIL="${CLI_COMPAT_EMAIL:-}" # the key-minting user's email (verify.sh's whoami row)
export RENDER_CLI_CONFIG_PATH="${RENDER_CLI_CONFIG_PATH:-$(mktemp -d)/cli.yaml}"

if [ "${1:-}" = "verify" ]; then
  exec bash .pm/w9/done/m2/verify.sh   # cwd is already repo root (see the cd above)
fi

if [ "${1:-}" = "mutation-check" ]; then
  # w9/m4/t005: prove the verify legs fail loudly against a broken wire shape.
  exec env BEX_API_URL="$BEX_API_URL" HYDRA_PUBLIC_URL="$HYDRA_PUBLIC_URL" \
    bash .pm/w9/done/m4/mutation-check.sh
fi

if [ "${1:-}" = "registry-credential-verify" ]; then
  exec bash scripts/registry-credential-cli-verify.sh
fi

exec "$RENDER_BIN" "$@"
