#!/usr/bin/env bash
# Self-test for scripts/edge-ssh-drain-validate.sh (w6/164). Each failure it
# guards is silent until an edge roll: a missing or short ssh graceTimeOut cuts
# every native SSH session 10 s into the roll, a pod grace below it lets the
# kubelet do the same at the grace, required anti-affinity holds the roll's
# second pod for the hour, an hour's drain on the Recreate edge is an hour with
# no edge, and a draining pod Prometheus still scrapes fails the next hour's
# egress windows.
#
# Method: synthetic rendered Traefik Deployments (the shape helm template
# gives) and a copy of the real Prometheus Application, one mutated fact per
# case, asserting the exit code and the message.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
SCRIPT="$here/edge-ssh-drain-validate.sh"

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# render <file> <strategy> <grace> <args...> — a rendered Traefik Deployment.
render() {
  local file="$1" strategy="$2" grace="$3"
  shift 3
  {
    cat <<YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: traefik
  namespace: traefik
spec:
  strategy:
    type: $strategy
  template:
    spec:
      terminationGracePeriodSeconds: $grace
      containers:
        - name: traefik
          args:
            - "--entryPoints.ssh.address=:2222/tcp"
YAML
    local arg
    for arg in "$@"; do
      printf '            - "%s"\n' "$arg"
    done
  } >"$file"
}

# spread <file> [<yq update>] — add the production topology spread, then an
# optional mutation of it.
spread() {
  yq -i '.spec.template.spec.topologySpreadConstraints = [{"labelSelector": {"matchLabels": {"app.kubernetes.io/name": "traefik", "app.kubernetes.io/instance": "traefik-traefik"}}, "topologyKey": "kubernetes.io/hostname", "whenUnsatisfiable": "DoNotSchedule", "maxSkew": 1}]' "$1"
  if [ -n "${2:-}" ]; then yq -i "$2" "$1"; fi
}

# fixture <name> — a passing set of inputs; echoes the fixture dir.
fixture() {
  local dir="$tmp/$1"
  mkdir -p "$dir"
  render "$dir/base.yaml" Recreate 60
  render "$dir/prod.yaml" RollingUpdate 3660 "--entryPoints.ssh.transport.lifeCycle.graceTimeOut=3600s"
  spread "$dir/prod.yaml"
  cp "$root/deploy/gitops/base/prometheus.yaml" "$dir/prometheus.yaml"
  echo "$dir"
}

# scrape_keep <dir> <job> <yaml list> <regex> — rewrite one job's keep rule.
scrape_keep() {
  local values
  values="$(yq ".spec.source.helm.values" "$1/prometheus.yaml" | yq "
    (.serverFiles.\"prometheus.yml\".scrape_configs[] | select(.job_name == \"$2\") |
      .relabel_configs[] | select(.action == \"keep\")) |= {\"source_labels\": $3, \"action\": \"keep\", \"regex\": \"$4\"}")"
  VALUES="$values" yq -i '.spec.source.helm.values = strenv(VALUES)' "$1/prometheus.yaml"
}

# expect <name> <dir> <0|1> [<message fragment>]
expect() {
  local name="$1" dir="$2" want="$3" fragment="${4:-}" out rc=0
  out="$(TRAEFIK_BASE_RENDER="$dir/base.yaml" TRAEFIK_PROD_RENDER="$dir/prod.yaml" \
    PROMETHEUS_APP="$dir/prometheus.yaml" bash "$SCRIPT" 2>&1)" || rc=$?
  if [ "$rc" != "$want" ]; then
    echo "FAIL [$name]: exit $rc, want $want; output: $out" >&2
    fails=$((fails + 1))
  elif [ -n "$fragment" ] && ! grep -qF -- "$fragment" <<<"$out"; then
    echo "FAIL [$name]: output lacks '$fragment': $out" >&2
    fails=$((fails + 1))
  else
    echo "ok   [$name]"
  fi
}

# The real Prometheus Application with the production layout passes.
dir="$(fixture pass)"
expect "the hour, its grace, the spread and Ready-only scrapes pass" "$dir" 0

dir="$(fixture hour-in-minutes)"
render "$dir/prod.yaml" RollingUpdate 3660 "--entryPoints.ssh.transport.lifeCycle.graceTimeOut=60m"
spread "$dir/prod.yaml"
expect "the hour spelled 60m passes" "$dir" 0

dir="$(fixture no-hour)"
render "$dir/prod.yaml" RollingUpdate 60
spread "$dir/prod.yaml"
expect "no ssh graceTimeOut (Traefik's 10 s) fails" "$dir" 1 "graceTimeOut is ''"

dir="$(fixture short-hour)"
render "$dir/prod.yaml" RollingUpdate 3660 "--entryPoints.ssh.transport.lifeCycle.graceTimeOut=600s"
spread "$dir/prod.yaml"
expect "a 10-minute ssh graceTimeOut fails" "$dir" 1 "graceTimeOut is '600s'"

dir="$(fixture short-pod-grace)"
render "$dir/prod.yaml" RollingUpdate 3600 "--entryPoints.ssh.transport.lifeCycle.graceTimeOut=1h"
spread "$dir/prod.yaml"
expect "a pod grace equal to the hour fails" "$dir" 1 "terminationGracePeriodSeconds is '3600'"

dir="$(fixture anti-affinity)"
yq -i '.spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution = [{"labelSelector": {"matchLabels": {"app.kubernetes.io/name": "traefik"}}, "topologyKey": "kubernetes.io/hostname"}]' "$dir/prod.yaml"
expect "required pod anti-affinity fails" "$dir" 1 "required pod anti-affinity (kubernetes.io/hostname)"

dir="$(fixture no-spread)"
yq -i 'del(.spec.template.spec.topologySpreadConstraints)' "$dir/prod.yaml"
expect "no topology spread fails" "$dir" 1 "exactly the topology spread"

dir="$(fixture soft-spread)"
spread "$dir/prod.yaml" '.spec.template.spec.topologySpreadConstraints[0].whenUnsatisfiable = "ScheduleAnyway"'
expect "a ScheduleAnyway spread fails" "$dir" 1 "exactly the topology spread"

dir="$(fixture per-revision-spread)"
spread "$dir/prod.yaml" '.spec.template.spec.topologySpreadConstraints[0].matchLabelKeys = ["pod-template-hash"]'
expect "a spread per pod-template-hash fails" "$dir" 1 "exactly the topology spread"

dir="$(fixture recreate-drain)"
render "$dir/base.yaml" Recreate 3660 "--entryPoints.ssh.transport.lifeCycle.graceTimeOut=3600s"
expect "an hour's drain on the Recreate edge fails" "$dir" 1 "rolls with Recreate but sets an ssh graceTimeOut (3600s)"

for job in traefik traefik-websocket-meter; do
  dir="$(fixture "scrape-draining-$job")"
  scrape_keep "$dir" "$job" '["__meta_kubernetes_pod_label_app_kubernetes_io_name"]' traefik
  expect "Prometheus job $job scraping a draining pod fails" "$dir" 1 \
    "Prometheus job $job keeps '{\"src\":[\"__meta_kubernetes_pod_label_app_kubernetes_io_name\"],\"regex\":\"traefik\"}'"
done

if [ "$fails" -ne 0 ]; then
  echo "$fails case(s) failed" >&2
  exit 1
fi
echo "edge-ssh-drain-validate self-test: all cases passed"
