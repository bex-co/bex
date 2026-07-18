#!/usr/bin/env bash
# Prove the unmodified render-oss/cli threads --registry-credential through
# service create and update and that bex binds it to the resulting App. Invoke through
# scripts/cli-compat.sh so RENDER_HOST/API_KEY/WORKSPACE and RENDER_BIN are set.
#
# Required registry inputs are deliberately external: the verifier never
# invents or prints a credential. Point them at an auth-enabled registry image
# the current Kubernetes cluster can pull.
#
#   REGISTRY_VERIFY_HOST=registry.example.com \
#   REGISTRY_VERIFY_USERNAME=robot \
#   REGISTRY_VERIFY_TOKEN=... \
#   REGISTRY_VERIFY_IMAGE=registry.example.com/acme/private-smoke:v1 \
#   BEX_VERIFY_APPS_NAMESPACE=dev-6 \
#     scripts/cli-compat.sh registry-credential-verify
set -euo pipefail

: "${RENDER_HOST:?invoke through scripts/cli-compat.sh}"
: "${RENDER_API_KEY:?invoke through scripts/cli-compat.sh}"
: "${RENDER_WORKSPACE:?set the target workspace}"
: "${RENDER_BIN:?set the official Render CLI binary}"
: "${REGISTRY_VERIFY_HOST:?set the private registry host}"
: "${REGISTRY_VERIFY_USERNAME:?set the private registry username}"
: "${REGISTRY_VERIFY_TOKEN:?set the private registry token/password}"
: "${REGISTRY_VERIFY_IMAGE:?set a pullable private image on REGISTRY_VERIFY_HOST}"
: "${BEX_VERIFY_APPS_NAMESPACE:?set the App namespace used by bex-api/operator}"

API="${RENDER_HOST%/}"
NAME="verify-rc-$$"
ANON_PROBE="$NAME-anon"
CREDENTIAL_IDS=""
CREATE_CREDENTIAL_ID=""
UPDATE_CREDENTIAL_ID=""
CREATED_CREDENTIAL_ID=""
SERVICE_ID=""

cleanup() {
  kubectl delete pod "$ANON_PROBE" -n "$BEX_VERIFY_APPS_NAMESPACE" \
    --ignore-not-found --wait=false >/dev/null 2>&1 || true
  if [ -n "$SERVICE_ID" ]; then
    "$RENDER_BIN" services delete "$SERVICE_ID" --confirm -o json >/dev/null 2>&1 || true
  fi
  for credential_id in $CREDENTIAL_IDS; do
    curl -sf -X DELETE -H "Authorization: Bearer $RENDER_API_KEY" \
      "$API/registry-credentials/$credential_id" >/dev/null 2>&1 || true
  done
}
trap cleanup EXIT
trap 'rc=$?; echo "FAIL: registry credential verifier stopped at line $LINENO (exit $rc)" >&2' ERR

# Prove the supplied image really requires authentication. Without this
# negative control, a public image plus an irrelevant credential could make
# the positive App leg green without exercising kubelet registry auth at all.
# Always forces a registry manifest request even when the node has cached the
# image's layers.
probe_overrides="$(jq -nc --arg name "$ANON_PROBE" --arg image "$REGISTRY_VERIFY_IMAGE" '{spec:{securityContext:{runAsNonRoot:true,seccompProfile:{type:"RuntimeDefault"}},containers:[{name:$name,image:$image,imagePullPolicy:"Always",securityContext:{allowPrivilegeEscalation:false,capabilities:{drop:["ALL"]}}}]}}')"
kubectl run "$ANON_PROBE" -n "$BEX_VERIFY_APPS_NAMESPACE" \
  --image="$REGISTRY_VERIFY_IMAGE" --image-pull-policy=Always \
  --restart=Never --overrides="$probe_overrides" >/dev/null
anonymous_reason=""
deadline=$((SECONDS + 60))
while [ "$SECONDS" -lt "$deadline" ]; do
  anonymous_phase="$(kubectl get pod "$ANON_PROBE" -n "$BEX_VERIFY_APPS_NAMESPACE" \
    -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  anonymous_reason="$(kubectl get pod "$ANON_PROBE" -n "$BEX_VERIFY_APPS_NAMESPACE" \
    -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || true)"
  case "$anonymous_reason" in
    ErrImagePull | ImagePullBackOff) break ;;
  esac
  case "$anonymous_phase" in
    Running | Succeeded)
      echo "FAIL: anonymous control pulled $REGISTRY_VERIFY_IMAGE; use a genuinely private image" >&2
      exit 1
      ;;
  esac
  sleep 2
done
case "$anonymous_reason" in
  ErrImagePull | ImagePullBackOff) ;;
  *)
    echo "FAIL: anonymous control did not reach an image-auth failure (reason ${anonymous_reason:-<empty>})" >&2
    exit 1
    ;;
esac
kubectl delete pod "$ANON_PROBE" -n "$BEX_VERIFY_APPS_NAMESPACE" --wait=true >/dev/null

create_credential() {
  local credential_name="$1" payload response credential_id
  payload="$(jq -nc \
    --arg ownerId "$RENDER_WORKSPACE" \
    --arg name "$credential_name" \
    --arg host "$REGISTRY_VERIFY_HOST" \
    --arg username "$REGISTRY_VERIFY_USERNAME" \
    --arg authToken "$REGISTRY_VERIFY_TOKEN" \
    '{$ownerId,$name,$host,$username,$authToken}')"
  response="$(curl -sf -X POST \
    -H "Authorization: Bearer $RENDER_API_KEY" \
    -H 'Content-Type: application/json' \
    --data-binary "$payload" "$API/registry-credentials")"
  credential_id="$(jq -er '.id' <<<"$response")"
  CREDENTIAL_IDS="$CREDENTIAL_IDS $credential_id"
  CREATED_CREDENTIAL_ID="$credential_id"
}

