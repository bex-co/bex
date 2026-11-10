#!/usr/bin/env bash
# Unit tests for render-schema-drift.sh's failure modes (w1/m165 t004/t007).
# Network-free: Render's two published documents are served from file:// URLs,
# so what is under test is the script's own logic — integrity checking, the
# comparison, and whether each failure names its fault, its file and the action.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/render-schema-drift.sh"
pinned="$repo_root/lego/backend/internal/apps/schema/render.yaml.json"
fixture="$repo_root/docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-11-02.json"
pinned_openapi="$repo_root/lego/backend/internal/api/openapi/render-public-api-1.json"
omissions="$repo_root/lego/backend/internal/api/render_operation_coverage_test.go"
cli_pin="$repo_root/lego/cli/UPSTREAM_RENDER_CLI.md"

failures=0
check() {
  local name="$1" expected_exit="$2" expected_text="$3" actual_exit=0 output
  shift 3
  output="$("$@" 2>&1)" || actual_exit=$?
  if [[ "$actual_exit" != "$expected_exit" ]]; then
    echo "FAIL: $name — exit $actual_exit, want $expected_exit" >&2
    echo "$output" | head -5 >&2
    failures=$((failures + 1))
    return
  fi
  if [[ -n "$expected_text" ]] && ! grep -qF -- "$expected_text" <<<"$output"; then
    echo "FAIL: $name — output missing '$expected_text'" >&2
    echo "$output" | head -5 >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}

sandbox="$(mktemp -d -t bex-schema-drift-test.XXXXXX)"
trap 'rm -rf "$sandbox"' EXIT

# The "upstream" OpenAPI: Render's pinned spec with the event type enum set to
# exactly the fixture's values, so only a deliberate mutation below differs.
jq --slurpfile fx "$fixture" \
  '.paths["/events/{eventId}"].get.responses["200"].content["application/json"].schema.properties.type.enum = $fx[0].renderOpenAPI' \
  "$pinned_openapi" >"$sandbox/openapi.json"

# The "upstream" CLI release: one older than any pin, unless a case says otherwise.
jq -n '{tag_name: "v0.0.1"}' >"$sandbox/cli-latest.json"
jq -n '{tag_name: "v99.0.0"}' >"$sandbox/cli-newer.json"

root="$sandbox/repo"
run_with() { # run_with <upstream-schema-file> <upstream-openapi-file> [upstream-cli-release-file]
  rm -rf "$root"
  mkdir -p "$root/scripts" "$root/lego/backend/internal/apps/schema" "$root/lego/backend/internal/api/openapi" "$root/docs/render-artifacts/fixtures" "$root/lego/cli"
  cp "$script" "$root/scripts/render-schema-drift.sh"
  cp "$pinned" "$root/lego/backend/internal/apps/schema/render.yaml.json"
  # The pinned OpenAPI is the same baseline the matching upstream serves, so
  # only a case's deliberate mutation differs.
  cp "$sandbox/openapi.json" "$root/lego/backend/internal/api/openapi/render-public-api-1.json"
  cp "$omissions" "$root/lego/backend/internal/api/"
  cp "$cli_pin" "$root/lego/cli/"
  cp "$fixture" "$root/docs/render-artifacts/fixtures/"
  RENDER_BLUEPRINT_SCHEMA_URL="file://$1" RENDER_OPENAPI_URL="file://$2" RENDER_CLI_LATEST_URL="file://${3:-$sandbox/cli-latest.json}" \
    "$root/scripts/render-schema-drift.sh"
}

check "matching upstream passes" 0 "Render Blueprint schema matches pinned" \
  run_with "$pinned" "$sandbox/openapi.json"

# Blueprint drift: a new top-level field and a new definition, the shape of the
# real buildSources drift the repaired job found on its first run (re-pinned by
# w1/m170, so the synthetic names below must never exist upstream).
jq '.allOf[1].properties.bexDriftProbe = {"type": "array"} | .definitions.bexDriftProbeDefinition = {"type": "object"}' \
  "$pinned" >"$sandbox/schema-added.json"
