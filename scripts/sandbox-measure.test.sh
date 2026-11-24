#!/usr/bin/env bash
# Self-test for scripts/lib/sandbox-measure.sh's pod CPU read (w22/024).
# sandbox-measure run 45146685744 (leg `usage`) stopped before its first check:
# it read the busy pod's counter from the node's cAdvisor half a second after
# the sandbox reported `running`, the node had no row for the pod yet, and the
# empty pipeline ended the leg. A stub kubectl stands in for the node and
# serves the pod's row only from a chosen read on.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

mkdir -p "$tmp/bin"
cat >"$tmp/bin/kubectl" <<'STUB'
#!/usr/bin/env bash
# Stub kubectl: `get --raw …/metrics/cadvisor` answers another pod's rows, and
# this pod's from read number $ROW_FROM on. Reads are counted in $STATE/reads.
case " $* " in
  *"/proxy/metrics/cadvisor "*)
    n=$(($(cat "$STATE/reads" 2>/dev/null || echo 0) + 1))
    echo "$n" >"$STATE/reads"
    echo 'container_cpu_usage_seconds_total{container="",id="/kubepods.slice/other",name="",namespace="ws-sandbox",pod="other-0"} 7.5 1'
    echo 'container_spec_cpu_quota{container="",id="/kubepods.slice/new",name="",namespace="ws-sandbox",pod="new-0"} 100000 1'
    if [ "$n" -ge "$ROW_FROM" ]; then
      echo 'container_cpu_usage_seconds_total{container="",id="/kubepods.slice/new/pause",name="k8s_POD_new-0",namespace="ws-sandbox",pod="new-0"} 0.01 1'
      echo 'container_cpu_usage_seconds_total{container="",id="/kubepods.slice/new",name="",namespace="ws-sandbox",pod="new-0"} 0.947664 1'
    fi
    ;;
  *) echo "stub kubectl: unexpected call: $*" >&2; exit 1 ;;
esac
STUB
chmod +x "$tmp/bin/kubectl"

# run <row from> <snippet>: the snippet's output and exit status, with the
# library sourced as a leg sources it.
run() {
  local state="$tmp/state-$1-$RANDOM"
  mkdir -p "$state"
  STATE="$state" ROW_FROM="$1" PATH="$tmp/bin:$PATH" KUBECONFIG=/dev/null \
    BEX_API_URL=http://127.0.0.1:9 BEX_WORKSPACE_A=ws BEX_WS_A_OWNER_TOKEN=t BEX_MEASURE_CADVISOR_PAUSE=0 \
    bash -c ". '$here/lib/sandbox-measure.sh'; $2" 2>"$state/stderr"
  echo "status=$? reads=$(cat "$state/reads" 2>/dev/null || echo 0)"
}
expect() {
  if [ "$2" = "$3" ]; then echo "ok: $1"; else
    echo "FAIL: $1"
    echo "  want: $3"
    echo "  got:  $2"
    fails=$((fails + 1))
  fi
}

expect "a pod whose row is there is read once" \
  "$(run 1 'echo "c0=$(pod_cpu_new new-0 node-a)"' | tr '\n' ' ')" "c0=0.947664 status=0 reads=1 "
expect "a new pod's row that appears on the third read is waited for (run 45146685744 stopped on the first)" \
  "$(run 3 'echo "c0=$(pod_cpu_new new-0 node-a)"' | tr '\n' ' ')" "c0=0.947664 status=0 reads=3 "
expect "a row that never appears reads empty after twelve tries, and the leg goes on to fail by name" \
  "$(run 99 'c0="$(pod_cpu_new new-0 node-a)"; [ -n "$c0" ] || fail "no pod-level CPU row"' 2>&1 | tr '\n' ' ')" "status=1 reads=12 "
expect "pod_cpu reads the pod cgroup's row, not the pause container's or another pod's" \
  "$(run 1 'echo "c=$(pod_cpu new-0 node-a) other=$(pod_cpu other-0 node-a)"' | tr '\n' ' ')" "c=0.947664 other=7.5 status=0 reads=2 "

