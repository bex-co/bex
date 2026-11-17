#!/usr/bin/env bash
# Tests for the production-pool guard and the dry-run of sandbox-docker-spike.sh
# (w22/m312 t006). No docker daemon, no cluster: kubectl, docker and hostname are
# stubs on PATH, and the script's --guard-only / --dry-run modes never start a
# sandbox.
#
#   bash scripts/sandbox-docker-spike.test.sh
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

SCRIPT="scripts/sandbox-docker-spike.sh"
STUBS="$(mktemp -d)"
trap 'rm -rf "$STUBS"' EXIT

cat >"$STUBS/kubectl" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *"config current-context"*) [ "${STUB_NO_CONTEXT:-0}" = 1 ] && exit 1; echo stub-context; exit 0 ;;
  *"get nodes"*) [ "${STUB_GET_FAIL:-0}" = 1 ] && { echo "forbidden" >&2; exit 1; }; printf '%s' "${STUB_POOL:-}"; exit 0 ;;
  *"get pod"*) printf '%s' "${STUB_POD_NODE:-}"; exit 0 ;;
esac
exit 1
EOF
cat >"$STUBS/docker" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  "context show") echo stub-docker-context ;;
  "context inspect"*) echo "${STUB_DOCKER_HOST:-unix:///var/run/docker.sock}" ;;
  *) echo "stub docker: unexpected call: $*" >&2; exit 99 ;;
esac
EOF
cat >"$STUBS/hostname" <<'EOF'
#!/usr/bin/env bash
echo "${STUB_HOSTNAME:-dev-laptop}"
EOF
chmod +x "$STUBS/kubectl" "$STUBS/docker" "$STUBS/hostname"

POOL=$'bex-sandbox-1 10.0.0.5 bex-sandbox-1\nbex-sandbox-2 10.0.0.6 bex-sandbox-2\n'
FAILURES=0

# expect <exit code> <output substring or ""> <description> [ENV=VALUE ...] -- <script args...>
expect() {
  local want="$1" needle="$2" desc="$3" out rc
  shift 3
  local envs=()
  while [ "$1" != -- ]; do envs+=("$1"); shift; done
  shift
  out="$(env PATH="$STUBS:$PATH" ${envs[@]+"${envs[@]}"} bash "$SCRIPT" "$@" 2>&1)"
  rc=$?
  if [ "$rc" -eq "$want" ] && { [ -z "$needle" ] || printf '%s' "$out" | grep -qF -- "$needle"; }; then
    echo "ok   $desc"
  else
    echo "FAIL $desc: exit $rc (want $want), needle '$needle'"
    printf '%s\n' "$out" | sed 's/^/     | /'
    FAILURES=$((FAILURES + 1))
  fi
}

expect 3 "production sandbox pool" "refuses when the context has a sandbox pool and no --target-node" \
  STUB_POOL="$POOL" -- --guard-only
expect 3 "bex-sandbox-1 is a node of the production sandbox pool" "refuses --target-node naming a pool node" \
  STUB_POOL="$POOL" -- --guard-only --target-node bex-sandbox-1
expect 0 "guard: passed" "passes for a --target-node outside the pool" \
  STUB_POOL="$POOL" -- --guard-only --target-node colima-gvisor-spike
expect 3 "10.0.0.5 is a node of the production sandbox pool" "refuses a docker endpoint at a pool node's address" \
  STUB_POOL="$POOL" STUB_DOCKER_HOST="ssh://root@10.0.0.5" -- --guard-only --target-node elsewhere
expect 3 "bex-sandbox-2 is a node of the production sandbox pool" "refuses when this machine's hostname is a pool node" \
  STUB_POOL="$POOL" STUB_HOSTNAME=bex-sandbox-2 -- --guard-only --target-node elsewhere
expect 3 "bex-sandbox-1 is a node of the production sandbox pool" "refuses a --k8s-pod scheduled on a pool node" \
  STUB_POOL="$POOL" STUB_POD_NODE=bex-sandbox-1 -- --guard-only --k8s-namespace ns --k8s-pod p
expect 0 "the production-pool guard is OFF" "--i-know-this-is-production overrides, loudly" \
  STUB_POOL="$POOL" -- --guard-only --i-know-this-is-production
expect 3 "cannot be listed" "refuses when a context exists but its nodes cannot be listed" \
  STUB_GET_FAIL=1 -- --guard-only
