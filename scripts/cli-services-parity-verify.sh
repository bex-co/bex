#!/usr/bin/env bash
# Verify service create/update parity through the unmodified official Render CLI.
# Invoke through scripts/cli-compat.sh so the CLI endpoint and bearer are set.
#
#   scripts/cli-compat.sh services-parity-verify              # dependency-free
#   scripts/cli-compat.sh services-parity-verify configured   # OpenBao + private registry
#
# The configured leg additionally requires the REGISTRY_VERIFY_* and
# BEX_VERIFY_APPS_NAMESPACE inputs documented by registry-credential-cli-verify.sh.
set -euo pipefail

MODE="${1:-baseline}"
case "$MODE" in
  baseline | configured | self-test) ;;
  *) echo "usage: $0 [baseline|configured|self-test]" >&2; exit 2 ;;
esac

BASELINE_LEGS=(
  image-create image-update image-healthcheck image-delete repo-delete-during-build
  web-builder-roundtrip
  web-create web-update cron-create cron-update static-create static-update
  clone region-guard runtime-guard preview-update preview-create
)
CONFIGURED_LEGS=(configured-env-secret registry-credential)
COMPLETED_LEGS=()

complete_leg() {
  COMPLETED_LEGS+=("$1")
}

assert_leg_census() {
  local required completed found
  local required_legs=("${BASELINE_LEGS[@]}")
  if [ "$MODE" = configured ]; then
    required_legs+=("${CONFIGURED_LEGS[@]}")
  fi
  for required in "${required_legs[@]}"; do
    found=false
    for completed in "${COMPLETED_LEGS[@]}"; do
      if [ "$completed" = "$required" ]; then
        found=true
        break
      fi
    done
    if [ "$found" != true ]; then
      echo "FAIL: verifier leg did not run: $required" >&2
      return 1
    fi
  done
}

if [ "$MODE" = self-test ]; then
  MODE=configured
  all_legs=("${BASELINE_LEGS[@]}" "${CONFIGURED_LEGS[@]}")
  for omitted in "${all_legs[@]}"; do
    COMPLETED_LEGS=()
    for leg in "${all_legs[@]}"; do
      [ "$leg" = "$omitted" ] || complete_leg "$leg"
    done
    if assert_leg_census >/dev/null 2>&1; then
      echo "FAIL: leg census accepted a verifier missing $omitted" >&2
      exit 1
    fi
  done
  COMPLETED_LEGS=("${all_legs[@]}")
  assert_leg_census
  echo "PASS: verifier leg census rejects every single omitted create/update leg"
  exec bash scripts/cli-services-parity-self-test.sh
fi

: "${RENDER_HOST:?invoke through scripts/cli-compat.sh}"
: "${RENDER_API_KEY:?invoke through scripts/cli-compat.sh}"
: "${RENDER_WORKSPACE:?select a workspace before running the verifier}"
: "${RENDER_BIN:?set the official Render CLI binary}"

case "$RENDER_HOST" in
  http://localhost:* | https://localhost:* | http://127.0.0.1:* | https://127.0.0.1:*) ;;
  *)
    if [ "${BEX_CLI_SERVICES_ALLOW_REMOTE:-}" != "I-understand-this-creates-and-deletes-services" ]; then
      echo "error: refusing remote target $RENDER_HOST; set BEX_CLI_SERVICES_ALLOW_REMOTE=I-understand-this-creates-and-deletes-services" >&2
      exit 2
    fi
    ;;
esac

for dependency in curl jq mktemp; do
  command -v "$dependency" >/dev/null || { echo "error: $dependency is required" >&2; exit 1; }
done

