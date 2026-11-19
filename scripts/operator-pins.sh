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
#       then until every node it should run on runs a Ready pod of that
#       template that is not terminating. The image wait comes first because
#       an old, already-Ready DaemonSet looks rolled out before Argo has synced
#       the write-back. The pod wait reads the pods, not `rollout status`:
#       the DaemonSet's status counts each node's oldest pod, so it would wait
#       out every old pod's drain (w6/m518). A held image runs the same bytes,
#       so it is exactly as PROXY-protocol capable as the new one (infra.yml
#       waits for this). SNI_PROXY_WAIT_SECONDS (default 600) bounds each image
#       wait, SNI_PROXY_ROLLOUT_SECONDS (default 300) each pod wait.
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

# Reads a DaemonSet and the pods its selector matches, as two JSON documents on
# stdin, and prints "ready" once the DaemonSet controller has observed its
# generation and each node it should run on (desiredNumberScheduled) has a
# Ready pod of the current template that is not terminating. Otherwise it
# prints what is missing. Old pods that still drain do not count either way.
new_pods_verdict() {
  jq -rs '
    .[0] as $ds | .[1].items as $pods
    | ($ds.metadata.annotations["deprecated.daemonset.template.generation"] // "") as $template
    | ($ds.status.observedGeneration // 0) as $observed
    | ($ds.status.desiredNumberScheduled // 0) as $desired
    | [$pods[] | select(any(.metadata.ownerReferences[]?; .controller == true and .uid == $ds.metadata.uid))] as $owned
    | def counted:
        .metadata.deletionTimestamp == null
        and .metadata.labels["pod-template-generation"] == $template
        and any(.status.conditions[]?; .type == "Ready" and .status == "True");
      def why:
        if .metadata.deletionTimestamp != null then "terminating"
        elif .metadata.labels["pod-template-generation"] != $template then
          "template generation \(.metadata.labels["pod-template-generation"] // "none")"
        else "not Ready" end;
      ([$owned[] | select(counted) | .spec.nodeName] | unique | length) as $ready
    | if $observed < $ds.metadata.generation then
        "generation \($ds.metadata.generation) is not observed yet (observedGeneration \($observed))"
      elif $template == "" then "it has no deprecated.daemonset.template.generation annotation"
      elif $ready >= $desired then "ready"
      else
        "\($ready) of \($desired) nodes run a Ready pod of template generation \($template); other pods: "
        + ([$owned[] | select(counted | not) | "\(.metadata.name) on \(.spec.nodeName // "no node") (\(why))"]
          | if length == 0 then "none" else join(", ") end)
      end'
}

# Waits until new_pods_verdict reads "ready" for <daemonset>, for at most
# SNI_PROXY_ROLLOUT_SECONDS (300, the bound `rollout status` had).
wait_for_new_pods() {
  local daemonset="$1" bound="${SNI_PROXY_ROLLOUT_SECONDS:-300}"
  local deadline=$((SECONDS + bound)) ds selector pods verdict
  while :; do
    verdict="the DaemonSet or its pods could not be read"
    if ds="$(kubectl -n bex-system get daemonset "${daemonset}" -o json 2>/dev/null)" &&
      selector="$(jq -r '.spec.selector.matchLabels // {} | to_entries | map("\(.key)=\(.value)") | join(",")' <<<"${ds}")" &&
      [ -n "${selector}" ] &&
      pods="$(kubectl -n bex-system get pods -l "${selector}" -o json 2>/dev/null)"; then
      verdict="$(printf '%s\n%s\n' "${ds}" "${pods}" | new_pods_verdict)" || verdict="the DaemonSet or its pods could not be parsed"
    fi
    if [ "${verdict}" = ready ]; then
      echo "${daemonset}: every node runs a Ready pod of the current template"
      return 0
    fi
    ((SECONDS < deadline)) || break
    sleep "${SNI_PROXY_POLL_SECONDS:-5}"
  done
  echo "${daemonset}: ${verdict} after ${bound}s" >&2
  exit 1
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
    wait_for_new_pods "${daemonset}"
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
