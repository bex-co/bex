#!/usr/bin/env bash
# Tests deploy/argocd/daemonset-health.lua (w6/m518) in the Argo CD image
# production runs. deploy.yml installs the script as argocd-cm's
# resource.customizations.health.apps_DaemonSet, which replaces Argo's built-in
# DaemonSet check for every DaemonSet. So for each DaemonSet shape below that
# the bex operator has not annotated, the script must give exactly the built-in
# check's status and message, which `argocd admin settings resource-overrides
# health` returns when argocd-cm has no override. For an SNI proxy DaemonSet
# whose app.bex.co/new-pods-ready matches its generation and node count, it
# reads Healthy while the DaemonSet's status still counts a draining old pod.
#
# Needs docker. ARGOCD_IMAGE overrides the image.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
lua="$root/deploy/argocd/daemonset-health.lua"

# The image the pinned install.yaml runs (v3.4.6, the multi-arch index digest
# production's pods report). Moving ARGO_INSTALL_COMMIT means checking the
# built-in DaemonSet check again (gitops-engine pkg/health/health_daemonset.go)
# and moving both values here.
argo_install_commit=e1becb74c728a992804d39c3ceb2e9e6ae58f0ae
image="${ARGOCD_IMAGE:-quay.io/argoproj/argocd@sha256:6e9f4f1d646d9056c8e285495d0c8043b5f553c784181b3522ef324dcefdcc82}"

fails=0
fail() {
  echo "FAIL: $*" >&2
  fails=$((fails + 1))
}

if ! grep -qF "ARGO_INSTALL_COMMIT: ${argo_install_commit}" "$root/.github/workflows/deploy.yml"; then
  fail "deploy.yml no longer installs Argo CD ${argo_install_commit}: compare its DaemonSet health check with $lua and move the image in $0"
fi

# The key the operator writes is the key the script reads.
if ! grep -qF 'NewPodsReadyAnnotation = "app.bex.co/new-pods-ready"' "$root/lego/operator/internal/controller/sniproxy_rollout.go"; then
  fail "the operator no longer writes app.bex.co/new-pods-ready, which $lua reads"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/cases"

jq -n '{apiVersion: "v1", kind: "ConfigMap", metadata: {name: "argocd-cm", namespace: "argocd"}, data: {}}' >"$tmp/builtin.yaml"
jq -n --rawfile lua "$lua" '{apiVersion: "v1", kind: "ConfigMap", metadata: {name: "argocd-cm", namespace: "argocd"},
  data: {"resource.customizations.health.apps_DaemonSet": $lua}}' >"$tmp/lua.yaml"

# daemonset <case> <strategy|-> <generation> <observed|-> <desired> <updated> <available> <new-pods-ready|-> [other annotation]
# "-" leaves the field out; an observed of "-" leaves out the whole status.
daemonset() {
  jq -n --arg strategy "$2" --argjson generation "$3" --arg observed "$4" --argjson desired "$5" \
    --argjson updated "$6" --argjson available "$7" --arg ready "$8" --arg other "${9:-}" '{
      apiVersion: "apps/v1", kind: "DaemonSet",
      metadata: ({name: "bex-kv-sni-proxy", namespace: "bex-system", generation: $generation,
        annotations: ({"deprecated.daemonset.template.generation": "\($generation)"}
          + (if $ready == "-" then {} else {"app.bex.co/new-pods-ready": $ready} end)
          + (if $other == "" then {} else {($other): "1"} end))}),
      spec: (if $strategy == "-" then {} else {updateStrategy: {type: $strategy}} end)}
    + (if $observed == "-" then {} else {status: {observedGeneration: ($observed | tonumber),
        desiredNumberScheduled: $desired, updatedNumberScheduled: $updated, numberAvailable: $available}} end)' \
    >"$tmp/cases/$1.yaml"
}

