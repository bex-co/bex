#!/usr/bin/env bash
# Read-only metadata inventory for the bounded-filesystem migration (w8/m144).
# JSONPath emits only reviewed binding/capacity/platform-image columns. Full
# App specs are never persisted or printed; no Secret, LUKS passphrase, pod
# exec, tenant filesystem, or database-data request is made.
set -euo pipefail
umask 077
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
[ -n "${KUBECONFIG:-}" ] && [ -s "$KUBECONFIG" ] \
  || { echo 'service-disk-inventory: an explicit fetched KUBECONFIG is required' >&2; exit 1; }
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
chmod 700 "$work"
projector="$repo_root/scripts/service_disk_inventory.py"
apps=$(python3 "$projector" projection apps)
claims=$(python3 "$projector" projection claims)
volumes=$(python3 "$projector" projection volumes)
kubectl get apps.app.bex.co --all-namespaces -o "jsonpath=$apps" > "$work/apps.tsv"
kubectl get persistentvolumeclaims --all-namespaces -o "jsonpath=$claims" > "$work/claims.tsv"
# Read only volumes referenced by App-owned claims; managed datastore and
# platform volume details never enter the report.
python3 "$projector" volume-names "$work/claims.tsv" > "$work/volume-names"
: > "$work/volumes.tsv"
while IFS= read -r name; do
  kubectl get persistentvolume "$name" --ignore-not-found -o "jsonpath=$volumes" >> "$work/volumes.tsv"
done < "$work/volume-names"
deployments=$(python3 "$projector" projection deployments)
pods=$(python3 "$projector" projection pods)
classes=$(python3 "$projector" projection classes)
kubectl -n bex-system get deployment bex-controller-manager bex-api -o "jsonpath=$deployments" > "$work/deployments.tsv"
kubectl -n bex-system get pods -l 'app.kubernetes.io/name in (control-plane,bex-api)' -o "jsonpath=$pods" > "$work/pods.tsv"
# These two names are the whole class preflight. Missing objects remain
# explicit report entries; no arbitrary cluster configuration is enumerated.
kubectl get storageclass hcloud-volumes-luks hcloud-volumes-luks-bounded --ignore-not-found -o "jsonpath=$classes" > "$work/classes.tsv"
python3 "$projector" report "$work/apps.tsv" "$work/claims.tsv" "$work/volumes.tsv" "$work/deployments.tsv" "$work/pods.tsv" "$work/classes.tsv"