# The cpu leg's runc reference pod (w22/019). Run 43696023922's pod named no
# securityContext and PodSecurity `restricted` refused it in opensandbox-system.
expect "the runc reference pod carries what PodSecurity restricted requires, the node's default runtime and the sandbox's CPU limit" \
  "$(run 1 'runc_reference_pod ref-1 node-a img:1 500m | jq -c "{
    restricted: (.spec.securityContext.runAsNonRoot == true and .spec.securityContext.runAsUser > 0
      and .spec.securityContext.seccompProfile.type == \"RuntimeDefault\"
      and all(.spec.containers[]; .securityContext.allowPrivilegeEscalation == false
        and .securityContext.capabilities.drop == [\"ALL\"] and (.securityContext.privileged // false) == false)),
    runtimeClass: (.spec.runtimeClassName // null), node: .spec.nodeName, image: .spec.containers[0].image,
    cpu: [.spec.containers[0].resources.requests.cpu, .spec.containers[0].resources.limits.cpu],
    token: .spec.automountServiceAccountToken}"' | tr '\n' ' ')" \
  '{"restricted":true,"runtimeClass":null,"node":"node-a","image":"img:1","cpu":["500m","500m"],"token":false} status=0 reads=0 '

# The loads' own reports, as a 6 s busy loop and a 4 s dd printed them in
# bash:5.3 (BusyBox dd) and debian:12-slim (GNU dd), 2026-11-24.
cat >"$tmp/busy.out" <<'OUT'
beat
beat
beat
loops 1636068
0m5.705s 0m0.014s
0m0.000s 0m0.000s
OUT
cat >"$tmp/dd-busybox.out" <<'OUT'
beat
beat
12059252+0 records in
12059251+0 records out
12059251 bytes (11.5MB) copied, 4.002210 seconds, 2.9MB/s
0m0.009s 0m0.005s
0m1.124s 0m3.376s
OUT
cat >"$tmp/dd-gnu.out" <<'OUT'
beat
beat
12082652+0 records in
12082652+0 records out
12082652 bytes (12 MB, 12 MiB) copied, 4.00194 s, 3.0 MB/s
0m0.006s 0m0.007s
0m1.139s 0m3.362s
OUT
printf 'beat\nbeat\n0m0.009s 0m0.005s\n0m1.1s 0m3.3s\n' >"$tmp/dd-silent.out"
expect "the busy loop's work is its loop count, and its CPU the shell's own times line" \
  "$(run 1 "echo \$(load_work busy <'$tmp/busy.out') \$(load_own_cpu busy <'$tmp/busy.out')" | tr '\n' ' ')" "1636068 5.719 status=0 reads=0 "
expect "dd's work is the byte count BusyBox prints on SIGUSR1, and its CPU the children's times line" \
  "$(run 1 "echo \$(load_work syscall <'$tmp/dd-busybox.out') \$(load_own_cpu syscall <'$tmp/dd-busybox.out')" | tr '\n' ' ')" "12059251 4.5 status=0 reads=0 "
expect "dd's work is the byte count GNU dd prints on SIGUSR1" \
  "$(run 1 "echo \$(load_work syscall <'$tmp/dd-gnu.out') \$(load_own_cpu syscall <'$tmp/dd-gnu.out')" | tr '\n' ' ')" "12082652 4.501 status=0 reads=0 "
expect "a dd that printed no count reports no work, so the leg fails by name instead of pricing zero bytes" \
  "$(run 1 "echo \"[\$(load_work syscall <'$tmp/dd-silent.out')]\"" | tr '\n' ' ')" "[] status=0 reads=0 "

# The rate leg's execs (w22/m623 t005). Waves of 20 that each waited for their
# slowest exec sent 600 in 88 s, every one answered (run 45408380111). Call 1
# here ends only once call 5 has started, which waves of 2 never allow.
cat >"$tmp/flight.sh" <<'FLIGHT'
flight_call() {
  mkdir "$FLIGHT/run-$1"
  [ "$(find "$FLIGHT" -maxdepth 1 -name 'run-*' | wc -l)" -le 2 ] || touch "$FLIGHT/over"
  touch "$FLIGHT/started-$1"
  if [ "$1" = 1 ]; then
    for _ in $(seq 1 50); do [ -e "$FLIGHT/started-5" ] && break; sleep 0.1; done
    if [ -e "$FLIGHT/started-5" ]; then touch "$FLIGHT/one-saw-five"; fi
  fi
  rmdir "$FLIGHT/run-$1"
}
flight_report() {
  echo "calls=$(find "$FLIGHT" -name 'started-*' | wc -l | tr -d ' ')" \
    "one-saw-five=$([ -e "$FLIGHT/one-saw-five" ] && echo yes || echo no)" \
    "over-two=$([ -e "$FLIGHT/over" ] && echo yes || echo no)"
}
FLIGHT
expect "in_flight starts the next call when one of the running ends, never more than its limit at once" \
  "$(run 1 "export FLIGHT='$tmp/flight'; mkdir -p \"\$FLIGHT\"; . '$tmp/flight.sh'; in_flight 5 2 flight_call; flight_report" | tr '\n' ' ')" \
  "calls=5 one-saw-five=yes over-two=no status=0 reads=0 "

[ "$fails" = 0 ] || { echo "$fails case(s) failed"; exit 1; }
echo "all cases passed"