# Shapes the built-in check decides: every branch, for both strategies and none,
# and annotations that must not change the answer.
n=0
for strategy in RollingUpdate OnDelete -; do
  for gens in "3 3" "4 3" "3 -"; do
    read -r generation observed <<<"$gens"
    for counts in "3 3 3" "3 2 3" "3 3 2" "3 2 2" "3 0 0" "0 0 0" "17 17 17"; do
      read -r desired updated available <<<"$counts"
      readies=(- "$((generation - 1))/$desired" "$generation/$((desired + 1))" "$generation")
      # A matching annotation changes nothing before the controller observes
      # the generation, for OnDelete, or where the pod counts are complete.
      if [ "$observed" != "$generation" ] || [ "$strategy" = OnDelete ] ||
        { [ "$updated" -ge "$desired" ] && [ "$available" -ge "$desired" ]; }; then
        readies+=("$generation/$desired")
      fi
      for ready in "${readies[@]}"; do
        n=$((n + 1))
        daemonset "same-$n" "$strategy" "$generation" "$observed" "$desired" "$updated" "$available" "$ready"
      done
    done
  done
done
daemonset same-foreign-annotation RollingUpdate 3 3 3 2 3 - example.com/other

# Shapes the operator's annotation decides.
# An old pod drains on one node: the status counts it, the annotation does not.
daemonset ready-draining RollingUpdate 5 5 3 2 3 5/3
# The draining pod has turned NotReady too.
daemonset ready-draining-unavailable RollingUpdate 5 5 3 2 2 5/3
# Every old pod drains.
daemonset ready-all-draining RollingUpdate 5 5 3 0 3 5/3

# One line per case and argocd-cm: "<case> <builtin|lua> STATUS: …|MESSAGE: …|",
# each written at once, so the parallel runs do not interleave.
cat >"$tmp/health.sh" <<'EVAL'
for cm in builtin lua; do
  out="$(argocd admin settings resource-overrides health "/w/cases/$1.yaml" --argocd-cm-path "/w/$cm.yaml" 2>/dev/null | tr '\n' '|')"
  printf '%s %s %s\n' "$1" "$cm" "${out:-ERROR}"
done
EVAL
docker run --rm -v "$tmp:/w:ro" --entrypoint sh "$image" -c \
  'ls /w/cases | sed "s/[.]yaml$//" | xargs -P 8 -n 1 sh /w/health.sh' >"$tmp/out"

# health <case> <builtin|lua>: what Argo printed for it.
health() {
  awk -v c="$1" -v cm="$2" '$1 == c && $2 == cm { sub(/^[^ ]+ [^ ]+ /, ""); print }' "$tmp/out"
}

same=0
for f in "$tmp"/cases/same-*.yaml; do
  c="$(basename "$f" .yaml)"
  builtin="$(health "$c" builtin)" lua_health="$(health "$c" lua)"
  if [[ "$builtin" != "STATUS: "* ]]; then
    fail "$c: the built-in check printed '$builtin'"
  elif [ "$builtin" != "$lua_health" ]; then
    fail "$c: the script differs from the built-in check: '$lua_health', want '$builtin' ($(jq -c '{spec, status, a: .metadata.annotations}' "$f"))"
  else
    same=$((same + 1))
  fi
done
echo "ok: the script matches the built-in check on $same unannotated or mismatched shapes"

want_ready="STATUS: Healthy|MESSAGE: All 3 nodes run a Ready pod of generation 5 (app.bex.co/new-pods-ready); old pods are still draining|"
for c in ready-draining ready-draining-unavailable ready-all-draining; do
  got="$(health "$c" lua)" builtin="$(health "$c" builtin)"
  if [ "$got" != "$want_ready" ]; then
    fail "$c: the script reads '$got', want '$want_ready'"
  elif [[ "$builtin" != "STATUS: Progressing|"* ]]; then
    fail "$c: the built-in check reads '$builtin', so the case proves nothing"
  else
    echo "ok: $c is Healthy where the built-in check reads Progressing"
  fi
done

if [ "$fails" -eq 0 ]; then
  echo "PASS: argocd-daemonset-health"
else
  echo "FAIL: $fails case(s)" >&2
  exit 1
fi
