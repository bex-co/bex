#!/usr/bin/env bash
# Self-test for wait-capi-rollout.sh's retry of a worker Machine whose Hetzner
# server was refused (stock-out), with kubectl stubbed on PATH. No cluster.
#
# The fleet is a healthy control plane plus one MachineDeployment (bex-platform,
# 3 replicas) with a fourth, surge Machine that never got a server. Deleting that
# Machine heals the fleet, as it does in production.
set -euo pipefail
cd "$(dirname "$0")/.."

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
bin="$work/bin"
state="$work/state"
mkdir -p "$bin" "$state"

iso_ago() {
  python3 -I -c 'import datetime, sys
print((datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(seconds=int(sys.argv[1]))).strftime("%Y-%m-%dT%H:%M:%SZ"))' "$1"
}

machine() { # name, phase, node (omitted: no Node yet)
  local node_ref=""
  [ -z "${3:-}" ] || node_ref=",\"nodeRef\":{\"name\":\"$3\"}"
  printf '{"metadata":{"name":"%s"},"spec":{"version":"v1.34.9"},"status":{"phase":"%s"%s}}' "$1" "$2" "$node_ref"
}

cat > "$state/kcp.json" <<'JSON'
{"spec":{"version":"v1.34.9","replicas":3},"status":{"readyReplicas":3,
 "conditions":[{"type":"Available","status":"True"},{"type":"RollingOut","status":"False"}]}}
JSON
echo '{"items":[{"metadata":{"name":"bex-platform"}}]}' > "$state/mds.json"
echo '{"spec":{"replicas":3,"template":{"spec":{"version":"v1.34.9"}}},"status":{"readyReplicas":3,"availableReplicas":3}}' \
  > "$state/md.json"
printf '{"items":[%s,%s,%s]}' "$(machine cp-a Running cp-a)" "$(machine cp-b Running cp-b)" "$(machine cp-c Running cp-c)" \
  > "$state/cp-machines.json"
healed=$(printf '%s,%s,%s' "$(machine plat-a Running plat-a)" "$(machine plat-b Running plat-b)" "$(machine plat-c Running plat-c)")
printf '{"items":[%s]}' "$healed" > "$state/md-machines.healed.json"
printf '{"items":[%s,%s]}' "$healed" "$(machine plat-d Provisioning)" > "$state/md-machines.stalled.json"
echo '{"items":[]}' > "$state/hcms.healed.json"

# hcloudmachines: plat-d has no server; $1 = its creation time, $2 = providerID.
# CAPH rewrites the failure condition's lastTransitionTime minutes after the
# failure, so it is always fresh here: the age must come from the creation time.
stall() {
  cat > "$state/hcms.stalled.json" <<JSON
{"items":[
 {"metadata":{"name":"plat-a","creationTimestamp":"$(iso_ago 86400)"},"spec":{"providerID":"hcloud://1"},
  "status":{"conditions":[{"type":"Ready","status":"True"}]}},
 {"metadata":{"name":"plat-d","creationTimestamp":"$1","ownerReferences":[{"kind":"Machine","name":"plat-d"}]},
  "spec":{"providerID":"$2"},
  "status":{"conditions":[{"type":"Ready","status":"False","reason":"ServerCreateFailedIrrecoverableError",
   "message":"failed to create HCloud server in fsn1 (type cx33): error during placement (resource_unavailable, abc)",
   "lastTransitionTime":"$(iso_ago 1)"}]}}]}
JSON
}
owned_by() { # kind of the Machine plat-d's owner
  echo "{\"metadata\":{\"ownerReferences\":[{\"kind\":\"$1\",\"name\":\"x\"}]}}" > "$state/machine-plat-d.json"
}
reset() {
  cp "$state/md-machines.stalled.json" "$state/md-machines.json"
  cp "$state/hcms.stalled.json" "$state/hcms.json"
  rm -f "$state/deleted" "$state/kubectl.log"
}

