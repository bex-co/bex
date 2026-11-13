#!/usr/bin/env bash
# Check, but never update, Render's pinned Blueprint schema, webhook-event
# fixture, REST operations and their schemas, and the upstream CLI release.
# The repository bytes remain the runtime contract; temporary downloads are
# removed even when Render has changed upstream.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pinned="$repo_root/lego/backend/internal/apps/schema/render.yaml.json"
expected_sha256="6c6721a602ef6d368d5c9384c473cefaae99980e43dd4b1adcf8a2f258fc54f4"
schema_url="${RENDER_BLUEPRINT_SCHEMA_URL:-https://render.com/schema/render.yaml.json}"
openapi_url="${RENDER_OPENAPI_URL:-https://api-docs.render.com/openapi/render-public-api-1.json}"
webhook_fixture="$repo_root/docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-11-02.json"
pinned_openapi="$repo_root/lego/backend/internal/api/openapi/render-public-api-1.json"
omissions_file="$repo_root/lego/backend/internal/api/render_operation_coverage_test.go"
cli_pin_file="$repo_root/lego/cli/UPSTREAM_RENDER_CLI.md"
cli_latest_url="${RENDER_CLI_LATEST_URL:-https://api.github.com/repos/render-oss/cli/releases/latest}"

if [[ ! -f "$pinned" ]]; then
  echo "pinned Render Blueprint schema is missing: $pinned" >&2
  exit 1
fi
if [[ ! -f "$pinned_openapi" ]]; then
  echo "pinned Render OpenAPI is missing: $pinned_openapi" >&2
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
#   TestRenderDashboardAndAPIVocabulariesStayDistinct  — capturedAt, the dated
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
unknown fields (docs/ADR049-render-yaml-parity.md), so every added field and
enum value above needs a decision — implement it (equivalent/translated) or
reject it with a named error (unsupported); ADR049 has no "ignored" state —
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
  echo "Render webhook OpenAPI enum matches the pinned $(jq '.renderOpenAPI | length' "$webhook_fixture")-value fixture"
else
  echo "::error title=Render webhook OpenAPI drift::$openapi_url's event type enum no longer matches .renderOpenAPI in $webhook_fixture (diff above: - pinned, + upstream)" >&2
  echo "Action: capture a new dated webhook vocabulary fixture, re-disposition every added value in lego/backend/internal/webhooks/vocabulary_test.go, and re-audit the authenticated dashboard picker separately." >&2
  failed=1
fi

# REST operation surface (w2/m251): the Blueprint schema and webhook enum alone
# missed 44 new operations. A method+path list says WHICH operations moved;
# their schemas are re-reviewed at re-pin.
operation_surface() {
  jq -r '.paths | to_entries[] | .key as $path | .value | keys[]
    | select(IN("get", "post", "put", "patch", "delete")) | ascii_upcase + " " + $path' "$1" | sort -u
}
pinned_operations="$(operation_surface "$pinned_openapi")"
upstream_operations="$(operation_surface "$temporary_openapi")"
added_operations="$(comm -13 <(echo "$pinned_operations") <(echo "$upstream_operations"))"
removed_operations="$(comm -23 <(echo "$pinned_operations") <(echo "$upstream_operations"))"
if [[ -z "$added_operations" && -z "$removed_operations" ]]; then
  echo "Render REST operations match the pinned $(wc -l <<<"$pinned_operations" | tr -d ' ')-operation contract"
else
  echo "::error title=Render REST operation drift::$openapi_url's operations no longer match $pinned_openapi" >&2
  {
    echo "Added upstream:"
    echo "${added_operations:-(none)}" | sed 's/^/  /'
    echo "Removed upstream:"
    echo "${removed_operations:-(none)}" | sed 's/^/  /'
    cat <<MSG

Action: run a drift round (docs/ADR018-render-parity.md, the round paragraphs at
the top): re-pin the OpenAPI with renderOpenAPISHA256
(lego/backend/internal/api/render_openapi.go), then mount, refuse by name or
document every added operation in renderOperationOmissions with an owner.
MSG
  } >&2
  failed=1
fi

