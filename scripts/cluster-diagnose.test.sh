#!/usr/bin/env bash
# Self-test for scripts/cluster-diagnose.sh and scripts/lib/public_log.py
# (w5/m501). A stub kubectl serves recorded output (scripts/fixtures/
# cluster-diagnose/, every identifier synthetic: fake ids, documentation
# address ranges), so no cluster or credential is needed.
#   * --public-log, what the public Actions log shows, carries no tenant
#     namespace or id, service or pod name, node name or address, image or
#     tenant event text, and still reports counts, reasons, readiness,
#     allocation and autoscaler state;
#   * the local run prints the full detail;
#   * the redactor's patterns, keyed and stable hashes, and the
#     ::add-mask:: passthrough; mask-inputs.
set -euo pipefail
cd "$(dirname "$0")/.."

fixtures="$PWD/scripts/fixtures/cluster-diagnose"
work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT
mkdir -p "$work/bin"
fails=0
fail() { echo "FAIL: $*" >&2; fails=$((fails + 1)); }
ok() { echo "ok: $*"; }

# A fixture that .gitignore matches (*.log was one) is on the author's disk and
# missing from every checkout: the test passes there and fails in CI.
for fx in "$fixtures"/*; do
  ! git check-ignore -q -- "$fx" || fail "fixture ${fx#"$PWD"/} is gitignored, so CI would not have it"
done

# The stub answers by the arguments' shape, most specific first, and logs
# every call so the test can prove what was (not) read.
cat >"$work/bin/kubectl" <<'STUB'
#!/usr/bin/env bash
f="$FIXTURES"
echo "$*" >>"$KUBECTL_LOG"
case "$*" in
  "get --raw=/readyz"*) [ "${STUB_UNREACHABLE:-}" = 1 ] && { echo "Unable to connect to the server" >&2; exit 1; }; echo ok ;;
  "get namespace kube-system -o jsonpath={.metadata.uid}"*) echo 11111111-2222-3333-4444-555555555555 ;;
  "get nodes -o jsonpath="*) jq -r '[.items[].metadata.name] | join(" ")' "$f/nodes.json" ;;
  "get machines.cluster.x-k8s.io -A -o jsonpath="*) jq -r '[.items[] | select(.kind == "Machine") | .metadata.name] | join(" ")' "$f/capi.json" ;;
  "get nodes --request-timeout"*"-o json") cat "$f/nodes.json" ;;
  "get pods -A -l bex.co/app-id="*"-o json") cat "$f/service-pods.json" ;;
  "get events -n tea-c0ffee00000000000001 --field-selector involvedObject.kind=Pod,involvedObject.name=tea-c0ffee00000000000001-secret-shop-5d8f7c9b4-x2x9z"*) cat "$f/events.json" ;;
  "get events -n "*) echo '{"items": []}' ;;
  "get deploy,rs -A -l bex.co/app-id="*"-o json") cat "$f/workloads.json" ;;
  "get pods -A --field-selector=status.phase=Pending"*"-o json") cat "$f/pending.json" ;;
  "describe nodes"*) cat "$f/describe-nodes.txt" ;;
  "get machinedeployments.cluster.x-k8s.io,machines.cluster.x-k8s.io -n bex-capi"*) cat "$f/capi.json" ;;
  "get configmap -A --field-selector=metadata.name=cluster-autoscaler-status"*"-o json") cat "$f/autoscaler-status.json" ;;
  "get configmap -A --field-selector=metadata.name=cluster-autoscaler-status -o jsonpath="*) jq -r '.items[].data.status' "$f/autoscaler-status.json" ;;
  # The selector matches by label; the stub also answers with a tenant pod
  # whose name says autoscaler, which must never be read.
  "get pods -A -l app.kubernetes.io/instance=cluster-autoscaler"*"-o json") cat "$f/autoscaler-pods.json" ;;
  "logs -n bex-capi cluster-autoscaler-clusterapi-6c7d8-qwert"*) cat "$f/autoscaler-log.txt" ;;
  "logs -n "*) echo "TENANT-LOG-LINE error scale up from the tenant's own autoscaler" ;;
  # The local (full-detail) reads.
  "get pods -A -l bex.co/app-id="*"-o wide") jq -r '.items[] | "\(.metadata.namespace) \(.metadata.name) \(.status.phase) \(.spec.nodeName // "<none>") \(.status.podIP // "<none>")"' "$f/service-pods.json" ;;
  "get pods -A -l bex.co/app-id="*"-o jsonpath="*) jq -r '.items[] | "\(.metadata.namespace)/\(.metadata.name)"' "$f/service-pods.json" ;;
  "describe pod -n "*) printf 'Status: Running\nNode: bex-tenant-0-abcde-fghij/10.10.0.17\nEvents:\n  Warning Unhealthy Liveness probe failed: secret-shop internal error\n' ;;
  "get pods -A --field-selector=status.phase=Pending -o wide") jq -r '.items[] | "\(.metadata.namespace) \(.metadata.name)"' "$f/pending.json" ;;
  "get nodes -L nodepool,bex.co/pool -o wide") jq -r '.items[] | "\(.metadata.name) \([.status.addresses[].address] | join(" "))"' "$f/nodes.json" ;;
  "get pods -A -o jsonpath="*) jq -r '.items[] | "\(.metadata.namespace)/\(.metadata.name)"' "$f/all-pods.json" ;;
  *) echo "stub kubectl: unexpected call: $*" >&2; exit 1 ;;
esac
STUB
chmod +x "$work/bin/kubectl"
export PATH="$work/bin:$PATH" FIXTURES="$fixtures" KUBECTL_LOG="$work/kubectl.log"
unset PUBLIC_LOG_KEY

sid=srv-c0ffee0000000000000a

# --- --public-log --------------------------------------------------------------
: >"$KUBECTL_LOG"
if bash scripts/cluster-diagnose.sh "$sid" --public-log >"$work/public.out" 2>"$work/public.err"; then
  ok "--public-log run exits 0"
else
  fail "--public-log run failed: $(cat "$work/public.err")"
fi
grep -q 'unexpected call' "$work/public.err" && fail "--public-log made a call the stub does not know: $(grep unexpected "$work/public.err")"

# Nothing private survives: every recorded tenant identifier, node name and
# address, the image, and the tenant's own event and log text.
for needle in tea-c0ffee srv-c0ffee secret-shop acme-billing acme-handmade acme-worker hello-private \
  bex-tenant-0-abcde bex-control-plane-kkkkk bex-gpu-aaaaa 0a1b2c3d-0000 \
  10.10.0. 10.244. 203.0.113. 198.51.100. 2001:db8 123456789 \
  'Liveness probe' statuscode 'internal error' ghcr.io 'ran out of memory' 'restarting failed container' \
  TENANT-LOG-LINE TENANT-CONFIGMAP 'Successfully assigned'; do
  if grep -qF -- "$needle" "$work/public.out"; then
    fail "--public-log printed '$needle': $(grep -F -- "$needle" "$work/public.out" | head -1)"
  fi
done
leak_re='(^|[^a-z0-9])[a-z]{3}-[0-9a-v]{20}|([0-9]{1,3}\.){3}[0-9]{1,3}'
if grep -qE "$leak_re" "$work/public.out"; then
  fail "--public-log printed an id- or IPv4-shaped string: $(grep -E "$leak_re" "$work/public.out" | head -1)"
fi
ok "--public-log carries no tenant identifier, node name or address, image or tenant event text"

for want in 'pods: 2 (Pending 1, Running 1)' 'waiting reason=CrashLoopBackOff' 'restarts=7' \
  'last terminated reason=OOMKilled exitCode=137' 'condition PodScheduled=False reason=Unschedulable' \
  'event Warning Unhealthy x41' 'event Warning Failed x2' \
  'FailedScheduling x9 (' '2 Insufficient cpu' 'TriggeredScaleUp x1' 'MachineDeployment/bex-capi/bex-tenant-0 6->7' \
  'Deployment ws#' 'desired=2 ready=0 available=0 updated=2 Available=False(MinimumReplicasUnavailable)' 'revision=12' \
  'pending: 7 (tenant 6, platform 1) by scheduling: Scheduled 4, Unschedulable 3' \
  'bex-system/bex-api-6d5f8c7b9-xyz12' "didn't match Pod's node affinity/selector" 'waiting=ContainerCreating' \
  'bex-build/bld-ws#' 'default/pod#' '-sandbox/pod#' \
  'pool=tenant  Ready=True' 'pressure=MemoryPressure' 'cpu req 3967m (99%) lim 7900m (197%)' 'memory req 7138Mi (93%)' \
  'pool=control-plane  Ready=True' 'Ready=Unknown  schedulable=no' 'taints=node.kubernetes.io/unreachable:NoExecute' \
  'nodes: 3 (Ready 2)' 'MachineDeployment bex-tenant-0  desired=7 replicas=6 readyReplicas=6' 'phase=ScalingUp' \
  'pool=bex-gpu  phase=Provisioned' 'not=NodeHealthy(NodeConditionsFailed)' \
  'autoscalerStatus: Running' 'candidates: node#' 'is unschedulable' 'Final scale-up plan' \
  'node group min size reached' 'hcloud://server#' "can't be scheduled: 0/3 nodes are available: 3 Insufficient cpu/memory"; do
  grep -qF -- "$want" "$work/public.out" || fail "--public-log lost '$want'"
done
ok "--public-log still reports counts, reasons, readiness, allocation, Machines and the autoscaler"

# The Ready=False condition beside the hashes: one pod keeps one name. The
# pending service pod appears under Service pods and Pending pods alike.
pending_tag="$(grep -E '^ws#[0-9a-f]{6}/ws#[0-9a-f]{6}~[0-9a-f]{6}  phase=Pending' "$work/public.out" | awk '{print $1}')"
[ -n "$pending_tag" ] && [ "$(grep -cF -- "$pending_tag" "$work/public.out")" -ge 2 ] \
  || fail "the pending service pod's hash is not the same in both sections ('$pending_tag')"
# The node the service pod runs on is the node listed under Nodes.
node_tag="$(grep -oE 'node=node#[0-9a-f]{6}' "$work/public.out" | head -1 | cut -d= -f2)"
grep -qE "^${node_tag}  pool=tenant" "$work/public.out" || fail "the service pod's node hash ($node_tag) is not a listed node"
ok "a hashed name is stable within the log"

# A tenant pod whose name contains "autoscaler" is never read, and a
# cluster-autoscaler-status ConfigMap in a tenant namespace is not printed.
grep -q 'logs -n tea-c0ffee00000000000002' "$KUBECTL_LOG" && fail "read the logs of a tenant pod named *autoscaler*"
ok "only platform autoscaler pods are read"

# The key is the cluster's: the same run with another kube-system UID (or
# PUBLIC_LOG_KEY) gives other hashes, so the public cannot reproduce them.
PUBLIC_LOG_KEY=another-cluster bash scripts/cluster-diagnose.sh "$sid" --public-log >"$work/other.out" 2>/dev/null || true
ws_tag="$(grep -oE 'ws#[0-9a-f]{6}' "$work/public.out" | head -1)"
other_tag="$(grep -oE 'ws#[0-9a-f]{6}' "$work/other.out" | head -1)"
[ -n "$ws_tag" ] && [ -n "$other_tag" ] && [ "$ws_tag" != "$other_tag" ] || fail "hashes do not depend on the key ($ws_tag vs $other_tag)"
ok "hashes are keyed"

# An API server that does not answer is one line, not a dozen timeouts.
: >"$work/down.log"
if STUB_UNREACHABLE=1 KUBECTL_LOG="$work/down.log" bash scripts/cluster-diagnose.sh "$sid" --public-log >"$work/down.out" 2>&1; then
  fail "--public-log succeeded against an unreachable API server"
fi
grep -q '^the API server did not answer' "$work/down.out" && [ "$(wc -l <"$work/down.out")" -eq 1 ] \
  || fail "an unreachable API server did not print exactly one line: $(cat "$work/down.out")"
[ "$(wc -l <"$work/down.log")" -eq 1 ] || fail "read on after the API server did not answer: $(tr '\n' ';' <"$work/down.log")"
ok "an unreachable API server stops the public run with one line"

# --- local, full detail ----------------------------------------------------------
if bash scripts/cluster-diagnose.sh "$sid" >"$work/full.out" 2>"$work/full.err"; then
  ok "local run exits 0"
else
  fail "local run failed: $(cat "$work/full.err")"
fi
for want in tea-c0ffee00000000000001-secret-shop-5d8f7c9b4-x2x9z bex-tenant-0-abcde-fghij 203.0.113.17 \
  'Liveness probe failed' hello-private-7c9d-abc12 'cpu                3967m'; do
  grep -qF -- "$want" "$work/full.out" || fail "local run lost '$want'"
done
ok "a local run prints the full detail"

# --- refusals ----------------------------------------------------------------------
for bad in srv-short "srv-c0ffee0000000000000a;id" tea-c0ffee00000000000001 ""; do
  if bash scripts/cluster-diagnose.sh "$bad" --public-log >"$work/bad.out" 2>&1; then
    fail "accepted service id '$bad'"
  elif [ -n "$bad" ] && grep -qF -- "$bad" "$work/bad.out"; then
    fail "echoed the refused id '$bad'"
  fi
done
bash scripts/cluster-diagnose.sh "$sid" --verbose >/dev/null 2>&1 && fail "accepted an unknown option"
ok "a malformed service id or option is refused without echoing it"

# --- the redactor on its own -----------------------------------------------------
redact() { printf '%s\n' "$1" | PUBLIC_LOG_KEY=k python3 scripts/lib/public_log.py; }
expect() { # input, want (exact)
  local got; got="$(redact "$1")"
  [ "$got" = "$2" ] || fail "redact '$1' -> '$got', want '$2'"
}
ws="$(redact tea-c0ffee00000000000001)"
[[ "$ws" =~ ^ws#[0-9a-f]{6}$ ]] || fail "a workspace id is not ws#<6 hex>: $ws"
expect tea-c0ffee00000000000001 "$ws"
expect "ns tea-c0ffee00000000000001-sandbox" "ns $ws-sandbox"
expect "$(printf 'pvc disk-tea-c0ffee00000000000001-secret-shop bound')" \
  "pvc disk-$ws~$(redact tea-c0ffee00000000000001-secret-shop | cut -d'~' -f2) bound"
case "$(redact 'bld-tea-c0ffee00000000000001-secret-shop-gen-3-abcde')" in
  "bld-$ws~"??????) ;; *) fail "a build pod name keeps more than its prefix" ;;
esac
[ "$(redact tea-c0ffee00000000000001-secret-shop-a)" != "$(redact tea-c0ffee00000000000001-secret-shop-b)" ] \
  || fail "two pods of one workspace share a hash"
case "$(redact 'srv-c0ffee0000000000000a dpg-c0ffee0000000000000b red-c0ffee0000000000000c')" in
  "srv#"??????" pg#"??????" kv#"??????) ;; *) fail "service, Postgres and KeyValue ids are not labelled" ;;
esac
case "$(redact 'node/bex-tenant-burst-qwert-yuiop condition met; etcd-bex-control-plane-kkkkk')" in
  "node/node#"??????" condition met; etcd-node#"??????) ;; *) fail "node names are not hashed: $(redact 'node/bex-tenant-burst-qwert-yuiop condition met; etcd-bex-control-plane-kkkkk')" ;;
esac
expect "template bex-tenant-0-k134-cpx32 and pool bex-tenant-0" "template bex-tenant-0-k134-cpx32 and pool bex-tenant-0"
case "$(redact 'via CP 203.0.113.9:6443, 2001:db8::17 and fe80::1%eth0.')" in
  "via CP ip#"??????":6443, ip#"??????" and ip#"??????"%eth0.") ;; *) fail "addresses are not hashed: $(redact 'via CP 203.0.113.9:6443, 2001:db8::17 and fe80::1%eth0.')" ;;
esac
expect "127.0.0.1 ::1 0.0.0.0 at 07:13:20 v1.34.9 6.8.0-149" "127.0.0.1 ::1 0.0.0.0 at 07:13:20 v1.34.9 6.8.0-149"
case "$(redact 'https://secret-shop.onbex.co and https://api.bex.co')" in
  "https://host#"??????".onbex.co and https://api.bex.co") ;; *) fail "a tenant hostname is not hashed" ;;
esac
case "$(redact 'providerID hcloud://123456789')" in "providerID hcloud://server#"??????) ;; *) fail "a Hetzner id is not hashed" ;; esac
expect "::add-mask::tea-c0ffee00000000000001" "::add-mask::tea-c0ffee00000000000001"
[ "$(printf 'tea-c0ffee00000000000001\n' | PUBLIC_LOG_KEY=k2 python3 scripts/lib/public_log.py)" != "$ws" ] \
  || fail "the hash does not depend on PUBLIC_LOG_KEY"
ok "the redactor hashes ids, names built on them, nodes, addresses, tenant hosts and provider ids, and nothing else"

# --- mask-inputs -------------------------------------------------------------------
printf '{"inputs": {"service_id": "srv-c0ffee0000000000000a", "namespace": "tea-c0ffee00000000000001", "confirm": "", "mode": "diagnose"}}' >"$work/event.json"
got="$(GITHUB_EVENT_PATH="$work/event.json" python3 scripts/lib/public_log.py mask-inputs service_id namespace confirm)"
want="$(printf '::add-mask::srv-c0ffee0000000000000a\n::add-mask::tea-c0ffee00000000000001')"
[ "$got" = "$want" ] || fail "mask-inputs printed '$got'"
env -u GITHUB_EVENT_PATH python3 scripts/lib/public_log.py mask-inputs service_id >/dev/null 2>&1 \
  && fail "mask-inputs succeeded outside GitHub Actions"
ok "mask-inputs masks each set dispatch input, and refuses to run without an event"

if [ "$fails" -eq 0 ]; then
  echo "PASS: cluster-diagnose --public-log keeps tenant data out of a public log; the local run keeps full detail"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
