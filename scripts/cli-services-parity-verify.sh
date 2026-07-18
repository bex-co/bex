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
  exit 0
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
RUN_ID="$(date -u +%H%M%S)-$$"
PREFIX="cp-${RUN_ID}"
WEB_NAME="${PREFIX}-web"
CRON_NAME="${PREFIX}-cron"
STATIC_NAME="${PREFIX}-static"
CLONE_NAME="${PREFIX}-clone"
BARE_CLONE_NAME="${PREFIX}-bare"
PREVIEW_NAME="${PREFIX}-preview"
CREATED_IDS=""
TMP_DIR="$(mktemp -d)"

cleanup() {
  for id in $CREATED_IDS; do
    "$RENDER_BIN" services delete "$id" --confirm -o json >/dev/null 2>&1 || true
  done
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM
trap 'rc=$?; echo "FAIL: service parity verifier stopped at line $LINENO (exit $rc)" >&2' ERR

api() {
  curl -sf -H "Authorization: Bearer $RENDER_API_KEY" "$API$1"
}

service_id() {
  jq -er '(.data // .).id'
}

remember() {
  CREATED_IDS="$CREATED_IDS $1"
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
remember "$WEB_ID"
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
  .ipAllowList == [{cidrBlock:"203.0.113.0/24",description:"create-office"}]
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
  .ipAllowList == [
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
remember "$CRON_ID"
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
remember "$STATIC_ID"
assert_service "$STATIC_ID" '
  .type == "static_site" and .repo == "https://github.com/render-examples/static-site.git" and
  .branch == "main" and .rootDir == "site" and .autoDeploy == "no" and
  .buildFilter == {paths:["site/**"], ignoredPaths:["docs/**"]} and
  .serviceDetails.buildCommand == "npm run build" and .serviceDetails.publishPath == "dist" and
  .ipAllowList == [{cidrBlock:"192.0.2.0/24",description:"static-create"}]
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
  .ipAllowList == [{cidrBlock:"192.0.2.128/25",description:"static-update"}]
' "static-site update preserves publish and allowlist metadata"
complete_leg static-update

CLONE_ID="$("$RENDER_BIN" services create --from "$WEB_ID" --name "$CLONE_NAME" \
  --region frankfurt --confirm -o json | service_id)"
remember "$CLONE_ID"
assert_service "$CLONE_ID" '.name == $name and .type == "web_service"' \
  "clone works with an explicit official-CLI region" --arg name "$CLONE_NAME"
complete_leg clone

SOURCE_REGION="$(api "/services/$WEB_ID" | jq -r '.serviceDetails.region // ""')"
case "$SOURCE_REGION" in
  frankfurt | ohio | oregon | singapore | virginia)
    BARE_CLONE_ID="$("$RENDER_BIN" services create --from "$WEB_ID" --name "$BARE_CLONE_NAME" --confirm -o json | service_id)"
    remember "$BARE_CLONE_ID"
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
