#!/usr/bin/env bash
# Reproducible static-site trust-boundary verification (w7/m54).
#
# Usage:
#   scripts/verify-static-site-security.sh repo
#   KUBECONFIG=... scripts/verify-static-site-security.sh live
#
# PSL_EXPECTED defaults to `present`, the production contract. Use `absent`
# only to capture the explicitly vulnerable pre-upstream baseline. The live
# mode is read-only except for the isolated object-store probe created and
# deleted by static-s3-credentials.sh; admission requests use server dry-run.
set -euo pipefail
cd "$(dirname "$0")/.."

MODE="${1:-}"
case "$MODE" in
  repo|live) ;;
  *) echo "usage: $0 {repo|live}" >&2; exit 2 ;;
esac

PSL_EXPECTED="${PSL_EXPECTED:-present}"
PSL_URL="${PSL_URL:-https://raw.githubusercontent.com/publicsuffix/list/master/public_suffix_list.dat}"
MANAGER_IDENTITY="system:serviceaccount:bex-system:bex-controller-manager"

command -v curl >/dev/null || { echo "error: curl not found" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq not found" >&2; exit 1; }
command -v kubectl >/dev/null || { echo "error: kubectl not found" >&2; exit 1; }

psl_entry="$(curl -fsSL "$PSL_URL" | awk '$0 == "onbex.co" { print; exit }')"
case "$PSL_EXPECTED" in
  present)
    [ "$psl_entry" = "onbex.co" ] || {
      echo "FAIL  PSL: canonical list does not contain onbex.co" >&2
      exit 1
    }
    echo "PASS  PSL: canonical list contains onbex.co"
    ;;
  absent)
    [ -z "$psl_entry" ] || {
      echo "FAIL  PSL baseline: onbex.co is now present; update the expected contract" >&2
      exit 1
    }
    echo "PASS  PSL baseline: canonical list does not yet contain onbex.co"
    ;;
  *) echo "error: PSL_EXPECTED must be present or absent" >&2; exit 2 ;;
esac

PSL_EXPECTED="$PSL_EXPECTED" node scripts/static-site-browser-isolation.mjs
(cd lego/operator && go test ./internal/staticserver -run 'TestRewriteNeverFetchesAnUpstreamURL')

if [ "$MODE" = "repo" ]; then
  echo "PASS  repository static-site security checks"
  exit 0
fi

kubectl get validatingadmissionpolicy bex-operator-platform-aliases >/dev/null
kubectl get validatingadmissionpolicybinding bex-operator-platform-aliases >/dev/null
kubectl get validatingadmissionpolicybinding bex-operator-platform-aliases-default >/dev/null

