#!/usr/bin/env bash
# Production checks for the render-v1 container policy (w6/088), against the
# cluster in $KUBECONFIG. Driven by .github/workflows/container-policy-drill.yml.
#
#   NS=tea-... bash scripts/container-policy-drill.sh list
#   NS=tea-... SERVICE_ID=srv-... bash scripts/container-policy-drill.sh migrate          # dry run
#   NS=tea-... SERVICE_ID=srv-... CONFIRM=srv-... bash scripts/container-policy-drill.sh migrate
#   NS=tea-... bash scripts/container-policy-drill.sh admission
#
# list and admission write nothing (admission uses server-side dry runs while
# impersonating the operator). migrate without CONFIRM is bex-api's own dry
# run; with CONFIRM equal to SERVICE_ID it applies to exactly that service.
set -euo pipefail

mode="${1:?usage: container-policy-drill.sh list|migrate|admission}"
: "${NS:?NS (tenant namespace) is required}"
[[ "$NS" =~ ^tea-[a-z0-9]+$ ]] || { echo "NS must be a tenant namespace (tea-...)" >&2; exit 1; }
OPERATOR="system:serviceaccount:bex-system:bex-controller-manager"

section() { printf '\n=== %s\n' "$*"; }

list() {
  section "Apps in $NS by container policy"
  kubectl -n "$NS" get apps.app.bex.co \
    -o custom-columns='APP:.metadata.name,SERVICE:.metadata.labels.bex\.co/app-id,POLICY:.spec.containerPolicy,IMAGE:.spec.image,REPO:.spec.repo,BUILDER:.spec.builder,TYPE:.spec.type,SUSPENDED:.spec.suspended,PHASE:.status.phase'
}

migrate() {
  : "${SERVICE_ID:?SERVICE_ID (srv-...) is required}"
  [[ "$SERVICE_ID" =~ ^srv-[a-z0-9]+$ ]] || { echo "SERVICE_ID must be srv-..." >&2; exit 1; }
  local args=("$SERVICE_ID")
  if [ -n "${CONFIRM:-}" ]; then
    [ "$CONFIRM" = "$SERVICE_ID" ] || { echo "CONFIRM must exactly match SERVICE_ID to apply" >&2; exit 1; }
    args+=(--apply)
  fi
  section "migrate-container-policy ${args[*]}"
  kubectl -n bex-system exec deploy/bex-api -c api -- /api migrate-container-policy "${args[@]}"
}

# Server-side dry runs as the operator against a live render-v1 Deployment:
# the policy must refuse a capability outside render-v1's set and a foreign
# Localhost profile, and admit an unchanged template (the control, which shows
# the refusals are the policy's and not the impersonation's).
admission() {
  local app
  app="$(kubectl -n "$NS" get deploy -l app.bex.co/container-policy=render-v1 -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [ -z "$app" ]; then
    app="$(kubectl -n "$NS" get deploy -o json | jq -r '[.items[] | select(.spec.template.metadata.labels["app.bex.co/container-policy"]=="render-v1")][0].metadata.name // empty')"
  fi
  [ -n "$app" ] || { echo "no render-v1 Deployment in $NS to test against" >&2; exit 1; }
  section "admission dry runs against deploy/$app as the operator"
  local failures=0
  check() { # name, expect (admit|refuse), patch
    local out rc=0
    out="$(kubectl -n "$NS" patch "deploy/$app" --as="$OPERATOR" --dry-run=server --type=json -p "$3" 2>&1)" || rc=$?
    if { [ "$2" = admit ] && [ $rc -eq 0 ]; } || { [ "$2" = refuse ] && [ $rc -ne 0 ] && grep -q "bex-operator-workloads" <<<"$out"; }; then
      echo "PASS $1 ($2): ${out:0:220}"
    else
      echo "FAIL $1 (want $2, rc=$rc): ${out:0:400}"
      failures=$((failures + 1))
    fi
  }
  check "unchanged template" admit '[{"op":"add","path":"/metadata/annotations/drill.bex.co~1noop","value":"1"}]'
  check "capability outside render-v1" refuse '[{"op":"add","path":"/spec/template/spec/containers/0/securityContext/capabilities/add/-","value":"NET_ADMIN"}]'
  check "foreign Localhost profile" refuse '[{"op":"replace","path":"/spec/template/spec/containers/0/securityContext/seccompProfile","value":{"type":"Localhost","localhostProfile":"profiles/other.json"}}]'
  [ "$failures" -eq 0 ] || exit 1
}

case "$mode" in
  list) list ;;
  migrate) migrate ;;
  admission) admission ;;
  *) echo "unknown mode: $mode" >&2; exit 2 ;;
esac