expect 0 "guard skipped" "--no-kubectl asserts there is no cluster" \
  STUB_GET_FAIL=1 -- --guard-only --no-kubectl
expect 0 "no current context" "passes with a note when kubectl has no context" \
  STUB_NO_CONTEXT=1 -- --guard-only
expect 0 "guard: passed" "passes when the cluster has no sandbox pool" \
  STUB_POOL="" -- --guard-only
expect 3 "production sandbox pool" "a real run (not just --guard-only) is refused before any docker call" \
  STUB_POOL="$POOL" -- --docker-versions 29

# --dry-run touches neither docker nor kubectl: the stubs fail loudly on any call.
out="$(env PATH="$STUBS:$PATH" STUB_POOL="$POOL" bash "$SCRIPT" --dry-run --docker-versions "29 28" 2>&1)"
rc=$?
count="$(printf '%s\n' "$out" | grep -c '^PLAN  ')"
if [ "$rc" -eq 0 ] && [ "$count" -eq 13 ] && ! printf '%s' "$out" | grep -q "stub docker"; then
  echo "ok   --dry-run prints all 13 functional checks and calls nothing"
else
  echo "FAIL --dry-run: exit $rc, $count PLAN lines"
  FAILURES=$((FAILURES + 1))
fi

# The isolation checks are opt-in: absent by default, listed with --isolation.
if printf '%s' "$out" | grep -q "PLAN  spoof_arp"; then
  echo "FAIL --dry-run listed isolation checks without --isolation"
  FAILURES=$((FAILURES + 1))
else
  echo "ok   isolation checks are off by default"
fi
out="$(env PATH="$STUBS:$PATH" bash "$SCRIPT" --dry-run --isolation --lab-no-cilium 2>&1)"
count="$(printf '%s\n' "$out" | grep -c '^PLAN  \(floor_direct_tcp\|raw_syn_denied\|spoof_src_ip\|spoof_arp\|spoof_ndp\|fqdn_sni_bypass\)')"
if [ "$count" -eq 6 ]; then
  echo "ok   --dry-run --isolation lists the 6 isolation checks"
else
  echo "FAIL --dry-run --isolation listed $count of 6 isolation checks"
  FAILURES=$((FAILURES + 1))
fi

# --isolation without a pod or --lab-no-cilium is a usage error, not a silent skip.
expect 2 "--isolation needs --k8s-namespace and --k8s-pod" "--isolation alone is a usage error" -- --isolation --dry-run

# With a pod but no Cilium policy in its namespace, isolation exits 4 with a message.
cat >"$STUBS/kubectl" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *"config current-context"*) echo stub-context ;;
  *"get nodes"*) exit 0 ;;
  *"ciliumnetworkpolicies"*) exit 0 ;;
  *) exit 1 ;;
esac
EOF
expect 4 "has no CiliumNetworkPolicy" "--isolation exits 4 when the namespace has no Cilium policy" \
  -- --isolation --k8s-namespace ns --k8s-pod p --target-node elsewhere

# --- a pod run through stubs: the pod-mode skip and the node observer ---------
# kubectl answers as a cluster with one Cilium policy and a pod whose every exec
# succeeds, except a spoofing probe: its script carries the MANUAL verdict text,
# and STUB_SEND says whether the runtime accepted the send (exit 2) or refused it.
cat >"$STUBS/kubectl" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *"config current-context"*) echo stub-context ;;
  *"get nodes"*) exit 0 ;;
  *"get pod"*) echo throwaway-node ;;
  *"ciliumnetworkpolicies"*) echo ciliumnetworkpolicy.cilium.io/sandbox-egress-stub ;;
  *" exec "*)
    case "$*" in
      *"MANUAL: the send was accepted"*)
        if [ "${STUB_SEND:-accepted}" = refused ]; then echo "SEND FAILED: [Errno 95] Not supported"; exit 0; fi
        echo "sent SYN src=10.255.255.254 dst=203.0.113.9:443"; echo "MANUAL: the send was accepted. Observer: ..."; exit 2 ;;
    esac
    exit 0 ;;
  *) exit 1 ;;
esac
EOF
# The observer stub is the node: it has tcpdump unless STUB_NO_TCPDUMP=1, and
# its capture is whatever STUB_CAPTURE holds.
cat >"$STUBS/observer" <<'EOF'
#!/usr/bin/env bash
case "$1" in
  *"command -v tcpdump"*) [ "${STUB_NO_TCPDUMP:-0}" = 1 ] && exit 1; exit 0 ;;
  *"cat /tmp/spike-obs.txt"*) printf '%s' "${STUB_CAPTURE:-}" ;;