cat > "$bin/kubectl" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
args=()
for a in "$@"; do case "$a" in --request-timeout=*) ;; *) args+=("$a") ;; esac; done
set -- "${args[@]}"
echo "$*" >> "$STATE/kubectl.log"
case "$*" in
  "get kubeadmcontrolplane bex-control-plane -n bex-capi -o json") cat "$STATE/kcp.json" ;;
  "get machinedeployments -n bex-capi -o json") cat "$STATE/mds.json" ;;
  "get machinedeployment bex-platform -n bex-capi -o json") cat "$STATE/md.json" ;;
  "get machines -n bex-capi -l cluster.x-k8s.io/control-plane -o json") cat "$STATE/cp-machines.json" ;;
  "get machines -n bex-capi -l cluster.x-k8s.io/deployment-name=bex-platform -o json") cat "$STATE/md-machines.json" ;;
  "get hcloudmachines -n bex-capi -o json") cat "$STATE/hcms.json" ;;
  "get machine "*" -n bex-capi -o json") cat "$STATE/machine-$3.json" ;;
  "get node "*"-o jsonpath={.status.nodeInfo.kubeletVersion}") printf 'v1.34.9' ;;
  "delete machine "*" -n bex-capi --wait=false")
    echo "$3" >> "$STATE/deleted"
    cp "$STATE/md-machines.healed.json" "$STATE/md-machines.json"
    cp "$STATE/hcms.healed.json" "$STATE/hcms.json"
    ;;
  "get "*) echo "NAME   (diagnostic table)" ;;
  *) echo "unexpected kubectl call: $*" >&2; exit 1 ;;
esac
STUB
chmod +x "$bin/kubectl"

run() {
  PATH="$bin:$PATH" STATE="$state" POLL_SECONDS=1 "$@" bash scripts/wait-capi-rollout.sh \
    > "$work/stdout" 2> "$work/stderr"
}
fail() { echo "FAIL: $*"; cat "$work/stdout" "$work/stderr"; exit 1; }

# A surge that Hetzner refused a server for, created long enough ago: its
# MachineSet-owned Machine is deleted once and the fleet converges.
stall "$(iso_ago 900)" ""; owned_by MachineSet; reset
run env TIMEOUT_SECONDS=30 SERVER_CREATE_RETRY_SECONDS=300 || fail "a retried stock-out should converge"
[ "$(cat "$state/deleted")" = plat-d ] || fail "want exactly plat-d deleted, got: $(cat "$state/deleted")"
grep -q "resource_unavailable" "$work/stdout" || fail "the Hetzner error is not printed"
grep -q "CAPI fleet converged" "$work/stdout" || fail "no convergence report"

# A Machine younger than the retry interval is left alone (the attempts are paced),
# so the gate times out with the stall visible in its diagnostics.
stall "$(iso_ago 5)" ""; owned_by MachineSet; reset
if run env TIMEOUT_SECONDS=3 SERVER_CREATE_RETRY_SECONDS=300; then fail "a young stall must not converge"; fi
[ ! -e "$state/deleted" ] || fail "a young failure was deleted"
grep -q "timed out after 3s" "$work/stderr" || fail "no timeout message"
grep -q "hcloudmachines" "$state/kubectl.log" || fail "diagnostics omit hcloudmachines"

# A Machine that carries a providerID has a server: never deleted, however old.
stall "$(iso_ago 900)" "hcloud://9"; owned_by MachineSet; reset
if run env TIMEOUT_SECONDS=3 SERVER_CREATE_RETRY_SECONDS=300; then fail "a Machine with a server must not converge here"; fi
[ ! -e "$state/deleted" ] || fail "a Machine with a server was deleted"

# A control-plane Machine goes through KCP remediation: reported once, not deleted.
stall "$(iso_ago 900)" ""; owned_by KubeadmControlPlane; reset
if run env TIMEOUT_SECONDS=3 SERVER_CREATE_RETRY_SECONDS=300; then fail "a stalled control plane must not converge"; fi
[ ! -e "$state/deleted" ] || fail "a KCP-owned Machine was deleted"
[ "$(grep -c 'not a MachineSet' "$work/stderr")" = 1 ] || fail "the KCP-owned warning must print exactly once"

# A bad interval is refused before any kubectl call.
if run env SERVER_CREATE_RETRY_SECONDS=0; then fail "a zero retry interval must be refused"; fi
grep -q "must be positive integers" "$work/stderr" || fail "no input error"

echo "wait-capi-rollout.test.sh: ok"
