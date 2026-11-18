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
# A fake kubectl: each DaemonSet's template image is the content of
# $FAKE_K8S/<name>, which <name>.next replaces after $FAKE_K8S/<name>.polls
# reads; rollout status is recorded.
mkdir -p "$tmp/bin" "$tmp/k8s"
cat >"$tmp/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
args="$*"
case "$args" in
  "-n bex-system get daemonset "*)
    name="$5"
    if [ -f "$FAKE_K8S/$name.next" ]; then
      polls=$(($(cat "$FAKE_K8S/$name.polls" 2>/dev/null || echo 0) + 1))
      echo "$polls" >"$FAKE_K8S/$name.polls"
      [ "$polls" -le "$(cat "$FAKE_K8S/$name.after")" ] || mv "$FAKE_K8S/$name.next" "$FAKE_K8S/$name"
    fi
    cat "$FAKE_K8S/$name" 2>/dev/null || exit 1
    ;;
  "-n bex-system rollout status "*)
    echo "$5" >>"$FAKE_K8S/rollouts"
    ;;
  *) echo "unexpected kubectl $args" >&2; exit 2 ;;
esac
EOF
chmod +x "$tmp/bin/kubectl"

# wait_case <label> <want rc> <kv pinned> <pg pinned> <kv running> <pg running> [kv after N polls]
wait_case() {
  local label="$1" want="$2" kv="$3" pg="$4" kv_running="$5" pg_running="$6" kv_next="${7:-}"
  rm -rf "$tmp/k8s" && mkdir "$tmp/k8s"
  printf '%s' "$kv_running" >"$tmp/k8s/bex-kv-sni-proxy"
  printf '%s' "$pg_running" >"$tmp/k8s/bex-pg-sni-proxy"
  if [ -n "$kv_next" ]; then
    printf '%s' "$kv" >"$tmp/k8s/bex-kv-sni-proxy.next"
    echo "$kv_next" >"$tmp/k8s/bex-kv-sni-proxy.after"
  fi
  local got=0
  PATH="$tmp/bin:$PATH" FAKE_K8S="$tmp/k8s" SNI_PROXY_WAIT_SECONDS=2 SNI_PROXY_POLL_SECONDS=0.1 \
    KV_SNI_PROXY_IMAGE="$kv" PG_SNI_PROXY_IMAGE="$pg" "$SCRIPT" wait >"$tmp/out" 2>&1 || got=$?
  if [ "$got" -ne "$want" ]; then
    fail "$label: exit $got, want $want: $(cat "$tmp/out")"
    return 1
  fi
  echo "ok: $label (exit $got)"
}

held="${IMAGE}@${OLD}" moved="${IMAGE}@${NEW}"
if wait_case "a held DaemonSet already runs its pin" 0 "$held" "$moved" "$held" "$moved" &&
  [ "$(cat "$tmp/k8s/rollouts")" = "$(printf 'daemonset/bex-pg-sni-proxy\ndaemonset/bex-kv-sni-proxy')" ]; then
  echo "ok: each DaemonSet's rollout is awaited once its pin is in the template"
else
  fail "the wait did not await both rollouts: $(cat "$tmp/k8s/rollouts" 2>/dev/null)"
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

if [ "$fails" -eq 0 ]; then
  echo "PASS: operator-pins.sh"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