create_credential "$NAME-create"
CREATE_CREDENTIAL_ID="$CREATED_CREDENTIAL_ID"
create_credential "$NAME-update"
UPDATE_CREDENTIAL_ID="$CREATED_CREDENTIAL_ID"

create_response="$(
  # A background worker has no HTTP readiness probe, so the verifier tests the
  # registry pull itself without assuming anything about the container port.
  "$RENDER_BIN" services create --name "$NAME" --type background_worker \
    --image "$REGISTRY_VERIFY_IMAGE" --registry-credential "$CREATE_CREDENTIAL_ID" \
    --plan free --num-instances 1 --confirm -o json
)"
SERVICE_ID="$(jq -er '(.data // .).id' <<<"$create_response")"

service_response="$(curl -sf -H "Authorization: Bearer $RENDER_API_KEY" "$API/services/$SERVICE_ID")"
jq -e --arg credential "$CREATE_CREDENTIAL_ID" '.registryCredentialId == $credential' \
  <<<"$service_response" >/dev/null
jq -e --arg credential "$CREATE_CREDENTIAL_ID" --arg name "$NAME-create" \
  '.registryCredential == {id: $credential, name: $name}' \
  <<<"$service_response" >/dev/null

# A distinct id makes this an independent update assertion: if PATCH silently
# drops the flag, the create-time id remains and the checks below fail.
"$RENDER_BIN" services update "$SERVICE_ID" \
  --image "$REGISTRY_VERIFY_IMAGE" --registry-credential "$UPDATE_CREDENTIAL_ID" \
  --confirm -o json >/dev/null
service_response="$(curl -sf -H "Authorization: Bearer $RENDER_API_KEY" "$API/services/$SERVICE_ID")"
jq -e --arg credential "$UPDATE_CREDENTIAL_ID" '.registryCredentialId == $credential' \
  <<<"$service_response" >/dev/null
jq -e --arg credential "$UPDATE_CREDENTIAL_ID" --arg name "$NAME-update" \
  '.registryCredential == {id: $credential, name: $name}' \
  <<<"$service_response" >/dev/null

APP_NAME=""
PULL_SECRET=""
bound_credential=""
deadline=$((SECONDS + 60))
while [ "$SECONDS" -lt "$deadline" ]; do
  APP_NAME="$(kubectl get apps.app.bex.co -n "$BEX_VERIFY_APPS_NAMESPACE" \
    -l "bex.co/app-id=$SERVICE_ID" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [ -n "$APP_NAME" ]; then
    bound_credential="$(kubectl get app.app.bex.co "$APP_NAME" -n "$BEX_VERIFY_APPS_NAMESPACE" \
      -o jsonpath='{.spec.registryCredentialId}' 2>/dev/null || true)"
    PULL_SECRET="$(kubectl get app.app.bex.co "$APP_NAME" -n "$BEX_VERIFY_APPS_NAMESPACE" \
      -o jsonpath='{.spec.externalRegistryPullSecret}' 2>/dev/null || true)"
  fi
  [ "$bound_credential" = "$UPDATE_CREDENTIAL_ID" ] && [ -n "$PULL_SECRET" ] && break
  sleep 1
done
[ -n "$APP_NAME" ]
[ "$bound_credential" = "$UPDATE_CREDENTIAL_ID" ]
[ -n "$PULL_SECRET" ]
[ "$(kubectl get secret "$PULL_SECRET" -n "$BEX_VERIFY_APPS_NAMESPACE" \
  -o jsonpath='{.type}')" = "kubernetes.io/dockerconfigjson" ]

deadline=$((SECONDS + 300))
phase=""
while [ "$SECONDS" -lt "$deadline" ]; do
  phase="$(kubectl get app.app.bex.co "$APP_NAME" -n "$BEX_VERIFY_APPS_NAMESPACE" \
    -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  [ "$phase" = "Running" ] && break
  [ "$phase" = "Failed" ] && break
  sleep 2
done
[ "$phase" = "Running" ] || {
  echo "FAIL: $APP_NAME reached phase ${phase:-<empty>}, not Running" >&2
  exit 1
}

jq -e --arg secret "$PULL_SECRET" \
  'any(.spec.template.spec.imagePullSecrets[]?; .name == $secret)' \
  < <(kubectl get deployment "$APP_NAME" -n "$BEX_VERIFY_APPS_NAMESPACE" -o json) >/dev/null

POD_NAME="$(kubectl get pods -n "$BEX_VERIFY_APPS_NAMESPACE" \
  -l "app.bex.co/app=$APP_NAME" -o jsonpath='{.items[0].metadata.name}')"
[ -n "$POD_NAME" ]
[ "$(kubectl get pod "$POD_NAME" -n "$BEX_VERIFY_APPS_NAMESPACE" \
  -o jsonpath='{.status.containerStatuses[0].state.running.startedAt}')" != "" ]
jq -e --arg pod "$POD_NAME" \
  'any(.items[]; .involvedObject.kind == "Pod" and .involvedObject.name == $pod and .reason == "Pulled")' \
  < <(kubectl get events -n "$BEX_VERIFY_APPS_NAMESPACE" -o json) >/dev/null

echo "PASS: anonymous pull was rejected; official CLI created $SERVICE_ID with one credential, replaced it with another, and kubelet pulled the private image to Running"
