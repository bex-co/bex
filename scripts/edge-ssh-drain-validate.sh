#!/usr/bin/env bash
# The edge's half of Render's SSH maintenance hour (w6/164) — pure client-side,
# no cluster. Render "gives existing connections one hour before automatically
# closing them" when it maintains the infrastructure behind SSH
# (render.com/docs/ssh). The ssh-gateway gives that hour on its own release
# (w6/m482); Traefik sits in front of it on :22, and before w6/164 a Traefik
# roll closed every native SSH session 10 s after the old pod's SIGTERM.
#
# Over the rendered Traefik releases and the Prometheus Application:
#
#   1. Production gives the `ssh` entrypoint's open streams at least an hour
#      (transport.lifeCycle.graceTimeOut), and the pod's grace period covers
#      that hour plus Traefik's own close, or the kubelet kills the drain.
#   2. Production spreads its pods with a hostname topology spread, never with
#      required pod anti-affinity. The scheduler counts a terminating pod for
#      anti-affinity but not for a topology spread, so under anti-affinity the
#      roll's second new pod waits Pending behind a draining old pod for the
#      hour, and the edge runs two releases for that hour.
#   3. The base edge, one hostNetwork pod rolled with Recreate, keeps the
#      default grace: Recreate starts the new pod only after the old one exits,
#      so an hour's drain there is an hour with no edge.
#   4. Prometheus scrapes only Ready Traefik pods. A draining pod's metrics
#      entrypoint is closed, and its `up` of 0, or its series ending in the
#      next hour, would fail that hour's HTTP and WebSocket egress windows
#      (lego/backend/internal/egressquery Health) for every service.
#
# Called by scripts/gitops-validate.sh with the manifests it renders;
# scripts/edge-ssh-drain-validate.test.sh drives it against fixtures.
# Requires: yq v4.
set -euo pipefail
cd "$(dirname "$0")/.."

TRAEFIK_BASE_RENDER="${TRAEFIK_BASE_RENDER:?the rendered base Traefik release}"
TRAEFIK_PROD_RENDER="${TRAEFIK_PROD_RENDER:?the rendered production Traefik release}"
PROMETHEUS_APP="${PROMETHEUS_APP:-deploy/gitops/base/prometheus.yaml}"
HOUR=3600
# Traefik's own close stage after the bound (Server.Close panics after 10 s),
# with room to spare.
CLOSE_MARGIN=30
SPREAD='{"labelSelector":{"matchLabels":{"app.kubernetes.io/instance":"traefik-traefik","app.kubernetes.io/name":"traefik"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"DoNotSchedule"}'

fail=0

# seconds <duration> — a whole number of s, m or h (Traefik's spelling) in
# seconds, or nothing for any other spelling.
seconds() {
  [[ "$1" =~ ^([0-9]+)([smh])$ ]] || return 0
  case "${BASH_REMATCH[2]}" in
    s) echo $((10#${BASH_REMATCH[1]})) ;;
    m) echo $((10#${BASH_REMATCH[1]} * 60)) ;;
    h) echo $((10#${BASH_REMATCH[1]} * 3600)) ;;
  esac
}

deployment() { yq -N "select(.kind == \"Deployment\") | $1" "$2"; }

ssh_grace_arg() {
  deployment '.spec.template.spec.containers[] | select(.name == "traefik") | .args[] | select(test("^--entryPoints\.ssh\.transport\.lifeCycle\.graceTimeOut=")) | sub("^[^=]*="; "")' "$1"
}

prod_drain="$(ssh_grace_arg "$TRAEFIK_PROD_RENDER")"
prod_drain_s="$(seconds "$prod_drain")"
prod_pod_grace="$(deployment '.spec.template.spec.terminationGracePeriodSeconds' "$TRAEFIK_PROD_RENDER")"
if ! [[ "$prod_drain_s" =~ ^[0-9]+$ ]] || ((prod_drain_s < HOUR)); then
  echo "FAIL: production Traefik ssh entrypoint graceTimeOut is '$prod_drain', want at least ${HOUR}s: a Traefik roll otherwise closes every native SSH session 10 s after the old pod's SIGTERM, where Render gives them an hour (w6/164)" >&2
  fail=1
elif ! [[ "$prod_pod_grace" =~ ^[0-9]+$ ]] || ((prod_pod_grace < prod_drain_s + CLOSE_MARGIN)); then
  echo "FAIL: production Traefik terminationGracePeriodSeconds is '$prod_pod_grace', want at least the ssh graceTimeOut ($prod_drain) plus ${CLOSE_MARGIN} s, or the kubelet kills a draining edge pod mid-session (w6/164)" >&2
  fail=1
fi

prod_required="$(deployment '.spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[]? | .topologyKey' "$TRAEFIK_PROD_RENDER")"
prod_spread="$(yq -N -o=json -I=0 'select(.kind == "Deployment") | .spec.template.spec.topologySpreadConstraints[]? | sort_keys(..)' "$TRAEFIK_PROD_RENDER" | grep -cxF "$SPREAD" || true)"
if [ -n "$prod_required" ]; then
  echo "FAIL: production Traefik uses required pod anti-affinity ($prod_required): the scheduler counts a draining pod for it, so a roll's second new pod waits up to an hour behind the first old one; spread with a topology spread constraint instead (w6/164)" >&2
  fail=1
fi
if [ "$prod_spread" != 1 ]; then
  echo "FAIL: production Traefik must carry exactly the topology spread $SPREAD (one live pod per node; terminating pods are not counted, and no matchLabelKeys, which would let an old and a new pod share a node) (w6/164)" >&2
  fail=1
fi

base_strategy="$(deployment '.spec.strategy.type' "$TRAEFIK_BASE_RENDER")"
base_drain="$(ssh_grace_arg "$TRAEFIK_BASE_RENDER")"
if [ "$base_strategy" = Recreate ] && [ -n "$base_drain" ]; then
  echo "FAIL: the base Traefik release rolls with Recreate but sets an ssh graceTimeOut ($base_drain): its new pod would start only after the old one drained, with no edge meanwhile; set the hour in the production overlay only (w6/164)" >&2
  fail=1
fi

for job in traefik traefik-websocket-meter; do
  keep="$(yq -N -o=json -I=0 "
    .spec.source.helm.values | from_yaml |
    .serverFiles.\"prometheus.yml\".scrape_configs[] | select(.job_name == \"$job\") |
    .relabel_configs[] | select(.action == \"keep\") | {\"src\": .source_labels, \"regex\": .regex}" "$PROMETHEUS_APP")"
  if [ "$keep" != '{"src":["__meta_kubernetes_pod_label_app_kubernetes_io_name","__meta_kubernetes_pod_ready"],"regex":"traefik;true"}' ]; then
    echo "FAIL: Prometheus job $job keeps '$keep', want only Ready traefik pods (source_labels [__meta_kubernetes_pod_label_app_kubernetes_io_name, __meta_kubernetes_pod_ready], regex traefik;true): a draining edge pod would fail the next hour's egress windows (w6/164)" >&2
    fail=1
  fi
done

exit "$fail"