check "Blueprint drift fails" 1 "::error title=Render Blueprint schema drift::" \
  run_with "$sandbox/schema-added.json" "$sandbox/openapi.json"
check "Blueprint drift names the added property" 1 "property allOf.1.properties.bexDriftProbe" \
  run_with "$sandbox/schema-added.json" "$sandbox/openapi.json"
check "Blueprint drift names the added definition" 1 "definition bexDriftProbeDefinition" \
  run_with "$sandbox/schema-added.json" "$sandbox/openapi.json"
check "Blueprint drift names the ADR049 decision" 1 "reject it with a named error (unsupported)" \
  run_with "$sandbox/schema-added.json" "$sandbox/openapi.json"
check "Blueprint drift names every digest to move" 1 "RenderBlueprintSchemaSHA256" \
  run_with "$sandbox/schema-added.json" "$sandbox/openapi.json"

first_definition="$(jq -r '.definitions | keys[0]' "$pinned")"
jq --arg d "$first_definition" 'del(.definitions[$d])' "$pinned" >"$sandbox/schema-removed.json"
check "Blueprint drift names a removed definition" 1 "definition $first_definition" \
  run_with "$sandbox/schema-removed.json" "$sandbox/openapi.json"

# Webhook vocabulary drift: upstream adds an event type the fixture lacks.
jq '.paths["/events/{eventId}"].get.responses["200"].content["application/json"].schema.properties.type.enum += ["brand_new_event"]' \
  "$sandbox/openapi.json" >"$sandbox/openapi-drift.json"
check "webhook enum drift fails and names the fixture" 1 "render-webhook-vocabulary-2026-11-02.json" \
  run_with "$pinned" "$sandbox/openapi-drift.json"
check "webhook enum drift shows the new value" 1 "+brand_new_event" \
  run_with "$pinned" "$sandbox/openapi-drift.json"
check "webhook enum drift names the action" 1 "capture a new dated webhook vocabulary fixture" \
  run_with "$pinned" "$sandbox/openapi-drift.json"

jq 'del(.paths["/events/{eventId}"])' "$sandbox/openapi.json" >"$sandbox/openapi-moved.json"
check "moved webhook enum says the fixture is not at fault" 1 "is not at fault" \
  run_with "$pinned" "$sandbox/openapi-moved.json"

# REST operation drift: upstream adds an operation and removes another.
jq '.paths["/bex-drift-probe"] = {"get": {"operationId": "bex-drift-probe"}} | del(.paths["/services/{serviceId}/suspend"])' \
  "$sandbox/openapi.json" >"$sandbox/openapi-ops.json"
check "matching operations pass" 0 "Render REST operations match the pinned" \
  run_with "$pinned" "$sandbox/openapi.json"
check "operation drift fails" 1 "::error title=Render REST operation drift::" \
  run_with "$pinned" "$sandbox/openapi-ops.json"
check "operation drift names the added operation" 1 "GET /bex-drift-probe" \
  run_with "$pinned" "$sandbox/openapi-ops.json"
check "operation drift names the removed operation" 1 "POST /services/{serviceId}/suspend" \
  run_with "$pinned" "$sandbox/openapi-ops.json"
check "operation drift names the action" 1 "renderOperationOmissions" \
  run_with "$pinned" "$sandbox/openapi-ops.json"

# In-operation drift (w3/m303): a served operation's response schema changes
# under an unchanged method+path; an omitted operation's change is not bex's.
jq '.paths["/services/{serviceId}/suspend"].post.responses["202"].description = "bex drift probe"' \
  "$sandbox/openapi.json" >"$sandbox/openapi-op-schema.json"
# cancelFlowRun is a Render Workflows operation, a recorded omission.
grep -q '"cancelFlowRun":' "$omissions" || { echo "FAIL: cancelFlowRun is no longer in renderOperationOmissions; pick another omitted operation" >&2; exit 1; }
jq '(.paths[][] | objects | select(.operationId? == "cancelFlowRun") | .description) = "bex drift probe"' \
  "$sandbox/openapi.json" >"$sandbox/openapi-omitted-schema.json"
