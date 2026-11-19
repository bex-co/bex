#!/usr/bin/env bash
# Unit test for scripts/operator-pins.sh (w6/m470): the write-back keeps an SNI
# proxy pin only when proxy-pin answers that its binary is byte-identical,
# moves it on every other answer, and the wait expects each DaemonSet's own
# pinned image. proxy-pin and kubectl are shell fakes; nothing leaves the host.
set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd)/operator-pins.sh"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0

IMAGE=ghcr.io/bex-co/bex-operator
OLD="sha256:$(printf 'a%.0s' {1..64})"
NEW="sha256:$(printf 'b%.0s' {1..64})"

fail() {
  echo "FAIL: $*" >&2
  fails=$((fails + 1))
}

# bex.yaml as deploy/gitops/base/bex.yaml carries it.
manifest() { # <controller ref> <kv digest> <pg digest>
  cat <<EOF
spec:
  source:
    kustomize:
      images:
        - controller=${IMAGE}$1
        - kv-sni-proxy=${IMAGE}@$2
        - pg-sni-proxy=${IMAGE}@$3
  destination:
    namespace: bex-system
EOF
}

# A fake proxy-pin: records its arguments, then answers per -file from
# $FAKE_ANSWER_<KV|PG> (a digest, "current", "candidate", or "fail").
cat >"$tmp/proxy-pin" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_CALLS"
while [ "$#" -gt 0 ]; do
  case "$1" in
    -current) current="$2" ;;
    -candidate) candidate="$2" ;;
    -file) file="$2" ;;
  esac
  shift 2
done
case "$file" in
  /kv-sni-proxy) answer="$FAKE_ANSWER_KV" ;;
  /pg-sni-proxy) answer="$FAKE_ANSWER_PG" ;;
esac
case "$answer" in
  current) echo "kept: $file unchanged" >&2; echo "$current" ;;
  candidate) echo "moved: $file changed" >&2; echo "$candidate" ;;
  fail) echo "proxy-pin: crashed" >&2; exit 1 ;;
  *) echo "$answer" ;;
esac
EOF
chmod +x "$tmp/proxy-pin"

# write_case <label> <kv answer> <pg answer> <want kv digest> <want pg digest> [proxy-pin path]
# Runs the write-back on a fresh bex.yaml pinned to OLD and checks the diff:
# controller always moves; each SNI pin lands where it should, and nothing else.
write_case() {
  local label="$1" kv="$2" pg="$3" want_kv="$4" want_pg="$5" pin="${6-$tmp/proxy-pin}"
  manifest "@$OLD" "$OLD" "$OLD" >"$tmp/bex.yaml"
  manifest "@$NEW" "$want_kv" "$want_pg" >"$tmp/want.yaml"
  : >"$tmp/env" && : >"$tmp/calls"
  if ! FAKE_ANSWER_KV="$kv" FAKE_ANSWER_PG="$pg" FAKE_CALLS="$tmp/calls" PROXY_PIN="$pin" GITHUB_ENV="$tmp/env" \
    "$SCRIPT" write "$tmp/bex.yaml" "$IMAGE" "$NEW" >"$tmp/out" 2>&1; then
    fail "$label: the write-back failed: $(cat "$tmp/out")"
    return
  fi
  if ! diff -u "$tmp/want.yaml" "$tmp/bex.yaml" >"$tmp/diff"; then
    fail "$label: bex.yaml is not as expected:"
    cat "$tmp/diff" >&2
    return
  fi
  local want_env="KV_SNI_PROXY_IMAGE=${IMAGE}@${want_kv}
PG_SNI_PROXY_IMAGE=${IMAGE}@${want_pg}"
  if [ "$(cat "$tmp/env")" != "$want_env" ]; then
    fail "$label: GITHUB_ENV is '$(cat "$tmp/env")', want '$want_env'"
    return
  fi
  echo "ok: $label"
}

write_case "both binaries identical: only controller= moves" current current "$OLD" "$OLD"
write_case "both binaries changed: all three move" candidate candidate "$NEW" "$NEW"
write_case "only kv-sni-proxy identical: only its pin stays" current candidate "$OLD" "$NEW"
write_case "only pg-sni-proxy identical: only its pin stays" candidate current "$NEW" "$OLD"
write_case "proxy-pin fails: all three move" fail fail "$NEW" "$NEW"
write_case "proxy-pin answers a third digest: all three move" \
  "sha256:$(printf 'c%.0s' {1..64})" "sha256:$(printf 'c%.0s' {1..64})" "$NEW" "$NEW"
