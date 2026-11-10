#!/usr/bin/env bash
# Production's Helm chart pins for the local scripts (w2/m289 t001, w2/m295 t003).
#
# deploy/helm-artifacts.lock is the one place production's chart pins live:
# `chart|version|upstream repository|archive sha256|OCI manifest sha256`. The
# local mock (scripts/mock-cluster.sh) and every dev-N (scripts/dev-env.sh)
# install the same archives production runs, verified against the same
# sha256, so a local cluster never drifts to a different chart version the way
# dev-env's old cloudnative-pg 0.26.0 fallback did beside production's 0.29.1.
#
#   . scripts/lib/helm-pinned.sh
#   chart=$(fetch_pinned_chart cloudnative-pg "$dir") && helm upgrade --install ... "$chart"
#
# Callers run from the repo root and define `helm` (scripts/lib/helm-isolated.sh
# keeps a locked keychain from hanging the pull).

# helm_pin CHART prints "VERSION REPO SHA256" for CHART from the lock.
helm_pin() {
  awk -F'|' -v chart="$1" '$1 == chart { print $2, $3, $4; exit }' deploy/helm-artifacts.lock
}

sha256_of() { shasum -a 256 "$1" | awk '{print $1}'; }

# fetch_pinned_chart CHART DIR pulls CHART's locked archive into DIR, refuses
# it unless its sha256 matches the lock, and prints the archive's path.
fetch_pinned_chart() {
  local chart=$1 dir=$2 version repo sha got
  read -r version repo sha <<<"$(helm_pin "$chart")" || true
  if [ -z "${sha:-}" ]; then
    echo "error: deploy/helm-artifacts.lock has no $chart|VERSION|REPO|SHA256 line" >&2
    return 1
  fi
  if ! helm pull "$chart" --repo "$repo" --version "$version" --destination "$dir" >/dev/null; then
    echo "error: could not fetch the $chart $version chart from $repo" >&2
    return 1
  fi
  got=$(sha256_of "$dir/$chart-$version.tgz")
  if [ "$got" != "$sha" ]; then
    echo "error: $chart $version archive sha256 $got does not match deploy/helm-artifacts.lock ($sha)" >&2
    return 1
  fi
  printf '%s\n' "$dir/$chart-$version.tgz"
}