cmp -s "$sandbox/openapi.json" "$sandbox/openapi-omitted-schema.json" && { echo "FAIL: the omitted-operation mutation changed nothing" >&2; exit 1; }
# A shared component change shows on every served operation that reaches it.
jq '.components.schemas.serviceRuntime.description = "bex drift probe"' \
  "$sandbox/openapi.json" >"$sandbox/openapi-component.json"
cmp -s "$sandbox/openapi.json" "$sandbox/openapi-component.json" && { echo "FAIL: the component mutation changed nothing" >&2; exit 1; }
check "matching operation schemas pass" 0 "Render REST operation schemas match the pinned contract" \
  run_with "$pinned" "$sandbox/openapi.json"
check "a served operation's schema drift fails" 1 "::error title=Render REST operation schema drift::" \
  run_with "$pinned" "$sandbox/openapi-op-schema.json"
check "schema drift names the changed operation" 1 "POST /services/{serviceId}/suspend" \
  run_with "$pinned" "$sandbox/openapi-op-schema.json"
check "an omitted operation's schema drift is not bex's" 0 "Render REST operation schemas match the pinned contract" \
  run_with "$pinned" "$sandbox/openapi-omitted-schema.json"
check "a shared component's drift fails on the operations reaching it" 1 "GET /services" \
  run_with "$pinned" "$sandbox/openapi-component.json"

# Render CLI release drift (w3/m303).
check "an older upstream CLI release passes" 0 "is the latest upstream release" \
  run_with "$pinned" "$sandbox/openapi.json"
check "a newer CLI release fails" 1 "::error title=Render CLI release drift::render-oss/cli v99.0.0 is newer than bex's pin" \
  run_with "$pinned" "$sandbox/openapi.json" "$sandbox/cli-newer.json"

# Both halves report on one run: Blueprint drift must not hide webhook drift.
check "Blueprint and webhook drift are both reported" 1 "Render webhook OpenAPI drift" \
  run_with "$sandbox/schema-added.json" "$sandbox/openapi-drift.json"

# Check, never update: a drifting run leaves the pin and fixture byte-identical.
pins_untouched() {
  run_with "$sandbox/schema-added.json" "$sandbox/openapi-drift.json" >/dev/null 2>&1 || true
  cmp -s "$pinned" "$root/lego/backend/internal/apps/schema/render.yaml.json" &&
    cmp -s "$fixture" "$root/docs/render-artifacts/fixtures/$(basename "$fixture")"
}
check "a drifting run leaves pin and fixture untouched" 0 "" pins_untouched

# A hand-edited pin fails on integrity, before any download, naming the action.
edited_pin() {
  rm -rf "$root"
  mkdir -p "$root/scripts" "$root/lego/backend/internal/apps/schema" "$root/lego/backend/internal/api/openapi" "$root/docs/render-artifacts/fixtures"
  cp "$script" "$root/scripts/render-schema-drift.sh"
  cp "$pinned_openapi" "$root/lego/backend/internal/api/openapi/render-public-api-1.json"
  jq '.title = "tampered"' "$pinned" >"$root/lego/backend/internal/apps/schema/render.yaml.json"
  cp "$fixture" "$root/docs/render-artifacts/fixtures/"
  RENDER_BLUEPRINT_SCHEMA_URL="file:///nonexistent" RENDER_OPENAPI_URL="file:///nonexistent" \
    "$root/scripts/render-schema-drift.sh"
}
check "hand-edited pin fails integrity" 1 "Render Blueprint pin integrity" edited_pin
check "hand-edited pin names the Go constant" 1 "RenderBlueprintSchemaSHA256" edited_pin

if ((failures > 0)); then
  echo "$failures test(s) failed" >&2
  exit 1
fi
echo "all render-schema-drift.sh tests passed"