write_case "proxy-pin answers garbage: all three move" "not a digest" "" "$NEW" "$NEW"
write_case "no proxy-pin (it did not build): all three move" current current "$NEW" "$NEW" ""
write_case "a proxy-pin path that does not exist: all three move" current current "$NEW" "$NEW" "$tmp/missing"

# proxy-pin compares production's platform for each proxy's own file, the
# current pin against the new digest.
manifest "@$OLD" "$OLD" "$OLD" >"$tmp/bex.yaml" && : >"$tmp/calls"
FAKE_ANSWER_KV=current FAKE_ANSWER_PG=current FAKE_CALLS="$tmp/calls" PROXY_PIN="$tmp/proxy-pin" \
  "$SCRIPT" write "$tmp/bex.yaml" "$IMAGE" "$NEW" >/dev/null 2>&1
want_calls="-image $IMAGE -current $OLD -candidate $NEW -file /kv-sni-proxy -platform linux/amd64
-image $IMAGE -current $OLD -candidate $NEW -file /pg-sni-proxy -platform linux/amd64"
if [ "$(cat "$tmp/calls")" = "$want_calls" ]; then
  echo "ok: proxy-pin compares each proxy's file for linux/amd64"
else
  fail "proxy-pin was called as '$(cat "$tmp/calls")', want '$want_calls'"
fi

# A re-run whose pins are already the new digest reads nothing.
manifest "@$NEW" "$NEW" "$NEW" >"$tmp/bex.yaml" && cp "$tmp/bex.yaml" "$tmp/want.yaml" && : >"$tmp/calls"
if FAKE_CALLS="$tmp/calls" PROXY_PIN="$tmp/proxy-pin" "$SCRIPT" write "$tmp/bex.yaml" "$IMAGE" "$NEW" >/dev/null 2>&1 &&
  diff -q "$tmp/want.yaml" "$tmp/bex.yaml" >/dev/null && [ ! -s "$tmp/calls" ]; then
  echo "ok: pins already on the new digest stay, with no comparison"
else
  fail "a re-run on pinned digests changed bex.yaml or called proxy-pin ($(cat "$tmp/calls"))"
fi

# The bootstrap controller tag (w1/m7 t002) still pins.
manifest ":latest" "$OLD" "$OLD" >"$tmp/bex.yaml"
manifest "@$NEW" "$OLD" "$OLD" >"$tmp/want.yaml"
if FAKE_ANSWER_KV=current FAKE_ANSWER_PG=current FAKE_CALLS="$tmp/calls" PROXY_PIN="$tmp/proxy-pin" \
  "$SCRIPT" write "$tmp/bex.yaml" "$IMAGE" "$NEW" >/dev/null 2>&1 && diff -q "$tmp/want.yaml" "$tmp/bex.yaml" >/dev/null; then
  echo "ok: a bootstrap controller tag pins to the digest"
else
  fail "a bootstrap controller tag did not pin"
fi

# Shapes the write-back cannot pin fail the step instead of pinning the wrong line.
refuse() { # <label> <bex.yaml content> [digest]
  printf '%s\n' "$2" >"$tmp/bex.yaml"
  if FAKE_ANSWER_KV=current FAKE_ANSWER_PG=current FAKE_CALLS="$tmp/calls" PROXY_PIN="$tmp/proxy-pin" \
    "$SCRIPT" write "$tmp/bex.yaml" "$IMAGE" "${3:-$NEW}" >"$tmp/out" 2>&1; then
    fail "$1: the write-back succeeded"
  else
    echo "ok: $1 fails"
  fi
}
refuse "a missing kv-sni-proxy pin" "$(manifest "@$OLD" "$OLD" "$OLD" | grep -v kv-sni-proxy)"
refuse "a repeated pg-sni-proxy pin" "$(manifest "@$OLD" "$OLD" "$OLD"; echo "        - pg-sni-proxy=${IMAGE}@${OLD}")"
refuse "an SNI pin held by tag" "$(manifest "@$OLD" "$OLD" "$OLD" | sed "s#kv-sni-proxy=${IMAGE}@${OLD}#kv-sni-proxy=${IMAGE}:latest#")"
refuse "an SNI pin on another image" "$(manifest "@$OLD" "$OLD" "$OLD" | sed "s#pg-sni-proxy=${IMAGE}#pg-sni-proxy=ghcr.io/bex-co/other#")"
refuse "a missing controller pin" "$(manifest "@$OLD" "$OLD" "$OLD" | grep -v controller=)"
refuse "an invalid build digest" "$(manifest "@$OLD" "$OLD" "$OLD")" "sha256:abc"