# In-operation drift (w3/m303): an unchanged method+path can still change its
# parameters, bodies or responses, and the operation list above cannot see it.
# Every operation bex serves (each pinned operation except
# renderOperationOmissions, whose drift bex does not answer) is compared with
# its upstream copy together with every component it reaches through $ref,
# transitively, so a changed shared schema shows on the operations that use it.
omitted_operations() {
  awk '/^var renderOperationOmissions = map\[string\]string\{/ { inside = 1; next }
    inside && /^\}/ { inside = 0 }
    inside && match($0, /^[[:space:]]*"[^"]+"/) { name = substr($0, RSTART, RLENGTH); gsub(/[[:space:]"]/, "", name); print name }' \
    "$omissions_file"
}
operation_bodies() { # operationId<TAB>METHOD path<TAB>operation JSON<TAB>reached components, for served operations
  jq -r --arg omitted "$omitted" '
    def segments: split("/") | map(gsub("~1"; "/") | gsub("~0"; "~") | if test("^[0-9]+$") then tonumber else . end);
    def refs: [.. | objects | .["$ref"]? | strings | select(startswith("#/"))] | unique;
    def target($doc; $ref): try ($doc | getpath($ref[2:] | segments)) catch null;
    def reached($doc): {seen: [], todo: refs}
      | until(.todo == []; (.seen + .todo | unique) as $seen
        | {seen: $seen, todo: (([.todo[] as $ref | target($doc; $ref) | refs[]] | unique) - $seen)})
      | .seen;
    ($omitted | split("\n")) as $omit
    | . as $doc | .paths | to_entries[] | .key as $path | .value | to_entries[]
    | select(.key | IN("get", "post", "put", "patch", "delete"))
    | select(.value.operationId as $id | $omit | index($id) | not)
    | "\(.value.operationId)\t\(.key | ascii_upcase) \($path)\t\(.value | tojson)\t\([.value | reached($doc)[] as $ref | {($ref): target($doc; $ref)}] | tojson)"' "$1" | sort
}
omitted="$(omitted_operations)"
if [[ -z "$omitted" ]]; then
  echo "::error title=Render operation omissions unreadable::found no renderOperationOmissions entries in $omissions_file" >&2
  echo "Action: the map moved or changed shape; update omitted_operations in this script." >&2
  failed=1
else
  pinned_bodies="$(operation_bodies "$pinned_openapi")"
  upstream_bodies="$(operation_bodies "$temporary_openapi")"
  changed_operations="$(comm -12 \
    <(comm -23 <(echo "$pinned_bodies") <(echo "$upstream_bodies") | cut -f2 | sort -u) \
    <(comm -13 <(echo "$pinned_bodies") <(echo "$upstream_bodies") | cut -f2 | sort -u))"
  if [[ -z "$changed_operations" ]]; then
    echo "Render REST operation schemas match the pinned contract for the $(wc -l <<<"$pinned_bodies" | tr -d ' ') operations bex serves"
  else
    echo "::error title=Render REST operation schema drift::operations bex serves changed inside their schemas upstream at $openapi_url" >&2
    {
      echo "Changed upstream:"
      echo "$changed_operations" | sed 's/^/  /'
      cat <<MSG

Action: run a drift round (docs/ADR018-render-parity.md): diff each operation
above against the pin, decide how bex answers every changed parameter, body or
response field, then re-pin the OpenAPI with renderOpenAPISHA256
(lego/backend/internal/api/render_openapi.go).
MSG
    } >&2
    failed=1
  fi
fi

# Render CLI release drift (w3/m303): bex's launcher imports the upstream CLI
# at a reviewed release, and a newer upstream release went unreported for days.
pinned_cli="$(sed -nE 's/^- \*\*Render release:\*\* `(v[0-9][0-9.]*)`.*/\1/p' "$cli_pin_file" | head -1)"
cli_auth=()
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  cli_auth=(-H "Authorization: Bearer $GITHUB_TOKEN")
fi
# ${a[@]+...}: an empty array under set -u is an unbound variable on bash 3.2.
latest_cli="$(curl --fail --location --retry 2 --silent --show-error ${cli_auth[@]+"${cli_auth[@]}"} "$cli_latest_url" | jq -r '.tag_name // empty')" || latest_cli=""
if [[ -z "$pinned_cli" || -z "$latest_cli" ]]; then
  echo "::error title=Render CLI release unreadable::pinned='$pinned_cli' (from $cli_pin_file) latest='$latest_cli' (from $cli_latest_url)" >&2
  failed=1
elif [[ "$(printf '%s\n%s\n' "$pinned_cli" "$latest_cli" | sort -V | tail -1)" == "$pinned_cli" ]]; then
  echo "Render CLI pin $pinned_cli is the latest upstream release"
else
  echo "::error title=Render CLI release drift::render-oss/cli $latest_cli is newer than bex's pin $pinned_cli ($cli_pin_file)" >&2
  echo "Action: bump the pin per \"Updating the pin\" in $cli_pin_file and walk docs/cli-compatibility-checklist.md with the new binary." >&2
  failed=1
fi

exit "$failed"
