#!/usr/bin/env bash
set -euo pipefail

# Render SDK compatibility check (w32/m284): run the unmodified, pinned Render
# Python and TypeScript SDKs against a bex API base, as a client to verify (the
# same stance DO_NOT_DO.md takes for the Render CLI). Grades go to
# docs/cli-compatibility-checklist.md § Render SDK.
#
# Required:
#   BEX_API_URL           bex-api origin, e.g. https://api.bex.co (no /v1)
#   RENDER_API_KEY        a bex bearer token for the workspace
#   RENDER_WORKSPACE_ID   the workspace (tea-…) to create the fixture sandbox in
# Optional:
#   SDK_COMPAT_ONLY=python|typescript   run one SDK
#
# Creates one short-lived deny-all sandbox per SDK and terminates it.

cd "$(dirname "$0")/.."
: "${BEX_API_URL:?set BEX_API_URL to the bex-api origin}"
: "${RENDER_API_KEY:?set RENDER_API_KEY to a bex bearer token}"
: "${RENDER_WORKSPACE_ID:?set RENDER_WORKSPACE_ID to the fixture workspace}"
export BEX_API_URL="${BEX_API_URL%/}"

python_sdk_version=1.8.0
typescript_sdk_version=1.8.0
only="${SDK_COMPAT_ONLY:-}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
status=0

if [ -z "$only" ] || [ "$only" = python ]; then
  echo "==> Render Python SDK render-sdk==$python_sdk_version"
  python3 -m venv "$work/venv"
  "$work/venv/bin/pip" install --quiet --disable-pip-version-check "render-sdk==$python_sdk_version"
  "$work/venv/bin/python" -I scripts/render-sdk-compat/scenario.py || status=$((status + $?))
fi
if [ -z "$only" ] || [ "$only" = typescript ]; then
  echo "==> Render TypeScript SDK @renderinc/sdk@$typescript_sdk_version"
  mkdir -p "$work/ts"
  cp scripts/render-sdk-compat/scenario.mjs "$work/ts/"
  (cd "$work/ts" && npm init -y >/dev/null && npm install --silent --no-audit --no-fund "@renderinc/sdk@$typescript_sdk_version" >/dev/null)
  (cd "$work/ts" && node scenario.mjs) || status=$((status + $?))
fi
exit "$status"