# --- wait --------------------------------------------------------------------
# A fake kubectl over $FAKE_K8S:
#   - get daemonset <name> -o jsonpath=…: the template image, <name>
#   - get daemonset <name> -o json: <name>.json
#   - get pods -l app.bex.co/component=<proxy> -o json: <proxy>.pods
#   - rollout status daemonset/<name>: what kubectl decides from <name>.json's
#     status, so a wait that falls back to it fails the cases it should
# Each file is replaced by its .next after .after reads, and every read is
# logged to $FAKE_K8S/reads.
mkdir -p "$tmp/bin" "$tmp/k8s"
cat >"$tmp/bin/kubectl" <<'FAKE'
#!/usr/bin/env bash
serve() {
  local file="$FAKE_K8S/$1" polls
  if [ -f "$file.next" ]; then
    polls=$(($(cat "$file.polls" 2>/dev/null || echo 0) + 1))
    echo "$polls" >"$file.polls"
    [ "$polls" -le "$(cat "$file.after")" ] || mv "$file.next" "$file"
  fi
  cat "$file" 2>/dev/null || exit 1
}
args="$*"
case "$args" in
  "-n bex-system get daemonset "*" -o jsonpath="*) serve "$5" ;;
  "-n bex-system get daemonset "*" -o json")
    echo "daemonset $5" >>"$FAKE_K8S/reads"
    serve "$5.json"
    ;;
  "-n bex-system get pods -l "*" -o json")
    echo "pods $6" >>"$FAKE_K8S/reads"
    serve "${6#*=}.pods"
    ;;
  "-n bex-system rollout status daemonset/"*)
    echo "rollout status $5" >>"$FAKE_K8S/reads"
    jq -e '.metadata.generation <= .status.observedGeneration
      and .status.updatedNumberScheduled >= .status.desiredNumberScheduled
      and .status.numberAvailable >= .status.desiredNumberScheduled' \
      "$FAKE_K8S/${5#daemonset/}.json" >/dev/null || { echo "error: timed out waiting for the condition" >&2; exit 1; }
    ;;
  *) echo "unexpected kubectl $args" >&2; exit 2 ;;
esac
FAKE
chmod +x "$tmp/bin/kubectl"

# ds_json <proxy> <generation> <observedGeneration> <template generation> <desired> <updated> <available>
# The DaemonSet's status is what the controller computes from each node's
# oldest pod, as on a real cluster.
ds_json() {
  jq -n --arg proxy "$1" --argjson gen "$2" --argjson observed "$3" --arg template "$4" \
    --argjson desired "$5" --argjson updated "$6" --argjson available "$7" '{
      metadata: {name: "bex-\($proxy)", uid: "uid-bex-\($proxy)", generation: $gen,
        annotations: {"deprecated.daemonset.template.generation": $template}},
      spec: {selector: {matchLabels: {"app.bex.co/component": $proxy}}},
      status: {observedGeneration: $observed, desiredNumberScheduled: $desired,
        updatedNumberScheduled: $updated, numberAvailable: $available}}'
}

# pod_json <proxy> <node> <template generation> <Ready: True|False> [terminating] [owner=<uid>] [name=<name>]
pod_json() {
  local proxy="$1" node="node-$2" template="$3" ready="$4" terminating=false owner="uid-bex-$1" name=""
  shift 4
  for flag in "$@"; do
    case "$flag" in
      terminating) terminating=true ;;
      owner=*) owner="${flag#owner=}" ;;
      name=*) name="${flag#name=}" ;;
    esac
  done
  jq -n --arg proxy "$proxy" --arg node "$node" --arg template "$template" --arg ready "$ready" \
    --argjson terminating "$terminating" --arg owner "$owner" --arg name "${name:-bex-$proxy-$node-g$template}" '{
      metadata: ({name: $name,
        labels: {"app.bex.co/component": $proxy, "pod-template-generation": $template},
        ownerReferences: [{apiVersion: "apps/v1", kind: "DaemonSet", name: "bex-\($proxy)", uid: $owner, controller: true}]}
        + (if $terminating then {deletionTimestamp: "2026-11-19T12:00:00Z"} else {} end)),
      spec: {nodeName: $node},
      status: {conditions: [{type: "Ready", status: $ready}]}}'
}

