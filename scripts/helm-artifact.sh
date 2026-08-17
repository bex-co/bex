#!/usr/bin/env bash
# Fetch and authenticate production Helm inputs against repository-reviewed
# digests. Argo never resolves the mutable upstream repositories: deploy.yml
# mirrors the verified archives to GHCR and Applications select the immutable
# OCI manifest digests committed in deploy/helm-artifacts.lock.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
lock_file="$repo_root/deploy/helm-artifacts.lock"
mirror_registry=${BEX_HELM_MIRROR:-ghcr.io/bex-co/bex-charts}

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
  output=$(helm push "$archive" "oci://$mirror_registry" 2>&1) || die "failed to mirror $chart: $output"
  local pushed_digest
  pushed_digest=$(printf '%s\n' "$output" | awk '/^Digest: sha256:/ { print $2 }' | tail -1)
  [ "$pushed_digest" = "$locked_oci_digest" ] || die "$chart OCI digest $pushed_digest does not match reviewed $locked_oci_digest"
  printf '%s\n' "$output"
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
