#!/usr/bin/env bash
# Read-only production view for an incident (w5/m501): why a service's pod is
# not running, whether the cluster autoscaler is adding nodes, and how full
# each node is. It only gets, describes and reads logs; it never mutates.
#
#   bash scripts/fetch-app-kubeconfig.sh /tmp/bex-app.kubeconfig
#   KUBECONFIG=/tmp/bex-app.kubeconfig bash scripts/cluster-diagnose.sh srv-<20 chars>
#       full detail, for an operator's own terminal
#   bash scripts/cluster-diagnose.sh srv-<20 chars> --public-log
#       what .github/workflows/cluster-diagnose.yml prints: counts, phases,
#       reasons, readiness and allocation, with tenant namespaces, pod and
#       service names, node names and addresses as keyed hashes and tenant
#       event text left out (scripts/lib/cluster_diagnose_public.py)
#
# This repository is public, so its Actions logs are too: a CI run uses
# --public-log, and the full view stays a local command away
# (docs/ADR019-infra-credentials.md §Decision 5).
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"

usage() { echo "usage: cluster-diagnose.sh srv-<20 chars> [--public-log]" >&2; exit 2; }
service_id=""
public=0
for arg in "$@"; do
  case "$arg" in
    --public-log) public=1 ;;
    srv-*) [ -z "$service_id" ] || usage; service_id="$arg" ;;
    *) usage ;;
  esac
done
# The id names a label selector below; refuse anything else before it reaches
# kubectl (and never echo it: the caller may be a public log).
[[ "$service_id" =~ ^srv-[0-9a-v]{20}$ ]] || { echo "service id must be srv-<20 chars>" >&2; exit 2; }

if [ "$public" = 1 ]; then
  exec python3 "$here/lib/cluster_diagnose_public.py" "$service_id"
fi

set +e
echo "== Service pods"
kubectl get pods -A -l "bex.co/app-id=$service_id" -o wide
for p in $(kubectl get pods -A -l "bex.co/app-id=$service_id" -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}'); do
  echo "===== $p"
  kubectl describe pod -n "${p%%/*}" "${p##*/}" | sed -n '/^Status:/,/^Events:/p;/^Events:/,$p' | tail -60
done
kubectl get deploy,rs -A -l "bex.co/app-id=$service_id" -o wide

echo
echo "== Pending pods cluster-wide"
kubectl get pods -A --field-selector=status.phase=Pending -o wide

echo
echo "== Nodes and allocation"
kubectl get nodes -L nodepool,bex.co/pool -o wide
kubectl describe nodes | grep -E '^Name:|^Taints:|Allocated resources' -A0
kubectl describe nodes | awk '/^Name:/{n=$2} /Allocated resources/{p=1;next} p&&/^Events:/{p=0} p{print n": "$0}' | grep -E 'cpu|memory'
kubectl get machinedeployments,machines -n bex-capi -o wide

echo
echo "== Cluster autoscaler"
kubectl get configmap -A --field-selector=metadata.name=cluster-autoscaler-status -o jsonpath='{range .items[*]}{.data.status}{"\n"}{end}' | head -80
for p in $(kubectl get pods -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}' | grep -i autoscaler); do
  echo "===== $p"
  kubectl logs -n "${p%%/*}" "${p##*/}" --tail=400 | grep -i -E 'scale.?up|unschedulable|max node|no node group|error|predicate|skipping|pod .* is unschedulable' | tail -60
done
exit 0
