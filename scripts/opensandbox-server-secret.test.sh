#!/usr/bin/env bash
# Tests for the two scripted restarts of the OpenSandbox lifecycle server that
# a deploy used to make unconditionally (w8/248). The server is one Recreate
# replica, so each restart is a window with no sandbox API. No cluster: kubectl
# is a stub on PATH.
#
#   bash scripts/opensandbox-server-secret.test.sh
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

SCRIPT="scripts/opensandbox-server-secret.sh"
WORKFLOW=".github/workflows/deploy.yml"
STUBS="$(mktemp -d)"
trap 'rm -rf "$STUBS"' EXIT
LOG="$STUBS/calls.log"
TOKEN="stub-control-plane-token-0123456789abcdef"

cat >"$STUBS/kubectl" <<'EOF_STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$STUB_LOG"
case "$*" in
  "get namespace "*) exit 0 ;;
  *"get secret bex-control-plane"*) printf '%s' "$STUB_TOKEN" | base64 | tr -d '\n'; exit 0 ;;
  *"create secret generic"*) echo "kind: Secret"; exit 0 ;;
  "apply -f -") cat >/dev/null; exit 0 ;;
  *"get secret opensandbox-server-auth -o jsonpath={.data.api_key}") printf '%s' "${STUB_LEGACY_KEY:-}"; exit 0 ;;
  *"patch secret opensandbox-server-auth"*) exit 0 ;;
  *"get secret opensandbox-server-auth -o jsonpath={.metadata.resourceVersion}") printf '%s' "${STUB_SECRET_VERSION-4711}"; exit 0 ;;
  *"get deployment/opensandbox-server") [ "${STUB_NO_DEPLOYMENT:-0}" = 1 ] && exit 1; exit 0 ;;
  *"patch deployment/opensandbox-server"*) exit 0 ;;
  *"get deployment "*" -o json") [ -n "${STUB_DEPLOYMENT_JSON:-}" ] || exit 1; printf '%s' "$STUB_DEPLOYMENT_JSON"; exit 0 ;;
esac
echo "stub kubectl: unexpected call: $*" >&2
exit 99
EOF_STUB
chmod +x "$STUBS/kubectl"

FAILURES=0
ok() { echo "ok   $1"; }
bad() {
  echo "FAIL $1"
  FAILURES=$((FAILURES + 1))
}

# mirror [ENV=VALUE ...]: run the script against the stub; sets $out and $rc.
mirror() {
  : >"$LOG"
  out="$(env PATH="$STUBS:$PATH" STUB_LOG="$LOG" STUB_TOKEN="$TOKEN" "$@" bash "$SCRIPT" 2>&1)"
  rc=$?
}

# --- the Secret mirror stamps the pod template; it never restarts the server ---
mirror
[ "$rc" -eq 0 ] && ok "mirrors the credential" || bad "mirror exited $rc: $out"
grep -qF 'patch deployment/opensandbox-server --type=merge' "$LOG" \
  && grep -qF '"bex.co/cp-token-secret-version":"4711"' "$LOG" \
  && ok "stamps the Secret's version on the pod template" \
  || bad "the pod template is not stamped with the Secret's version"
grep -q 'rollout restart' "$LOG" \
  && bad "restarts the lifecycle server on every run" \
  || ok "does not restart the lifecycle server itself"
{ grep -qF "$TOKEN" "$LOG" || grep -qF "$TOKEN" <<<"$out"; } \
  && bad "the token reached a command line or the output" \
  || ok "the token stays off command lines and output"

mirror STUB_NO_DEPLOYMENT=1
[ "$rc" -eq 0 ] && ! grep -q 'patch deployment' "$LOG" \
  && ok "a cluster without the Deployment yet is left alone" \
  || bad "patched a Deployment that does not exist (exit $rc)"

mirror STUB_SECRET_VERSION=
[ "$rc" -ne 0 ] && grep -qF 'could not read the version' <<<"$out" && ! grep -q 'patch deployment' "$LOG" \
  && ok "an unreadable Secret version fails instead of stamping nothing" \
  || bad "an unreadable Secret version was not refused (exit $rc)"

mirror STUB_LEGACY_KEY=bGVnYWN5
grep -qF '"path":"/data/api_key"' "$LOG" \
  && ok "still removes the legacy shared api_key" \
  || bad "the legacy shared api_key is no longer removed"

# --- the deploy's wait leaves a converged Deployment alone ---
# The function is defined inline in the workflow; take it from there so this
# tests the text that runs.
fn="$(awk '/^          opensandbox_converged\(\) \{/{take=1} take{print} take && /^          \}$/{exit}' "$WORKFLOW" | sed 's/^          //')"
if [ -z "$fn" ]; then
  bad "deploy.yml no longer defines opensandbox_converged"
else
  # shellcheck disable=SC1090
  source /dev/stdin <<<"$fn"
  sleep() { :; }
  # converged <deployment json>: 0 when the deploy would leave it alone.
  converged() {
    PATH="$STUBS:$PATH" STUB_LOG="$LOG" STUB_DEPLOYMENT_JSON="$1" opensandbox_converged opensandbox-server 1 2>/dev/null
  }
  state() { # <generation> <observed> <spec> <total> <updated> <available>
    printf '{"metadata":{"generation":%s},"spec":{"replicas":%s},"status":{"observedGeneration":%s,"replicas":%s,"updatedReplicas":%s,"availableReplicas":%s}}' \
      "$1" "$3" "$2" "$4" "$5" "$6"
  }
  converged "$(state 7 7 1 1 1 1)" && ok "a rolled, available Deployment is converged" || bad "a healthy Deployment reads as not converged"
  converged "$(state 7 6 1 1 1 1)" && bad "an unobserved generation reads as converged" || ok "waits for the controller to observe the generation"
  converged "$(state 7 7 1 1 1 0)" && bad "a Recreate roll with no available pod reads as converged" || ok "a Recreate roll in flight is not converged"
  converged "$(state 7 7 1 2 1 1)" && bad "an old pod still running reads as converged" || ok "a rolling update with an old pod left is not converged"
  converged "$(state 7 7 1 1 0 1)" && bad "an available old pod reads as converged" || ok "an old pod alone is not converged"
  converged '{"metadata":{"generation":1},"spec":{}}' && bad "a Deployment with no status reads as converged" || ok "a Deployment with no status is not converged"
  converged "" && bad "a missing Deployment reads as converged" || ok "a missing Deployment is not converged"
fi

# --- neither restart may come back unconditional ---
grep -q 'rollout restart' "$SCRIPT" \
  && bad "$SCRIPT restarts the lifecycle server again" \
  || ok "$SCRIPT has no rollout restart"
grep -A1 'rollout restart \\$' "$WORKFLOW" | grep -q 'deploy/opensandbox-server' \
  && bad "deploy.yml restarts the lifecycle server unconditionally again" \
  || ok "deploy.yml restarts a Deployment only after it failed to converge"

[ "$FAILURES" -eq 0 ] || { echo "$FAILURES failure(s)" >&2; exit 1; }
echo "all opensandbox-server-secret tests passed"