API="${RENDER_HOST%/}"
RUN_ID="$(date -u +%H%M%S)-$(( $$ % 100000 ))"
PREFIX="cp-${RUN_ID}"
IMAGE_NAME="${PREFIX}-image"
DELETE_BUILD_NAME="${PREFIX}-del"
WEB_NAME="${PREFIX}-web"
BUILDER_NAME="${PREFIX}-builder"
CRON_NAME="${PREFIX}-cron"
STATIC_NAME="${PREFIX}-static"
CLONE_NAME="${PREFIX}-clone"
BARE_CLONE_NAME="${PREFIX}-bare"
PREVIEW_NAME="${PREFIX}-preview"
CREATED_IDS=()
CREATED_NAMES=()
TMP_DIR="$(mktemp -d)"

# shellcheck source=cli-service-delete-lib.sh
source scripts/cli-service-delete-lib.sh

assert_service_fixture_names \
  "$IMAGE_NAME" "$DELETE_BUILD_NAME" "$WEB_NAME" "${WEB_NAME}-updated" \
  "$BUILDER_NAME" "$CRON_NAME" "$STATIC_NAME" "$CLONE_NAME" "$BARE_CLONE_NAME" "$PREVIEW_NAME"

cleanup() {
  local rc=$? cleanup_failed=0 path
  trap - EXIT INT TERM ERR
  set +e
  cleanup_created_services || cleanup_failed=1
  for path in "$TMP_DIR"/*; do
    [ ! -f "$path" ] || unlink "$path"
  done
  rmdir "$TMP_DIR" 2>/dev/null || cleanup_failed=1
  if [ "$rc" -eq 0 ] && [ "$cleanup_failed" -ne 0 ]; then
    rc=1
  fi
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'rc=$?; echo "FAIL: service parity verifier stopped at line $LINENO (exit $rc)" >&2' ERR

api() {
  curl -sf -H "Authorization: Bearer $RENDER_API_KEY" "$API$1"
}

# Raw JSON POST — used to create the out-of-band service shapes (Dockerfile-via-
# builder, no runtime) that a dashboard/Blueprint/hand-applied App produces but
# the CLI's own create flags cannot express, so the CLI's `services update`
# round-trip can be graded against them (w4/052).
api_post() {
  curl -sf -H "Authorization: Bearer $RENDER_API_KEY" -H 'Content-Type: application/json' \
    -X POST --data "$2" "$API$1"
}

service_id() {
  jq -er '(.data // .).id'
}

remember() {
  CREATED_IDS+=("$1")
  CREATED_NAMES+=("$2")
}

forget() {
  local id="$1" index
  local next_ids=() next_names=()
  for index in "${!CREATED_IDS[@]}"; do
    if [ "${CREATED_IDS[$index]}" != "$id" ]; then
      next_ids+=("${CREATED_IDS[$index]}")
      next_names+=("${CREATED_NAMES[$index]}")
    fi
  done
  CREATED_IDS=("${next_ids[@]}")
  CREATED_NAMES=("${next_names[@]}")
}

assert_service() {
  local id="$1" expression="$2" description="$3"
  shift 3
  if ! api "/services/$id" | jq "$@" -e "$expression" >/dev/null; then
    echo "FAIL: $description" >&2
    exit 1
  fi
  echo "PASS: $description"
}

expect_failure() {
  local description="$1" pattern="$2"
  shift 2
  local output status
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e
  if [ "$status" -eq 0 ] || ! grep -qiE "$pattern" <<<"$output"; then
    echo "FAIL: $description (exit $status; expected $pattern)" >&2
    exit 1
  fi
  echo "PASS: $description"
}

SECRET_PATH="$TMP_DIR/cli-secret"
umask 077
printf 'cli-secret-%s' "$RUN_ID" > "$SECRET_PATH"
ENV_VALUE="cli-env-${RUN_ID}"

IMAGE_ID="$("$RENDER_BIN" services create --name "$IMAGE_NAME" --type web_service \
  --image traefik/whoami:v1.10.3 --region frankfurt --plan starter \
  --env-var WHOAMI_PORT_NUMBER=8080 --confirm -o json | service_id)"
remember "$IMAGE_ID" "$IMAGE_NAME"
assert_service "$IMAGE_ID" '.name == $name and .imagePath == "traefik/whoami:v1.10.3" and .ownerId == $owner' \
  "image create accepts the official CLI nested owner contract" --arg name "$IMAGE_NAME" --arg owner "$RENDER_WORKSPACE"
complete_leg image-create

"$RENDER_BIN" services update "$IMAGE_ID" --image traefik/whoami:latest --confirm -o json >/dev/null
assert_service "$IMAGE_ID" '.imagePath == "traefik/whoami:latest" and .ownerId == $owner' \
  "image update accepts the official CLI nested owner contract" --arg owner "$RENDER_WORKSPACE"
complete_leg image-update

# w4/052: an image service is created without --runtime, so before the fix GET
# returned no serviceDetails.runtime and the CLI refused any serviceDetails-field
# update client-side ("unsupported runtime"). The read now derives runtime
# "image", so a partial `services update --health-check-path` round-trips.
"$RENDER_BIN" services update "$IMAGE_ID" --health-check-path /healthz --confirm -o json >/dev/null
assert_service "$IMAGE_ID" '.serviceDetails.healthCheckPath == "/healthz" and .serviceDetails.runtime == "image" and .serviceDetails.env == "image"' \
  "official CLI round-trips --health-check-path on a runtime-less image service (w4/052)"
complete_leg image-healthcheck

"$RENDER_BIN" services delete "$IMAGE_ID" --confirm -o json >/dev/null
wait_service_gone "$IMAGE_ID" "$IMAGE_NAME"
forget "$IMAGE_ID"
complete_leg image-delete

DELETE_BUILD_ID="$("$RENDER_BIN" services create --name "$DELETE_BUILD_NAME" --type web_service \
  --repo https://github.com/render-examples/go-gin.git --branch main --runtime go \
  --region frankfurt --plan starter --build-command "go build ./..." \
  --start-command ./server --confirm -o json | service_id)"
remember "$DELETE_BUILD_ID" "$DELETE_BUILD_NAME"
"$RENDER_BIN" services delete "$DELETE_BUILD_ID" --confirm -o json >/dev/null
wait_service_gone "$DELETE_BUILD_ID" "$DELETE_BUILD_NAME"
forget "$DELETE_BUILD_ID"
complete_leg repo-delete-during-build

# w4/052 — the exact failing shape: a repo Dockerfile build with NO runtime set,
# which a dashboard, Blueprint, or hand-applied App produces but the CLI's create
# flags cannot (it always sends a runtime). Create it out-of-band via raw REST,
# confirm the read derives runtime "docker", then prove the official CLI can
# round-trip a serviceDetails-field update against it — the operation that was
# impossible before the fix (empty runtime → "unsupported runtime" / "cannot
# switch runtimes via the CLI").
# ownerId belongs in the create body (Render's create envelope), not as a
# query param — POST /v1/services rejects unknown query keys with 400.
BUILDER_ID="$(api_post "/services" \
  "$(jq -nc --arg name "$BUILDER_NAME" --arg owner "$RENDER_WORKSPACE" '{
    name: $name, type: "web_service", ownerId: $owner,
    repo: "https://github.com/render-examples/go-gin.git", branch: "main",
    serviceDetails: { plan: "starter", region: "frankfurt" }
  }')" | jq -er '.service.id')"
remember "$BUILDER_ID" "$BUILDER_NAME"
assert_service "$BUILDER_ID" '.serviceDetails.runtime == "docker" and .serviceDetails.env == "docker"' \
  "builder-expressed Dockerfile service (no runtime set) reads back runtime docker (w4/052)"
"$RENDER_BIN" services update "$BUILDER_ID" --health-check-path /healthz --pre-deploy-command "./server migrate" --confirm -o json >/dev/null
assert_service "$BUILDER_ID" '.serviceDetails.healthCheckPath == "/healthz" and .serviceDetails.preDeployCommand == "./server migrate" and .serviceDetails.runtime == "docker"' \
  "official CLI round-trips serviceDetails updates on a builder-expressed Dockerfile service (w4/052)"
"$RENDER_BIN" services delete "$BUILDER_ID" --confirm -o json >/dev/null
wait_service_gone "$BUILDER_ID" "$BUILDER_NAME"
forget "$BUILDER_ID"
complete_leg web-builder-roundtrip

web_args=(
  services create --name "$WEB_NAME" --type web_service
  --repo https://github.com/render-examples/go-gin.git --branch main
  --runtime go --region frankfurt --plan starter --num-instances 2
  --root-directory cmd/api --build-command "go build ./..." --start-command ./server
  --pre-deploy-command "./server migrate" --health-check-path /healthz
  --auto-deploy=false --build-filter-path 'cmd/**' --build-filter-ignored-path 'docs/**'
  --maintenance-mode --maintenance-mode-uri https://status.example.test/maintenance
  --max-shutdown-delay 41
  --ip-allow-list 'cidr=203.0.113.0/24,description=create-office'
  --confirm -o json
)
if [ "$MODE" = configured ]; then
  web_args+=(--env-var "CLI_PARITY_VALUE=$ENV_VALUE" --secret-file "cli-secret:$SECRET_PATH")
fi

WEB_ID="$("$RENDER_BIN" "${web_args[@]}" | service_id)"
remember "$WEB_ID" "$WEB_NAME"
assert_service "$WEB_ID" '
  .name == $name and .type == "web_service" and
  .repo == "https://github.com/render-examples/go-gin.git" and .branch == "main" and
  .rootDir == "cmd/api" and .autoDeploy == "no" and .replicas == 2 and
  .buildFilter == {paths:["cmd/**"], ignoredPaths:["docs/**"]} and
  .serviceDetails.runtime == "go" and .serviceDetails.plan == "starter" and
  .serviceDetails.healthCheckPath == "/healthz" and
  .serviceDetails.preDeployCommand == "./server migrate" and
  .serviceDetails.maxShutdownDelaySeconds == 41 and
  .serviceDetails.maintenanceMode == {enabled:true, uri:"https://status.example.test/maintenance"} and
  .serviceDetails.envSpecificDetails.buildCommand == "go build ./..." and
  .serviceDetails.envSpecificDetails.startCommand == "./server" and
  .serviceDetails.ipAllowList == [{cidrBlock:"203.0.113.0/24",description:"create-office"}]
' "web create preserves every applicable baseline flag" --arg name "$WEB_NAME"
complete_leg web-create

if [ "$MODE" = configured ]; then
  : "${BEX_VERIFY_APPS_NAMESPACE:?configured mode requires the App namespace}"
  KUBECONFIG="${KUBECONFIG:-}" kubectl get apps.app.bex.co -n "$BEX_VERIFY_APPS_NAMESPACE" \
    -l "bex.co/app-id=$WEB_ID" -o json | jq -e --arg value "$ENV_VALUE" \
    'any(.items[0].spec.env[]?; .name == "CLI_PARITY_VALUE" and .value == $value)' >/dev/null
  api "/services/$WEB_ID/secret-files/cli-secret" | \
    jq -e --rawfile content "$SECRET_PATH" '.name == "cli-secret" and .content == $content' >/dev/null
  echo "PASS: official CLI env-var and configured OpenBao secret-file values round-trip"
  complete_leg configured-env-secret
fi

"$RENDER_BIN" services update "$WEB_ID" --name "${WEB_NAME}-updated" --plan standard \
  --repo https://github.com/render-examples/go-echo.git --branch release \
  --root-directory services/api --build-command "go build ./cmd/..." --start-command ./api \
  --pre-deploy-command "./api migrate" --health-check-path /ready --auto-deploy=true \
  --build-filter-path 'services/**' --build-filter-ignored-path 'examples/**' \
  --maintenance-mode=false --maintenance-mode-uri https://status.example.test/ready \
  --max-shutdown-delay 42 \
  --ip-allow-list 'cidr=198.51.100.0/24,description=update-office' \
  --ip-allow-list 'cidr=2001:db8::/32,description=update-v6' --confirm -o json >/dev/null
assert_service "$WEB_ID" '
  .name == $name and .repo == "https://github.com/render-examples/go-echo.git" and
  .branch == "release" and .rootDir == "services/api" and .autoDeploy == "yes" and
  .buildFilter == {paths:["services/**"], ignoredPaths:["examples/**"]} and
  .serviceDetails.plan == "standard" and .serviceDetails.healthCheckPath == "/ready" and
  .serviceDetails.preDeployCommand == "./api migrate" and
  .serviceDetails.maxShutdownDelaySeconds == 42 and
  .serviceDetails.maintenanceMode == {enabled:false, uri:"https://status.example.test/ready"} and
  .serviceDetails.envSpecificDetails.buildCommand == "go build ./cmd/..." and
  .serviceDetails.envSpecificDetails.startCommand == "./api" and
  .serviceDetails.ipAllowList == [
    {cidrBlock:"198.51.100.0/24",description:"update-office"},
    {cidrBlock:"2001:db8::/32",description:"update-v6"}
  ]
' "web update replaces values without dropping allowlist descriptions" --arg name "${WEB_NAME}-updated"
complete_leg web-update

CRON_ID="$("$RENDER_BIN" services create --name "$CRON_NAME" --type cron_job \
  --repo https://github.com/render-examples/go-cron.git --branch main --runtime go \
  --region frankfurt --plan starter --root-directory jobs --build-command "go build ./..." \
  --cron-command "./job create" --cron-schedule '*/15 * * * *' --auto-deploy=false \
  --build-filter-path 'jobs/**' --build-filter-ignored-path 'docs/**' --confirm -o json | service_id)"
remember "$CRON_ID" "$CRON_NAME"
assert_service "$CRON_ID" '
  .type == "cron_job" and .repo == "https://github.com/render-examples/go-cron.git" and
  .branch == "main" and .rootDir == "jobs" and .autoDeploy == "no" and
  .buildFilter == {paths:["jobs/**"], ignoredPaths:["docs/**"]} and
  .serviceDetails.runtime == "go" and .serviceDetails.plan == "starter" and
  .serviceDetails.schedule == "*/15 * * * *" and
  .serviceDetails.envSpecificDetails.buildCommand == "go build ./..." and
  .serviceDetails.envSpecificDetails.startCommand == "./job create"
' "native cron create preserves command and schedule"
complete_leg cron-create

"$RENDER_BIN" services update "$CRON_ID" --plan standard \
  --repo https://github.com/render-examples/go-cron-v2.git --branch release \
  --root-directory tasks --build-command "go build ./cmd/job" --cron-command "./job update" \
  --cron-schedule '7 * * * *' --auto-deploy=true --build-filter-path 'tasks/**' \
  --build-filter-ignored-path 'examples/**' --confirm -o json >/dev/null
assert_service "$CRON_ID" '
  .repo == "https://github.com/render-examples/go-cron-v2.git" and .branch == "release" and
  .rootDir == "tasks" and .autoDeploy == "yes" and
  .buildFilter == {paths:["tasks/**"], ignoredPaths:["examples/**"]} and
  .serviceDetails.plan == "standard" and .serviceDetails.schedule == "7 * * * *" and
  .serviceDetails.envSpecificDetails.buildCommand == "go build ./cmd/job" and
  .serviceDetails.envSpecificDetails.startCommand == "./job update"
' "native cron update preserves cron command and other mutable fields"
complete_leg cron-update

STATIC_ID="$("$RENDER_BIN" services create --name "$STATIC_NAME" --type static_site \
  --repo https://github.com/render-examples/static-site.git --branch main \
  --root-directory site --build-command "npm run build" --publish-directory dist \
  --auto-deploy=false --build-filter-path 'site/**' --build-filter-ignored-path 'docs/**' \
  --ip-allow-list 'cidr=192.0.2.0/24,description=static-create' --confirm -o json | service_id)"
remember "$STATIC_ID" "$STATIC_NAME"
assert_service "$STATIC_ID" '
  .type == "static_site" and .repo == "https://github.com/render-examples/static-site.git" and
  .branch == "main" and .rootDir == "site" and .autoDeploy == "no" and
  .buildFilter == {paths:["site/**"], ignoredPaths:["docs/**"]} and
  .serviceDetails.buildCommand == "npm run build" and .serviceDetails.publishPath == "dist" and
  .serviceDetails.ipAllowList == [{cidrBlock:"192.0.2.0/24",description:"static-create"}]
' "static-site create preserves publish and allowlist metadata"
complete_leg static-create

"$RENDER_BIN" services update "$STATIC_ID" \
  --repo https://github.com/render-examples/static-site-v2.git --branch release \
  --root-directory web --build-command "npm run generate" --publish-directory public \
  --auto-deploy=true --build-filter-path 'web/**' --build-filter-ignored-path 'drafts/**' \
  --ip-allow-list 'cidr=192.0.2.128/25,description=static-update' --confirm -o json >/dev/null
assert_service "$STATIC_ID" '
  .repo == "https://github.com/render-examples/static-site-v2.git" and .branch == "release" and
  .rootDir == "web" and .autoDeploy == "yes" and
  .buildFilter == {paths:["web/**"], ignoredPaths:["drafts/**"]} and
  .serviceDetails.buildCommand == "npm run generate" and .serviceDetails.publishPath == "public" and
  .serviceDetails.ipAllowList == [{cidrBlock:"192.0.2.128/25",description:"static-update"}]
' "static-site update preserves publish and allowlist metadata"
complete_leg static-update

CLONE_ID="$("$RENDER_BIN" services create --from "$WEB_ID" --name "$CLONE_NAME" \
  --region frankfurt --confirm -o json | service_id)"
remember "$CLONE_ID" "$CLONE_NAME"
assert_service "$CLONE_ID" '.name == $name and .type == "web_service"' \
  "clone works with an explicit official-CLI region" --arg name "$CLONE_NAME"
complete_leg clone

SOURCE_REGION="$(api "/services/$WEB_ID" | jq -r '.serviceDetails.region // ""')"
case "$SOURCE_REGION" in
  frankfurt | ohio | oregon | singapore | virginia)
    BARE_CLONE_ID="$("$RENDER_BIN" services create --from "$WEB_ID" --name "$BARE_CLONE_NAME" --confirm -o json | service_id)"
    remember "$BARE_CLONE_ID" "$BARE_CLONE_NAME"
    echo "PASS: bare clone works because the configured platform region is in the CLI enum"
    complete_leg region-guard
    ;;
  *)
    expect_failure "bare clone fails explicitly on the CLI's closed region enum" \
      'region must be one of' "$RENDER_BIN" services create --from "$WEB_ID" \
      --name "$BARE_CLONE_NAME" --confirm -o json
    complete_leg region-guard
    ;;
esac

expect_failure "runtime update is rejected explicitly by the official CLI" \
  'cannot switch runtimes via the CLI' "$RENDER_BIN" services update "$WEB_ID" \
  --runtime python --confirm -o json
complete_leg runtime-guard
expect_failure "preview generation is rejected explicitly by bex" \
  'not supported by this platform' "$RENDER_BIN" services update "$WEB_ID" \
  --previews manual --confirm -o json
complete_leg preview-update
expect_failure "preview generation is rejected explicitly on create" \
  'not supported by this platform' "$RENDER_BIN" services create --name "$PREVIEW_NAME" \
  --type static_site --repo https://github.com/render-examples/static-site.git \
  --build-command true --publish-directory public --previews manual \
  --confirm -o json
complete_leg preview-create

if [ "$MODE" = configured ]; then
  bash scripts/registry-credential-cli-verify.sh
  complete_leg registry-credential
fi

assert_leg_census
echo "PASS: official Render CLI service create/update parity ($MODE)"
