#!/usr/bin/env bash
# wrangler-r2-contract.sh — run Cloudflare's real wrangler against bex's own
# Cloudflare-shaped object storage handlers (ADR112 D11, w21/m416 t007).
#
# bex serves the routes stock `wrangler r2` calls under /cloudflare/client/v4,
# so a customer needs only CLOUDFLARE_API_BASE_URL, CLOUDFLARE_API_TOKEN and
# CLOUDFLARE_ACCOUNT_ID. wrangler is Cloudflare's tool and its wire can change
# in any release, so this is the early warning: it installs the pinned version
# into a throwaway directory, then lets
# lego/backend/internal/api/cloudflare_r2_contract_test.go start the handlers
# over a fake S3 on a loopback socket and drive every command ADR112 names
# (bucket create/list/info/delete with a location, a jurisdiction and a storage
# class; object put/get/delete --remote; cors list/set/delete; lifecycle
# list/add/set/remove; domain, dev-url, lock, sippy and notification refusals).
#
# The requests of a passing run are the fixtures the offline replay test checks
# on every build. After changing WRANGLER_VERSION (and the wranglerVersion
# constant in the test), re-record them:
#
#   BEX_WRANGLER_RECORD=1 scripts/wrangler-r2-contract.sh
#
# Needs node >= 22.12 (wrangler's own engines) and go. Nothing here touches a
# real bex deployment, Cloudflare or Wasabi.
set -euo pipefail

WRANGLER_VERSION="4.162.0"

root=$(cd "$(dirname "$0")/.." && pwd)
fixture="$root/lego/backend/internal/api/testdata/wrangler-$WRANGLER_VERSION/requests.json"

for tool in node npm go; do
  command -v "$tool" >/dev/null || {
    echo "wrangler-r2-contract: $tool is not installed" >&2
    exit 2
  }
done

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# --ignore-scripts: no install hook of wrangler's dependencies runs, and none is
# needed for the remote r2 commands.
npm install --prefix "$work" --no-audit --no-fund --ignore-scripts "wrangler@$WRANGLER_VERSION" >/dev/null
bin="$work/node_modules/.bin/wrangler"
[ -x "$bin" ] || {
  echo "wrangler-r2-contract: wrangler $WRANGLER_VERSION did not install" >&2
  exit 2
}

if [ "${BEX_WRANGLER_RECORD:-}" = 1 ]; then
  export BEX_WRANGLER_RECORD="$fixture"
else
  unset BEX_WRANGLER_RECORD
fi

cd "$root/lego/backend"
BEX_WRANGLER_BIN="$bin" BEX_WRANGLER_VERSION="$WRANGLER_VERSION" \
  go test ./internal/api -run 'TestWranglerEndToEnd|TestWranglerFixturesReplay' -count=1 -v
