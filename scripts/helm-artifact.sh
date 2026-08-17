#!/usr/bin/env bash
# Fetch and authenticate production Helm inputs against repository-reviewed
# digests. Argo never resolves the mutable upstream repositories: deploy.yml
# mirrors the verified archives to GHCR and Applications select the immutable
# OCI manifest digests committed in deploy/helm-artifacts.lock. Helm injects the
# current time into org.opencontainers.image.created on every push, so the raw
# manifest digest is inherently unstable. The mirror rewrites only that field
# away after validating the exact Helm manifest shape and archive layer, then
# publishes and verifies the resulting deterministic manifest.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
lock_file="$repo_root/deploy/helm-artifacts.lock"
mirror_registry=${BEX_HELM_MIRROR:-ghcr.io/bex-co/bex-charts}
helm_transport_args=()
oras_transport_args=()
if [ "${BEX_HELM_PLAIN_HTTP:-}" = 1 ]; then
  helm_transport_args+=(--plain-http)
  oras_transport_args+=(--plain-http)
fi

die() {
  printf 'helm-artifact: %s\n' "$*" >&2
  exit 1
}

lookup() {
  local chart=$1
  local line
  line=$(awk -F '|' -v chart="$chart" '$1 == chart { print }' "$lock_file")
  [ -n "$line" ] || die "chart is not locked: $chart"
  [ "$(printf '%s\n' "$line" | wc -l | tr -d ' ')" = 1 ] || die "duplicate lock entry: $chart"
  IFS='|' read -r locked_name locked_version locked_repo locked_archive_sha locked_oci_digest <<<"$line"
  [[ "$locked_name" =~ ^[a-z0-9][a-z0-9-]*$ ]] || die "invalid chart name in lock: $locked_name"
  [[ "$locked_version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([-.][A-Za-z0-9.-]+)?$ ]] || die "invalid chart version in lock: $locked_version"
  [[ "$locked_repo" =~ ^https://[^[:space:]]+$ ]] || die "invalid upstream URL in lock: $locked_repo"
  [[ "$locked_archive_sha" =~ ^[0-9a-f]{64}$ ]] || die "invalid archive digest in lock: $locked_name"
  if [ "$locked_oci_digest" != "-" ]; then
    [[ "$locked_oci_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || die "invalid OCI digest in lock: $locked_name"
  fi
}

pull_locked() {
  local chart=$1
  local destination=$2
  lookup "$chart"
  install -d -m 700 "$destination"
  helm pull "$locked_name" \
    --repo "$locked_repo" \
    --version "$locked_version" \
    --destination "$destination"
  local archives=("$destination/$locked_name-"*.tgz)
  [ "${#archives[@]}" = 1 ] || die "expected one downloaded archive for $locked_name"
  printf '%s  %s\n' "$locked_archive_sha" "${archives[0]}" | sha256sum -c - >&2
  printf '%s\n' "${archives[0]}"
}

mirror_one() {
  local chart=$1
  local destination=$2
  lookup "$chart"
  [ "$locked_oci_digest" != "-" ] || die "$chart has no reviewed OCI identity"
  local archive
  archive=$(pull_locked "$chart" "$destination")
  local output
  output=$(helm push "$archive" "oci://$mirror_registry" "${helm_transport_args[@]}" 2>&1) \
    || die "failed to upload $chart blobs: $output"

  # `helm push` adds the wall clock to the OCI manifest, which makes its digest
  # change on every invocation. Fetch the just-pushed manifest, prove it has
  # exactly one locked chart layer and the expected Helm media types, remove
  # only the volatile timestamp, then replace the tag through ORAS. The stable
  # manifest digest is the identity reviewed in the lock and used by Argo.
  command -v oras >/dev/null || die "oras is required to normalize Helm OCI manifests"
  local reference="$mirror_registry/$locked_name:$locked_version"
  local raw_manifest="$destination/$locked_name.raw-manifest.json"
  local stable_manifest="$destination/$locked_name.stable-manifest.json"
  oras manifest fetch "${oras_transport_args[@]}" "$reference" -o "$raw_manifest" \
    || die "failed to fetch the uploaded $chart manifest"
  jq -ce --arg archive_digest "sha256:$locked_archive_sha" '
    if .schemaVersion != 2
      or ((.mediaType // "application/vnd.oci.image.manifest.v1+json")
          != "application/vnd.oci.image.manifest.v1+json")
      or .config.mediaType != "application/vnd.cncf.helm.config.v1+json"
      or (.layers | length) != 1
      or .layers[0].mediaType != "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
      or .layers[0].digest != $archive_digest
      or ((.annotations["org.opencontainers.image.created"] | type) != "string")
    then error("unexpected Helm OCI manifest shape")
    else del(.annotations["org.opencontainers.image.created"])
    end
  ' "$raw_manifest" >"$stable_manifest" \
    || die "uploaded $chart manifest does not contain the reviewed chart layer"
  local descriptor
  descriptor=$(oras manifest push "${oras_transport_args[@]}" --descriptor \
    --media-type application/vnd.oci.image.manifest.v1+json \
    "$reference" "$stable_manifest" 2>&1) \
    || die "failed to publish the stable $chart manifest: $descriptor"
  local pushed_digest
  pushed_digest=$(printf '%s\n' "$descriptor" | jq -er '.digest') \
    || die "stable $chart push did not return a manifest digest"
  [ "$pushed_digest" = "$locked_oci_digest" ] \
    || die "$chart stable OCI digest $pushed_digest does not match reviewed $locked_oci_digest"
  printf '%s\n' "$output"
  printf 'Stable digest: %s\n' "$pushed_digest"
}

verify_public_one() {
  local chart=$1
  local destination=$2
  local anonymous_config=$3
  lookup "$chart"
  [ "$locked_oci_digest" != "-" ] || die "$chart has no reviewed OCI identity"
  install -d -m 700 "$destination"
  local output=''
  local attempt
  local pulled=false
  for attempt in 1 2 3; do
    if output=$(HELM_REGISTRY_CONFIG="$anonymous_config" helm pull \
      "oci://$mirror_registry/$locked_name" \
      --version "$locked_version" \
      "${helm_transport_args[@]}" \
      --destination "$destination" 2>&1); then
      pulled=true
      break
    fi
    [ "$attempt" = 3 ] || sleep 5
  done
  [ "$pulled" = true ] || die "$chart is not anonymously pullable: $output"
  local pulled_digest
  pulled_digest=$(printf '%s\n' "$output" | awk '/^Digest: sha256:/ { print $2 }' | tail -1)
  [ "$pulled_digest" = "$locked_oci_digest" ] || die "$chart public OCI digest $pulled_digest does not match reviewed $locked_oci_digest"
  local archives=("$destination/$locked_name-"*.tgz)
  [ "${#archives[@]}" = 1 ] || die "anonymous pull did not produce exactly one $locked_name archive"
  printf '%s  %s\n' "$locked_archive_sha" "${archives[0]}" | sha256sum -c - >&2
}

case "${1:-}" in
  pull)
    [ "$#" = 3 ] || die "usage: $0 pull CHART DESTINATION"
    pull_locked "$2" "$3"
    ;;
  mirror)
    [ "$#" = 2 ] || die "usage: $0 mirror DESTINATION"
    anonymous_config="$2/anonymous-registry.json"
    install -d -m 700 "$2"
    printf '{"auths":{}}\n' >"$anonymous_config"
    while IFS='|' read -r chart _ _ _ oci_digest; do
      case "$chart" in ''|'#'*) continue ;; esac
      [ "$oci_digest" = "-" ] && continue
      chart_destination="$2/$chart"
      mirror_one "$chart" "$chart_destination"
    done <"$lock_file"
    while IFS='|' read -r chart _ _ _ oci_digest; do
      case "$chart" in ''|'#'*) continue ;; esac
      [ "$oci_digest" = "-" ] && continue
      public_destination="$2/public-$chart"
      verify_public_one "$chart" "$public_destination" "$anonymous_config"
    done <"$lock_file"
    ;;
  *)
    die "usage: $0 {pull CHART DESTINATION|mirror DESTINATION}"
    ;;
esac
