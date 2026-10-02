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

# The fixture's INTERNAL consistency is not checked here, deliberately (w1/m165
# t001). It is owned by lego/backend/internal/webhooks/vocabulary_test.go, which
# asserts strictly more, by value rather than by count:
#
#   TestRenderDashboardAndAPIVocabulariesStayDistinct  — capturedAt, the 67/64
#     dated Render counts, and both set differences against .apiOnly/.dashboardOnly
#   TestBexWebhookVocabularyHasOneTruthfulDispositionPerValue — .bexSupported ==
#     bex's real EventTypes, every non-OpenAPI value dispositioned, and the
#     unsupported ledger exactly covering Render's union
#
# A jq block here used to duplicate those clauses as hardcoded literals, and that
# is precisely why this job had never once passed: bex's advertised vocabulary grew
# 35 -> 42 -> 53 as w3/m82 and its siblings shipped, the fixture and the Go test
# tracked it, and the literal `(.bexSupported | length) == 35` did not. Six of its
# seven clauses still agreed; that one aborted the script before any network call,
# so the REST drift comparison below — the only thing this script uniquely does —
# never ran for five weeks. Bumping the literal to 53 would only restart the same
# clock, so the duplicate is gone instead. The Go test runs on every
# `go test ./...`, which gates deploys, making it the stronger of the two.
#
# What remains below is this script's actual job, which no Go test can do: compare
# the pinned repository bytes against what Render publishes upstream, right now.

sha256() {
  shasum -a 256 "$1" | awk '{print $1}'
}

pinned_sha256="$(sha256 "$pinned")"
if [[ "$pinned_sha256" != "$expected_sha256" ]]; then
  echo "::error title=Render Blueprint pin integrity::$pinned has sha256 $pinned_sha256, want $expected_sha256" >&2
  cat >&2 <<MSG
The reviewed pin was changed without re-review. Restore it from git, or — if the
change IS a deliberate re-pin — set the new digest in BOTH expected_sha256 (this
script) and RenderBlueprintSchemaSHA256 (lego/backend/internal/apps/blueprint_schema.go).
MSG
  exit 1
fi

# schema_surface lists a Blueprint schema's contractual names — every
# definition and every declared property path — so a drift report can say WHICH
# fields moved instead of only that two digests differ.
schema_surface() {
  jq -r '(.definitions // {} | keys[] | "definition " + .),
    (paths | select(length >= 2 and .[-2] == "properties") | map(tostring) | "property " + join("."))' "$1" | sort -u
}

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
  added="$(comm -13 <(schema_surface "$pinned") <(schema_surface "$temporary_schema"))"
  removed="$(comm -23 <(schema_surface "$pinned") <(schema_surface "$temporary_schema"))"
  echo "::error title=Render Blueprint schema drift::$schema_url no longer matches the pin $pinned (pinned=$pinned_sha256 upstream=$upstream_sha256)" >&2
  {
    echo
    echo "Added upstream (${schema_url}):"
    echo "${added:-  (none — the change is inside existing definitions; see the diff below)}" | sed 's/^/  /'
    echo "Removed upstream:"
    echo "${removed:-  (none)}" | sed 's/^/  /'
    cat <<MSG

Action: re-review, do not just re-pin. bex's Blueprint compiler is fail-closed on
unknown fields (docs/ADR049-render-yaml-parity.md), so every added field above
needs a decision — accept-and-ignore, implement, or reject with a named error —
recorded in lego/backend/internal/apps/schema/capabilities.json. Then replace
the pin with the upstream bytes and set the new digest in expected_sha256 (this
script), RenderBlueprintSchemaSHA256 (lego/backend/internal/apps/blueprint_schema.go)
and capabilities.json's schema.sha256.
MSG
  } >&2
  diff -u "$pinned" "$temporary_schema" || true
  failed=1
fi

if ! jq -e '.paths["/events/{eventId}"].get.responses["200"].content["application/json"].schema.properties.type.enum | type == "array"' "$temporary_openapi" >/dev/null; then
  echo "::error title=Render webhook schema moved::could not locate GET /events/{eventId} response type enum in $openapi_url" >&2
  echo "Action: find where Render's OpenAPI now declares the event type enum and update this script's jq path; the fixture $webhook_fixture is not at fault." >&2
  failed=1
elif diff -u \
  <(jq -r '.renderOpenAPI[]' "$webhook_fixture") \
  <(jq -r '.paths["/events/{eventId}"].get.responses["200"].content["application/json"].schema.properties.type.enum[]' "$temporary_openapi"); then
  echo "Render webhook OpenAPI enum matches pinned 67-value fixture"
else
  echo "::error title=Render webhook OpenAPI drift::$openapi_url's event type enum no longer matches .renderOpenAPI in $webhook_fixture (diff above: - pinned, + upstream)" >&2
  echo "Action: capture a new dated webhook vocabulary fixture, re-disposition every added value in lego/backend/internal/webhooks/vocabulary_test.go, and re-audit the authenticated dashboard picker separately." >&2
  failed=1
fi

exit "$failed"
