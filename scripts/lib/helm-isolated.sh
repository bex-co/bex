#!/usr/bin/env bash
# helm and docker with private, store-less registry credentials (w2/m289 t004,
# w2/m295 t002).
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
# The docker CLI has the same rule (cli/config: a config that ContainsAuth()
# nothing gets the platform's default store), and `docker build` asks it for
# every base-image pull, so `mock-cluster.sh operator` hung on the same locked
# keychain (w2/m295 t002). docker_isolated gives the docker CLI its own private
# config of the same shape, plus what a bare DOCKER_CONFIG would lose: the
# active context (currentContext, with the contexts metadata linked in) and the
# CLI plugins (buildx), so the build still reaches the daemon it reached before.
#
#   . scripts/lib/helm-isolated.sh
#   helm upgrade --install ...                    # the function below
#   helm_isolated bash scripts/some-helm-user.sh  # a child script's helm too
#   docker_isolated docker build ...              # one docker command

# The placeholder auths entry. .invalid is reserved (RFC 2606), so the entry
# can never match a real registry; its presence is what stops native-store
# detection.
HELM_ISOLATED_CONFIG='{"auths":{"bex-anonymous.invalid":{}}}'

# isolated_config_dir DIR CONFIG — create DIR owner-only, refuse one another
# user owns (its config.json would decide which credentials are sent), write
# CONFIG to DIR/config.json when it differs, and print DIR. One fixed per-user
# path rather than a mktemp per call: the scripts call helm many times, and
# nothing would clean a dir per call up.
isolated_config_dir() {
  local dir="${1%/}" config=$2
  mkdir -p "$dir" || return 1
  [ -O "$dir" ] || { echo "error: $dir is not owned by $(id -un); refusing it as a registry config" >&2; return 1; }
  chmod 700 "$dir" || return 1
  if [ "$(cat "$dir/config.json" 2>/dev/null)" != "$config" ]; then
    printf '%s\n' "$config" >"$dir/config.json.$$" && mv "$dir/config.json.$$" "$dir/config.json" || return 1
  fi
  printf '%s\n' "$dir"
}

# helm_isolated_dir — print helm's private config directory.
helm_isolated_dir() {
  isolated_config_dir "${BEX_HELM_ISOLATED_DIR:-${TMPDIR:-/tmp}/bex-helm-registry-$(id -u)}" "$HELM_ISOLATED_CONFIG"
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

# docker_isolated_dir — print the docker CLI's private config directory: the
# placeholder auths entry, no credsStore/credHelpers, and the caller's active
# context. `docker context show` reads only the caller's config (and
# DOCKER_CONTEXT/DOCKER_HOST); it never calls a credential helper. contexts/
# and cli-plugins/ are symlinks into the caller's own config dir, so a context
# or plugin added later is seen without re-copying TLS material anywhere.
docker_isolated_dir() {
  local real="${DOCKER_CONFIG:-$HOME/.docker}" dir ctx config sub
  real="${real%/}"
  ctx=$(command docker context show 2>/dev/null) || ctx=
  config='{"auths":{"bex-anonymous.invalid":{}}'
  # Context names are [a-zA-Z0-9][a-zA-Z0-9_.+-]+ (docker validates them), so
  # they need no JSON escaping; "default" is what an absent field means.
  case $ctx in '' | default) ;; *) config="$config,\"currentContext\":\"$ctx\"" ;; esac
  dir=$(isolated_config_dir "${BEX_DOCKER_ISOLATED_DIR:-${TMPDIR:-/tmp}/bex-docker-config-$(id -u)}" "$config}") || return 1
  [ "$dir" != "$real" ] || { echo "error: DOCKER_CONFIG is already the isolated dir $dir" >&2; return 1; }
  for sub in contexts cli-plugins; do
    if [ -e "$real/$sub" ]; then
      ln -sfn "$real/$sub" "$dir/$sub" || return 1
    else
      rm -f "$dir/$sub" 2>/dev/null || true
    fi
  done
  printf '%s\n' "$dir"
}

# docker_isolated CMD... — run CMD with DOCKER_CONFIG at the private dir. Only
# CMD sees it; the caller's environment is unchanged.
docker_isolated() {
  local dir
  dir=$(docker_isolated_dir) || return 1
  DOCKER_CONFIG="$dir" "$@"
}