pods_json() { jq -s '{items: .}'; }

# wait_setup <kv pinned> <pg pinned> <kv running> <pg running> [kv after N polls]
# A fresh fake cluster: each DaemonSet's template image, and two nodes that
# each run one Ready pod of the current template.
wait_setup() {
  rm -rf "$tmp/k8s" && mkdir "$tmp/k8s"
  printf '%s' "$3" >"$tmp/k8s/bex-kv-sni-proxy"
  printf '%s' "$4" >"$tmp/k8s/bex-pg-sni-proxy"
  if [ -n "${5:-}" ]; then
    printf '%s' "$1" >"$tmp/k8s/bex-kv-sni-proxy.next"
    echo "$5" >"$tmp/k8s/bex-kv-sni-proxy.after"
  fi
  wait_pins=("$1" "$2")
  local proxy
  for proxy in kv-sni-proxy pg-sni-proxy; do
    ds_json "$proxy" 7 7 7 2 2 2 >"$tmp/k8s/bex-$proxy.json"
    { pod_json "$proxy" a 7 True && pod_json "$proxy" b 7 True; } | pods_json >"$tmp/k8s/$proxy.pods"
  done
}

# wait_run <label> <want rc>
wait_run() {
  local label="$1" want="$2" got=0
  PATH="$tmp/bin:$PATH" FAKE_K8S="$tmp/k8s" SNI_PROXY_WAIT_SECONDS=2 SNI_PROXY_ROLLOUT_SECONDS=2 \
    SNI_PROXY_POLL_SECONDS=0.1 KV_SNI_PROXY_IMAGE="${wait_pins[0]}" PG_SNI_PROXY_IMAGE="${wait_pins[1]}" \
    "$SCRIPT" wait >"$tmp/out" 2>&1 || got=$?
  if [ "$got" -ne "$want" ]; then
    fail "$label: exit $got, want $want: $(cat "$tmp/out")"
    return 1
  fi
  echo "ok: $label (exit $got)"
}

# wait_case <label> <want rc> <kv pinned> <pg pinned> <kv running> <pg running> [kv after N polls]
wait_case() {
  wait_setup "$3" "$4" "$5" "$6" "${7:-}"
  wait_run "$1" "$2"
}

held="${IMAGE}@${OLD}" moved="${IMAGE}@${NEW}"
if wait_case "a held DaemonSet already runs its pin" 0 "$held" "$moved" "$held" "$moved" &&
  [ "$(grep '^pods ' "$tmp/k8s/reads")" = "$(printf 'pods app.bex.co/component=pg-sni-proxy\npods app.bex.co/component=kv-sni-proxy')" ]; then
  echo "ok: each DaemonSet's pods are read once its pin is in the template"
else
  fail "the wait did not read both DaemonSets' pods: $(cat "$tmp/k8s/reads" 2>/dev/null)"
fi
wait_case "a moved DaemonSet reaches its pin after Argo syncs" 0 "$moved" "$moved" "$held" "$moved" 3 || true
wait_case "a held DaemonSet that Argo moved to the new digest times out" 1 "$held" "$held" "$moved" "$held" || true
if wait_case "a moved DaemonSet that never gets its pin times out" 1 "$moved" "$moved" "$held" "$moved"; then
  if grep -qxF "bex-kv-sni-proxy image is ${held}, want ${moved}" "$tmp/out"; then
    echo "ok: a timed-out wait names the DaemonSet, its image and the pin"
  else
    fail "a timed-out wait does not name the DaemonSet: $(cat "$tmp/out")"
  fi
fi
wait_case "a missing DaemonSet times out" 1 "$moved" "$moved" "" "$moved" || true
wait_case "no pin recorded for a DaemonSet fails at once" 1 "" "$moved" "$moved" "$moved" || true

