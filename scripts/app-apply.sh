#!/usr/bin/env bash
# Apply a project's bex.yml (App manifest, render.yaml-style) as App CRs.
# Usage: scripts/app-apply.sh <path-to-bex.yml | project-dir>
#        DRY_RUN=1 scripts/app-apply.sh ...   # print the App CRs instead of applying
# Requires: yq v4, kubectl (respects $KUBECONFIG).
#
# Manifest semantics: `type: web` (the default) is public — <name>.<base-domain>
# is auto-assigned; `domains:` adds custom domains on top. `type: private` is
# in-cluster only. The `expose` knob is CR-level mechanism, not a manifest key.
set -euo pipefail

manifest="${1:?usage: app-apply.sh <bex.yml | project-dir>}"
[ -d "$manifest" ] && manifest="$manifest/bex.yml"
[ -f "$manifest" ] || { echo "error: $manifest not found" >&2; exit 1; }

count="$(yq '.apps | length' "$manifest")"
[ "$count" -gt 0 ] 2>/dev/null || { echo "error: no .apps[] entries in $manifest" >&2; exit 1; }

for i in $(seq 0 $((count - 1))); do
  name="$(yq ".apps[$i].name" "$manifest")"
  [ "$name" != "null" ] || { echo "error: .apps[$i].name is required" >&2; exit 1; }

  # type decides exposure (render.yaml-style): "web" (default) is public — the
  # platform hostname <name>.<BEX_BASE_DOMAIN> is auto-assigned and mandatory
  # (spec.expose); "private" is in-cluster only (ClusterIP, no Ingress, no domains).
  apptype="$(yq ".apps[$i].type // \"web\"" "$manifest")"
  ndomains="$(yq ".apps[$i].domains | length" "$manifest")"
  case "$apptype" in
    web) expose_json=true ;;
    private)
      expose_json=null
      if [ "$ndomains" -gt 0 ] 2>/dev/null; then
        echo "error: $name is type: private but lists domains — private services have no ingress" >&2
        exit 1
      fi
      ;;
    *) echo "error: $name has unknown type \"$apptype\" (web|private)" >&2; exit 1 ;;
  esac

  # Project .apps[i] onto an App CR: name -> metadata, domains[0] -> spec.host
  # (canonical; keeps existing Apps' TLS secret stable), domains[1:] -> spec.hosts
  # (extra hosts, e.g. customers' custom domains), everything else 1:1; drop
  # null/absent fields so operator defaults apply.
  cr="$(yq -o=yaml "
    .apps[$i] as \$a |
    {
      \"apiVersion\": \"app.bex.co/v1alpha1\",
      \"kind\": \"App\",
      \"metadata\": {\"name\": \$a.name, \"namespace\": (\$a.namespace // \"default\")},
      \"spec\": {
        \"image\": \$a.image,
        \"repo\": \$a.repo,
        \"branch\": \$a.branch,
        \"builder\": \$a.builder,
        \"port\": \$a.port,
        \"replicas\": \$a.replicas,
        \"tier\": \$a.tier,
        \"healthCheckPath\": \$a.healthCheckPath,
        \"host\": \$a.domains[0],
        \"hosts\": ((\$a.domains // []) | .[1:] | select(length > 0) // null),
        \"expose\": $expose_json
      } | with_entries(select(.value != null))
    } | ... comments=\"\"" "$manifest")"

  if [ "${DRY_RUN:-}" = "1" ]; then
    echo "---"; echo "$cr"
  else
    echo "$cr" | kubectl apply -f -
  fi
done

if [ "${DRY_RUN:-}" != "1" ]; then
  echo "watch:  kubectl get apps.app.bex.co -w"
  echo "serve:  curl \$(kubectl get apps.app.bex.co <name> -o jsonpath='{.status.url}')/"
fi
