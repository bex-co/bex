#!/usr/bin/env bash
# Check, but never update, Render's pinned Blueprint schema and webhook-event
# OpenAPI fixture. The repository bytes remain the runtime contract; temporary
# downloads are removed even when Render has changed upstream.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pinned="$repo_root/lego/backend/internal/apps/schema/render.yaml.json"
expected_sha256="665539cb0c191856ba38d292b985a963880bb69b030d666e5fe7788e78e7e696"
schema_url="${RENDER_BLUEPRINT_SCHEMA_URL:-https://render.com/schema/render.yaml.json}"
openapi_url="${RENDER_OPENAPI_URL:-https://api-docs.render.com/openapi/render-public-api-1.json}"
webhook_fixture="$repo_root/docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-08-17.json"

if [[ ! -f "$pinned" ]]; then
  echo "pinned Render Blueprint schema is missing: $pinned" >&2
  exit 1
fi
if [[ ! -f "$webhook_fixture" ]]; then
  echo "pinned Render webhook vocabulary is missing: $webhook_fixture" >&2
  exit 1
fi

# Fail locally before any network call if counts, normalized set differences,
# or disposition data in the hand-audited dashboard/API fixture disagree.
jq -e '
  .renderOpenAPI as $api |
  .renderDashboard as $dashboard |
  ($api | length) == 67 and
  ($dashboard | length) == 64 and
  ([ $api[] | select(. as $event | $dashboard | index($event) | not) ] | sort) == (.apiOnly | sort) and
  ([ $dashboard[] | select(. as $event | $api | index($event) | not) ] | sort) == (.dashboardOnly | sort) and
  (.apiOnly | length) == 6 and
  (.dashboardOnly | length) == 3 and
  (.bexSupported | length) == 35
' "$webhook_fixture" >/dev/null || {
  echo "Render webhook vocabulary fixture is internally inconsistent: $webhook_fixture" >&2
  exit 1
}

sha256() {
  shasum -a 256 "$1" | awk '{print $1}'
}

pinned_sha256="$(sha256 "$pinned")"
if [[ "$pinned_sha256" != "$expected_sha256" ]]; then
  echo "pinned Render Blueprint schema integrity mismatch: got $pinned_sha256, want $expected_sha256" >&2
  exit 1
fi

temporary_schema="$(mktemp -t bex-render-schema.XXXXXX)"
temporary_openapi="$(mktemp -t bex-render-openapi.XXXXXX)"
trap 'rm -f "$temporary_schema" "$temporary_openapi"' EXIT
curl --fail --location --retry 2 --silent --show-error "$schema_url" --output "$temporary_schema"
curl --fail --location --retry 2 --silent --show-error "$openapi_url" --output "$temporary_openapi"

upstream_sha256="$(sha256 "$temporary_schema")"
failed=0
if [[ "$upstream_sha256" == "$pinned_sha256" ]]; then
  echo "Render Blueprint schema matches pinned $pinned_sha256"
else
  echo "::error title=Render Blueprint schema drift::pinned=$pinned_sha256 upstream=$upstream_sha256" >&2
  diff -u "$pinned" "$temporary_schema" || true
  failed=1
fi

if ! jq -e '.paths["/events/{eventId}"].get.responses["200"].content["application/json"].schema.properties.type.enum | type == "array"' "$temporary_openapi" >/dev/null; then
  echo "::error title=Render webhook schema moved::could not locate GET /events/{eventId} response type enum" >&2
  failed=1
elif diff -u \
  <(jq -r '.renderOpenAPI[]' "$webhook_fixture") \
  <(jq -r '.paths["/events/{eventId}"].get.responses["200"].content["application/json"].schema.properties.type.enum[]' "$temporary_openapi"); then
  echo "Render webhook OpenAPI enum matches pinned 67-value fixture"
else
  echo "::error title=Render webhook OpenAPI drift::update the dated API fixture and re-audit the authenticated dashboard picker separately" >&2
  failed=1
fi

exit "$failed"
