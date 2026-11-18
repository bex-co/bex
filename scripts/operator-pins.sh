#!/usr/bin/env bash
# The bex operator's three GitOps image pins in deploy/gitops/base/bex.yaml
# (w6/m470, docs/ADR021-keyvalue-management.md § 10), for deploy.yml.
#
#   operator-pins.sh write <bex.yaml> <image> <digest>
#       Pins controller= to <digest>, as every deploy did. Each SNI proxy pin
#       (kv-sni-proxy=, pg-sni-proxy=) stays where it is while $PROXY_PIN
#       (a built lego/operator/cmd/proxy-pin) finds that proxy's binary
#       byte-identical in <digest> for linux/amd64, production's platform, so
#       a deploy that leaves the proxy's code alone rolls none of its pods and
#       closes none of its external connections. Any doubt moves the pin to
#       <digest>, which every deploy did before: $PROXY_PIN unset, failing, or
#       answering anything but one of the two digests. Only a pin line that is
#       missing or repeated fails, since the write-back would target the wrong
#       shape. Appends KV_SNI_PROXY_IMAGE= and PG_SNI_PROXY_IMAGE= (image@digest)
#       to $GITHUB_ENV when it is set.
#
#   operator-pins.sh wait
#       Waits until each SNI proxy DaemonSet's template carries the image the
#       write-back pinned for it ($KV_SNI_PROXY_IMAGE, $PG_SNI_PROXY_IMAGE),
#       then for its rollout. Calling `rollout status` alone is not enough: an
#       old, already-Ready DaemonSet returns at once, before Argo has synced
#       the write-back. A held image runs the same bytes, so it is exactly as
#       PROXY-protocol capable as the new one (infra.yml waits for this).
#       SNI_PROXY_WAIT_SECONDS (default 600) bounds each image wait.
set -euo pipefail

proxies=(kv-sni-proxy pg-sni-proxy)

die() {
  echo "operator-pins: $*" >&2
  exit 1
}

is_digest() { [[ "$1" =~ ^sha256:[0-9a-f]{64}$ ]]; }

# KV_SNI_PROXY_IMAGE for kv-sni-proxy.
image_var() { printf '%s_IMAGE' "$(printf '%s' "$1" | tr 'a-z-' 'A-Z_')"; }

# Rewrite <file> through sed -E <expression>, portably (BSD and GNU sed differ
# on -i) and keeping the file's mode.
rewrite() {
  local tmp
  tmp="$(mktemp)"
  sed -E "$2" "$1" >"$tmp"
  cat "$tmp" >"$1"
  rm -f "$tmp"
}

# The lines `- <name>=<image>@sha256:<digest>` in <file>.
pin_lines() {
  grep -E "^[[:space:]]+- $2=${3//./\\.}@sha256:[0-9a-f]{64}\$" "$1" || true
}

write() {
  local file="$1" image="$2" digest="$3"
  [ -f "$file" ] || die "no $file"
  is_digest "$digest" || die "build emitted an invalid image digest"
  local quoted="${image//./\\.}"
  # The bootstrap value may still be a tag (w1/m7 t002), as before.
  rewrite "$file" "s#^([[:space:]]+- controller=${quoted})(:[^@[:space:]]+|@sha256:[0-9a-f]+)\$#\\1@${digest}#"
  [ "$(pin_lines "$file" controller "$image" | grep -c .)" -eq 1 ] &&
    grep -qF -- "- controller=${image}@${digest}" "$file" ||
    die "expected exactly one controller=${image}@${digest} pin in $file"

  local proxy lines current answer
  for proxy in "${proxies[@]}"; do
    lines="$(pin_lines "$file" "$proxy" "$image")"
    [ "$(printf '%s' "$lines" | grep -c .)" -eq 1 ] ||
      die "expected exactly one ${proxy}=${image}@sha256: pin in $file"
    current="${lines##*@}"
    answer=""
    if [ "$current" = "$digest" ]; then
      echo "${proxy}: already pinned to ${digest}"
    elif [ -z "${PROXY_PIN:-}" ]; then
      echo "${proxy}: moved to ${digest}: no proxy-pin command to compare /${proxy} with"
    else
      # proxy-pin prints its own kept/moved reason on stderr, into the log.
      if ! answer="$("$PROXY_PIN" -image "$image" -current "$current" -candidate "$digest" \
        -file "/$proxy" -platform linux/amd64)"; then
        echo "${proxy}: moved to ${digest}: proxy-pin failed"
        answer=""
      elif [ "$answer" = "$current" ]; then
        echo "${proxy}: kept ${current}: /${proxy} is byte-identical in ${digest}"
      elif [ "$answer" = "$digest" ]; then
        echo "${proxy}: moved to ${digest}"
      else
        echo "${proxy}: moved to ${digest}: proxy-pin answered '${answer}', neither digest"
      fi
    fi
    if [ "$answer" != "$current" ]; then
      answer="$digest"
    fi
    rewrite "$file" "s#^([[:space:]]+- ${proxy}=${quoted})@sha256:[0-9a-f]{64}\$#\\1@${answer}#"
    grep -qF -- "- ${proxy}=${image}@${answer}" "$file" ||
      die "the ${proxy} pin did not land in $file"
    if [ -n "${GITHUB_ENV:-}" ]; then
      echo "$(image_var "$proxy")=${image}@${answer}" >>"$GITHUB_ENV"
    fi
  done
}

wait_for_proxies() {
  local wait_seconds="${SNI_PROXY_WAIT_SECONDS:-600}"
  local proxy daemonset var expected actual deadline
  for proxy in pg-sni-proxy kv-sni-proxy; do
    daemonset="bex-${proxy}"
    var="$(image_var "$proxy")"
    expected="${!var:-}"
    [ -n "$expected" ] || die "no image was pinned for ${daemonset} (\$${var} is empty): did the write-back run?"
    deadline=$((SECONDS + wait_seconds))
    actual=""
    while :; do
      actual="$(kubectl -n bex-system get daemonset "${daemonset}" -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || true)"
      [ "${actual}" = "${expected}" ] && break
      ((SECONDS < deadline)) || break
      sleep "${SNI_PROXY_POLL_SECONDS:-5}"
    done
    if [ "${actual}" != "${expected}" ]; then
      echo "${daemonset} image is ${actual:-missing}, want ${expected}" >&2
      exit 1
    fi
    kubectl -n bex-system rollout status "daemonset/${daemonset}" --timeout=300s
  done
}

case "${1:-}" in
  write)
    [ "$#" -eq 4 ] || die "usage: operator-pins.sh write <bex.yaml> <image> <digest>"
    write "$2" "$3" "$4"
    ;;
  wait)
    [ "$#" -eq 1 ] || die "usage: operator-pins.sh wait"
    wait_for_proxies
    ;;
  *)
    die "usage: operator-pins.sh write <bex.yaml> <image> <digest> | wait"
    ;;
esac
