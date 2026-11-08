#!/usr/bin/env bash
# helm with private, store-less registry credentials (w2/m289 t004).
#
# Every chart the local scripts install is public, yet helm v4 still asks a
# credential helper before an OCI pull: its registry client (pkg/registry,
# oras-go credentials) reads $HELM_REGISTRY_CONFIG, falls back to the Docker
# config ($DOCKER_CONFIG/config.json, else ~/.docker/config.json), and — the
# part that defeated pointing either one at an empty file — when a config
# has NO auths, credsStore or credHelpers it "detects the default native
# store" and calls docker-credential-osxkeychain anyway. A locked keychain
# makes that call block forever, so the whole bring-up hung.
#
# The isolation therefore needs BOTH paths pointed at one private file that
# counts as "configured" (a placeholder auths entry) while naming no store:
# lookups then resolve from the file, find nothing, and pull anonymously.
# HOME, the repository cache and the kubeconfig are left alone, so
# `helm repo add` state and KUBECONFIG keep working.
#
#   . scripts/lib/helm-isolated.sh
#   helm upgrade --install ...                 # the function below
#   helm_isolated bash scripts/some-helm-user.sh  # a child script's helm too

# The placeholder config. .invalid is reserved (RFC 2606), so the entry can
# never match a real registry; its presence is what stops native-store
# detection.
HELM_ISOLATED_CONFIG='{"auths":{"bex-anonymous.invalid":{}}}'

# helm_isolated_dir — print the private config directory, (re)writing its
# config.json. One fixed per-user path rather than a mktemp per call: the
# scripts call helm many times, and nothing would clean a dir per call up.
helm_isolated_dir() {
  local dir="${BEX_HELM_ISOLATED_DIR:-${TMPDIR:-/tmp}/bex-helm-registry-$(id -u)}"
  dir="${dir%/}"
  mkdir -p "$dir" || return 1
  # Refuse a directory someone else owns: its config.json would decide which
  # credentials helm sends.
  [ -O "$dir" ] || { echo "error: $dir is not owned by $(id -un); refusing it as helm's registry config" >&2; return 1; }
  chmod 700 "$dir" || return 1
  if [ "$(cat "$dir/config.json" 2>/dev/null)" != "$HELM_ISOLATED_CONFIG" ]; then
    printf '%s\n' "$HELM_ISOLATED_CONFIG" >"$dir/config.json.$$" && mv "$dir/config.json.$$" "$dir/config.json" || return 1
  fi
  printf '%s\n' "$dir"
}

# helm_isolated CMD... — run CMD with helm's registry and Docker config both
# pointed at the private file. Only CMD sees them; the caller's environment
# (and its docker CLI) is unchanged.
helm_isolated() {
  local dir
  dir=$(helm_isolated_dir) || return 1
  HELM_REGISTRY_CONFIG="$dir/config.json" DOCKER_CONFIG="$dir" "$@"
}

helm() { helm_isolated command helm "$@"; }