esac
exit 0
EOF
chmod +x "$STUBS/observer"
POD_ARGS=(--k8s-namespace ns --k8s-pod p --target-node throwaway-node)
ISO_ARGS=("${POD_ARGS[@]}" --isolation --only none --allowed-fqdn a.example --denied-fqdn d.example)
DROPPED=$'19:19:43.1 lxcd55dad89ace8 In  ifindex 29 26:3e:51:dc:e4:a1 ethertype IPv4 (0x0800), length 60: 10.255.255.254.24912 > 203.0.113.9.443: Flags [S]\n'
FORWARDED="$DROPPED"$'19:19:43.2 eth0 Out ifindex 2 52:55:55:ce:08:30 ethertype IPv4 (0x0800), length 60: 10.255.255.254.24912 > 203.0.113.9.443: Flags [S]\n'
SECOND_DEVICE="$DROPPED"$'19:19:43.2 lxc28fa477c70f2 In  ifindex 20 c2:de:33:dc:5f:d0 ethertype IPv4 (0x0800), length 60: 10.255.255.254.24912 > 203.0.113.9.443: Flags [S]\n'

expect 0 "SKIP    published_port_from_outside" "a pod run skips the outer-daemon check instead of failing it" \
  -- "${POD_ARGS[@]}" --only published_port_from_outside
expect 0 "MANUAL  spoof_src_ip" "without --observer an accepted forged send stays MANUAL" \
  -- "${ISO_ARGS[@]}"
expect 0 "dropped where it entered the node" "--observer passes a forged packet seen only where it entered the node" \
  STUB_CAPTURE="$DROPPED" -- "${ISO_ARGS[@]}" --observer observer
expect 1 "FORGED PACKET FORWARDED" "--observer fails a forged packet that leaves an interface" \
  STUB_CAPTURE="$FORWARDED" -- "${ISO_ARGS[@]}" --observer observer
expect 1 "FORGED PACKET FORWARDED" "--observer fails a forged packet seen on a second interface" \
  STUB_CAPTURE="$SECOND_DEVICE" -- "${ISO_ARGS[@]}" --observer observer
expect 0 "the node saw no such packet" "--observer passes when the node never saw the packet" \
  STUB_CAPTURE="" -- "${ISO_ARGS[@]}" --observer observer
expect 0 "the runtime refused the send" "--observer passes a send the runtime refused, whatever the capture says" \
  STUB_SEND=refused STUB_CAPTURE="$FORWARDED" -- "${ISO_ARGS[@]}" --observer observer
expect 0 "MANUAL  spoof_arp" "an observer node without tcpdump leaves the check MANUAL, never PASS" \
  STUB_NO_TCPDUMP=1 STUB_CAPTURE="$DROPPED" -- "${ISO_ARGS[@]}" --observer observer

# The FQDN check runs after the raw-packet probes, which delete the SNAT rules:
# it must put them back and require its control, or "blocked" means nothing.
out="$(env PATH="$STUBS:$PATH" bash "$SCRIPT" --dry-run --isolation --lab-no-cilium --allowed-fqdn a.example --denied-fqdn d.example 2>&1)"
plan="$(printf '%s\n' "$out" | sed -n '/^PLAN  fqdn_sni_bypass/,$p')"
snat_line="$(printf '%s\n' "$plan" | grep -n -- '-j SNAT' | head -1 | cut -d: -f1)"
control_line="$(printf '%s\n' "$plan" | grep -n 'CONTROL FAILED' | head -1 | cut -d: -f1)"
denied_line="$(printf '%s\n' "$plan" | grep -n 'REACHED d.example' | head -1 | cut -d: -f1)"
if [ -n "$snat_line" ] && [ -n "$control_line" ] && [ -n "$denied_line" ] && [ "$snat_line" -lt "$control_line" ] && [ "$control_line" -lt "$denied_line" ]; then
  echo "ok   the FQDN check restores SNAT, then requires its control, before any denied name"
else
  echo "FAIL the FQDN check order: SNAT line '$snat_line', control line '$control_line', denied line '$denied_line'"
  FAILURES=$((FAILURES + 1))
fi

if [ "$FAILURES" -ne 0 ]; then
  echo "$FAILURES failed" >&2
  exit 1
fi
echo "all passed"
