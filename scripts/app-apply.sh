#!/usr/bin/env bash
# Apply a Render Blueprint through bex-api's one authoritative compiler.
#
# Usage: BEX_API_URL=https://api.example BEX_API_TOKEN=... \
#   scripts/app-apply.sh <render.yaml | project-dir>
#
# An omitted path in a project directory discovers render.yaml first, then the
# legacy bex.yml filename alias. This helper intentionally performs no YAML
# parsing and never writes App/Database CRs directly: API validation and deploy
# have the same compiler, authorization, action plan, and sync rules as every
# other public surface.
set -euo pipefail

input_path="${1:?usage: scripts/app-apply.sh <render.yaml | project-dir>}"
if [ -d "$input_path" ]; then
  canonical="$input_path/render.yaml"
  legacy="$input_path/bex.yml"
  if [ -f "$canonical" ] && [ -f "$legacy" ]; then
    echo "error: both render.yaml and legacy bex.yml exist; pass an explicit path" >&2
    exit 1
  fi
  if [ -f "$canonical" ]; then input_path="$canonical"
  elif [ -f "$legacy" ]; then
    input_path="$legacy"
    echo "warning: bex.yml is a deprecated filename-only alias; rename it to render.yaml" >&2
  else
    echo "error: no render.yaml (or legacy bex.yml) found in $1" >&2
    exit 1
  fi
fi
[ -f "$input_path" ] || { echo "error: $input_path not found" >&2; exit 1; }

: "${BEX_API_URL:?set BEX_API_URL to the bex-api base URL}"
: "${BEX_API_TOKEN:?set BEX_API_TOKEN to a Bearer API key or OAuth token}"

api_url="${BEX_API_URL%/}"
owner_id="${BEX_OWNER_ID:-}"
repo="${BEX_REPO:-}"
branch="${BEX_BRANCH:-}"
confirm="${BEX_CONFIRM:-}"
payload=$(jq -n --rawfile manifest "$input_path" \
  --arg ownerId "$owner_id" --arg repo "$repo" --arg branch "$branch" --arg confirm "$confirm" \
  '{ownerId: $ownerId, repo: $repo, branch: $branch, confirm: $confirm, bexYaml: $manifest}')

if [ "${DRY_RUN:-}" = "1" ]; then
  endpoint="/v1/blueprints/validate"
  payload=$(jq -n --rawfile manifest "$input_path" --arg ownerId "$owner_id" '{ownerId: $ownerId, bexYaml: $manifest}')
else
  endpoint="/v1/blueprints/deploy"
fi

curl --fail-with-body --silent --show-error \
  --request POST "$api_url$endpoint" \
  --header "Authorization: Bearer $BEX_API_TOKEN" \
  --header "Content-Type: application/json" \
  --data "$payload"