# The pod wait (w6/m518): bex-kv-sni-proxy rolls from template generation 7 to
# 8 on nodes a and b; bex-pg-sni-proxy stays settled.
kv_state() { # <ds_json args after the proxy> -- <pod_json lines as "node gen ready [flags]">...
  local ds=() pod
  while [ "$1" != -- ]; do ds+=("$1") && shift; done
  shift
  ds_json kv-sni-proxy "${ds[@]}" >"$tmp/k8s/bex-kv-sni-proxy.json"
  for pod in "$@"; do
    # shellcheck disable=SC2086
    pod_json kv-sni-proxy $pod
  done | pods_json >"$tmp/k8s/kv-sni-proxy.pods"
}
pods_case() { # <label> <want rc>; the fake cluster is already staged
  wait_run "$1" "$2"
}

wait_setup "$moved" "$moved" "$moved" "$moved"
# Node a's old pod drains beside its Ready new pod; it is a's oldest pod, so
# the controller counts 1 of 2 updated and `rollout status` would wait it out.
kv_state 8 8 8 2 1 2 -- "a 7 True terminating" "a 8 True" "b 8 True"
pods_case "an old pod that drains beside its node's Ready new pod does not hold the wait" 0 || true

wait_setup "$moved" "$moved" "$moved" "$moved"
kv_state 8 8 8 2 1 2 -- "a 7 True" "a 8 False" "b 8 True"
if pods_case "a new pod that never becomes Ready fails at the bound" 1; then
  if grep -qF "bex-kv-sni-proxy: 1 of 2 nodes run a Ready pod of template generation 8; other pods: bex-kv-sni-proxy-node-a-g7 on node-a (template generation 7), bex-kv-sni-proxy-node-a-g8 on node-a (not Ready) after 2s" "$tmp/out"; then
    echo "ok: a timed-out pod wait names the DaemonSet, the count and each pod it did not count"
  else
    fail "a timed-out pod wait does not say what is missing: $(cat "$tmp/out")"
  fi
fi

wait_setup "$moved" "$moved" "$moved" "$moved"
# A spec change the controller has not observed: the pods still look complete.
kv_state 9 8 8 2 2 2 -- "a 8 True" "b 8 True"
if pods_case "a stale observedGeneration waits" 1; then
  grep -qF "bex-kv-sni-proxy: generation 9 is not observed yet (observedGeneration 8)" "$tmp/out" ||
    fail "a stale observedGeneration is not named: $(cat "$tmp/out")"
fi
ds_json kv-sni-proxy 9 9 8 2 2 2 >"$tmp/k8s/bex-kv-sni-proxy.json.next" && echo 1 >"$tmp/k8s/bex-kv-sni-proxy.json.after"
kv_state 9 8 8 2 2 2 -- "a 8 True" "b 8 True"
pods_case "a stale observedGeneration passes once the controller observes it" 0 || true

wait_setup "$moved" "$moved" "$moved" "$moved"
kv_state 8 8 8 2 1 2 -- "a 8 True" "b 7 True"
pods_case "a node that still runs only its old pod waits" 1 || true
{ pod_json kv-sni-proxy a 8 True && pod_json kv-sni-proxy b 7 True terminating && pod_json kv-sni-proxy b 8 True; } |
  pods_json >"$tmp/k8s/kv-sni-proxy.pods.next" && echo 1 >"$tmp/k8s/kv-sni-proxy.pods.after"
pods_case "that node passes once its new pod is Ready" 0 || true

wait_setup "$moved" "$moved" "$moved" "$moved"
# Node a's current pod is being deleted and its replacement is not up yet.
kv_state 8 8 8 2 2 2 -- "a 8 True terminating" "b 8 True"
pods_case "a node whose only current pod is terminating waits" 1 || true

wait_setup "$moved" "$moved" "$moved" "$moved"
kv_state 8 8 8 2 1 2 -- "a 8 True owner=uid-other" "b 8 True"
pods_case "a pod the DaemonSet does not own does not count" 1 || true

wait_setup "$moved" "$moved" "$moved" "$moved"
kv_state 8 8 8 2 1 2 -- "a 8 True name=first" "a 8 True name=second"
pods_case "two Ready pods on one node count as one node" 1 || true

if [ "$fails" -eq 0 ]; then
  echo "PASS: operator-pins.sh"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