namespaces_json="$(kubectl get namespaces -o json)"
hosting_namespaces="$(jq -r '
  .items[] | select(.metadata.name == "default" or
    (.metadata.labels."app.kubernetes.io/managed-by" == "bex-controlplane" and
     .metadata.labels."app.kubernetes.io/part-of" == "bex" and
     .metadata.labels."app.bex.co/regime" == "hosting" and
     ((.metadata.labels."app.bex.co/workspace" // "") | test("^tea-[0-9a-v]{20}$")) and
     .metadata.name == .metadata.labels."app.bex.co/workspace")) |
  .metadata.name' <<<"$namespaces_json" | sort -u)"
hosting_namespace_json="$(jq -Rn '[inputs | select(length > 0)]' <<<"$hosting_namespaces")"
hosting_namespace_count="$(jq 'length' <<<"$hosting_namespace_json")"
[ "$hosting_namespace_count" -gt 1 ] || {
  echo "error: expected default plus at least one tenant hosting namespace" >&2
  exit 1
}

apps_json="$(kubectl get apps.app.bex.co -A -o json)"
unprotected_app_namespaces="$(jq -r --argjson protected "$hosting_namespace_json" '
  ([.items[].metadata.namespace] | unique) - $protected | .[]' <<<"$apps_json")"
[ -z "$unprotected_app_namespaces" ] || {
  echo "FAIL  inventory: App namespaces outside the admission-protected hosting set:" >&2
  printf '%s\n' "$unprotected_app_namespaces" >&2
  exit 1
}

invalid_aliases="$(while IFS= read -r namespace; do
  [ -n "$namespace" ] || continue
  kubectl -n "$namespace" get services -o json | jq -r '
    .items[] | select(.spec.type == "ExternalName") |
    . as $service |
    ($service.metadata.labels."app.bex.co/app" // "") as $app |
    ($service.metadata.labels."app.bex.co/platform-alias" // "") as $purpose |
    ([($service.metadata.ownerReferences // [])[] |
      select(.controller == true and .apiVersion == "app.bex.co/v1alpha1" and
             .kind == "App" and .name == $app)] | length) as $owners |
    select(
      $service.metadata.labels."app.kubernetes.io/managed-by" != "bex-operator" or
      $owners != 1 or
      ((($purpose == "static-server" and
        $service.metadata.name == ("bex-static-" + $app) and
        $service.spec.externalName == "bex-static-server.bex-system.svc.cluster.local" and
        ($service.spec.ports | length) == 1 and $service.spec.ports[0].port == 8080) or
       ($purpose == "maintenance" and
        $service.metadata.name == ("bex-maintenance-" + $app) and
        $service.spec.externalName == "bex-activator.bex-system.svc.cluster.local" and
        ($service.spec.ports | length) == 1 and $service.spec.ports[0].port == 8888)) | not)) |
    [.metadata.namespace,.metadata.name] | @tsv'
done <<<"$hosting_namespaces")"
[ -z "$invalid_aliases" ] || {
  echo "FAIL  inventory: noncanonical ExternalName aliases remain:" >&2
  printf '%s\n' "$invalid_aliases" >&2
  exit 1
}
echo "PASS  inventory: every hosting ExternalName alias has an exact operator/App-owned shape"

app_json="$(jq -c '
  [.items[] | select(.spec.type == "static_site" and (.status.url // "") != "")] as $apps |
  (($apps | map(select(.metadata.namespace != "default")) | first) //
   ($apps | first) // empty)' <<<"$apps_json")"
[ -n "$app_json" ] || { echo "error: no live static_site App with a URL found" >&2; exit 1; }
app_namespace="$(jq -r '.metadata.namespace' <<<"$app_json")"
app_name="$(jq -r '.metadata.name' <<<"$app_json")"
app_uid="$(jq -r '.metadata.uid' <<<"$app_json")"
app_url="$(jq -r '.status.url' <<<"$app_json")"
alias_name="bex-static-$app_name"
maintenance_alias_name="bex-maintenance-$app_name"

# Enumerate every ServiceAccount where it exists and every cross-namespace
# ServiceAccount subject where a RoleBinding grants it authority. Checking each
# identity in each effective namespace catches namespaced Roles as well as any
# accidental ClusterRoleBinding without an O(namespaces²) cross-product.
identity_scopes="$(while IFS= read -r namespace; do
  [ -n "$namespace" ] || continue
  kubectl -n "$namespace" get serviceaccounts -o json | jq -r --arg namespace "$namespace" '
    .items[].metadata.name |
    [$namespace, ("system:serviceaccount:" + $namespace + ":" + .)] | @tsv'
  kubectl -n "$namespace" get rolebindings -o json | jq -r --arg namespace "$namespace" '
    .items[].subjects[]? | select(.kind == "ServiceAccount") |
    [$namespace, ("system:serviceaccount:" + (.namespace // $namespace) + ":" + .name)] | @tsv'
done <<<"$hosting_namespaces" | sort -u | awk -F '\t' -v manager="$MANAGER_IDENTITY" '$2 != manager')"
[ -n "$identity_scopes" ] || { echo "error: no tenant-facing identity scopes found" >&2; exit 1; }

# Eight SubjectAccessReviews per scope run concurrently, keeping the exhaustive
# live proof bounded without weakening it to a manifest-only RBAC inspection.
rbac_failures="$(while IFS=$'\t' read -r scope_namespace identity; do
  [ -n "$scope_namespace" ] && [ -n "$identity" ] || continue
  for resource in services ingresses.networking.k8s.io; do
    for verb in create update patch delete; do
      (
        answer="$(kubectl auth can-i "$verb" "$resource" -n "$scope_namespace" --as="$identity" 2>/dev/null || true)"
        if [ "$answer" != "no" ]; then
          printf '%s can %s %s in %s (answer=%s)\n' "$identity" "$verb" "$resource" "$scope_namespace" "${answer:-empty}"
        fi
      ) &
    done
  done
  wait
done <<<"$identity_scopes")"
[ -z "$rbac_failures" ] || {
  echo "FAIL  RBAC: tenant-facing mutation authority detected:" >&2
  printf '%s\n' "$rbac_failures" >&2
  exit 1
}
identity_scope_count="$(wc -l <<<"$identity_scopes" | tr -d ' ')"
identity_count="$(cut -f2 <<<"$identity_scopes" | sort -u | wc -l | tr -d ' ')"
echo "PASS  RBAC: $identity_count identities in $identity_scope_count scopes across $hosting_namespace_count hosting namespaces cannot mutate Services or Ingresses"

for resource in services ingresses.networking.k8s.io; do
  for verb in create update patch delete; do
    answer="$(kubectl auth can-i "$verb" "$resource" -n "$app_namespace" --as="$MANAGER_IDENTITY" 2>/dev/null || true)"
    [ "$answer" = "yes" ] || {
      echo "FAIL  RBAC: operator cannot $verb $resource in $app_namespace" >&2
      exit 1
    }
  done
done
echo "PASS  RBAC: operator retains alias and Ingress reconciliation authority"

service_manifest() {
  local purpose="$1" name="$2" target="$3" port="$4"
  jq -cn \
    --arg namespace "$app_namespace" --arg name "$name" --arg app "$app_name" \
    --arg uid "$app_uid" --arg purpose "$purpose" --arg target "$target" \
    --argjson port "$port" '
    {apiVersion:"v1",kind:"Service",
     metadata:{namespace:$namespace,name:$name,
       labels:{"app.bex.co/app":$app,"app.bex.co/platform-alias":$purpose,
               "app.kubernetes.io/managed-by":"bex-operator"},
       ownerReferences:[{apiVersion:"app.bex.co/v1alpha1",kind:"App",name:$app,
                         uid:$uid,controller:true,blockOwnerDeletion:true}]},
     spec:{type:"ExternalName",externalName:$target,
           ports:[{name:"http",port:$port,protocol:"TCP"}]}}'
}

existing_alias="$(kubectl -n "$app_namespace" get service "$alias_name" -o json \
  | jq 'del(.metadata.annotations."kubectl.kubernetes.io/last-applied-configuration",
        .metadata.creationTimestamp,.metadata.generation,.metadata.managedFields,
        .metadata.selfLink,.status)')"
jq . <<<"$existing_alias" \
  | kubectl replace --dry-run=server --as="$MANAGER_IDENTITY" -f - >/dev/null

if existing_maintenance="$(kubectl -n "$app_namespace" get service "$maintenance_alias_name" -o json 2>/dev/null)"; then
  jq 'del(.metadata.annotations."kubectl.kubernetes.io/last-applied-configuration",
          .metadata.creationTimestamp,.metadata.generation,.metadata.managedFields,
          .metadata.selfLink,.status)' <<<"$existing_maintenance" \
    | kubectl replace --dry-run=server --as="$MANAGER_IDENTITY" -f - >/dev/null
else
  service_manifest maintenance "$maintenance_alias_name" \
    bex-activator.bex-system.svc.cluster.local 8888 \
    | kubectl create --dry-run=server --as="$MANAGER_IDENTITY" -f - >/dev/null
fi
echo "PASS  admission: operator exact static update and maintenance create/update shapes are admitted"

for target in \
  bex-api.bex-system.svc.cluster.local \
  zot.bex-registry.svc.cluster.local \
  openbao.secrets.svc.cluster.local \
  postgres-rw."$app_namespace".svc.cluster.local \
  example.com; do
  if jq --arg target "$target" '.spec.externalName = $target' <<<"$existing_alias" \
      | kubectl replace --dry-run=server --as="$MANAGER_IDENTITY" -f - >/dev/null 2>&1; then
    echo "FAIL  admission: hostile ExternalName target was admitted" >&2
    exit 1
  fi
done
echo "PASS  admission: platform, registry, secrets, database, and external targets are rejected"

curl -fsS --max-time 20 "$app_url" >/dev/null
echo "PASS  serving: live static platform URL returned success"

scripts/static-s3-credentials.sh verify
echo "PASS  live static-site trust boundaries"
