#!/usr/bin/env bash
# Resolve BEX_BOOTSTRAP_CLIENT_SECRET for a verify script that seeds the
# bex-bootstrap OAuth2 client on the current kubeconfig cluster's Hydra through
# scripts/auth-bootstrap-client.sh.
#
# bex-bootstrap is a permanent platform client and that seed replaces the
# stored client outright, so the value resolved here becomes the cluster's live
# secret until the next deploy seeds it again. It must therefore be the secret
# the cluster already holds, or one nobody can reconstruct. In order:
#   1. an exported BEX_BOOTSTRAP_CLIENT_SECRET;
#   2. .env's BEX_BOOTSTRAP_CLIENT_SECRET, the value deploy seeds, so a run
#      against a deployed cluster leaves the secret as it was;
#   3. on a cluster whose apiserver is on loopback (the local kind/CAPD
#      cluster), a fresh 256-bit random value.
# Anything else is refused: a verify run never invents the secret of a cluster
# other people use, and never derives one from the clock or the run.
#
# This file is sourced. Call sites run from the repository root, before the
# script changes anything on the cluster:
#   . scripts/lib/verify-bootstrap-secret.sh && bex_verify_bootstrap_secret || exit 1
# Only that one key is read from .env (through bex_load_env, in a subshell), so
# the rest of the file never reaches the caller's environment or the host-run
# bex-api it starts.

bex_verify_bootstrap_secret() {
  local secret="${BEX_BOOTSTRAP_CLIENT_SECRET:-}" server
  if [ -z "$secret" ] && [ -f .env ]; then
    secret="$(. scripts/lib/env-file.sh && bex_load_env ./.env && printf '%s' "${BEX_BOOTSTRAP_CLIENT_SECRET:-}")" || return 1
  fi
  if [ -z "$secret" ]; then
    server="$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null || true)"
    if ! [[ "$server" =~ ^https?://(127\.[0-9]+\.[0-9]+\.[0-9]+|localhost|0\.0\.0\.0|\[::1\])(:[0-9]+)?(/.*)?$ ]]; then
      echo "error: BEX_BOOTSTRAP_CLIENT_SECRET is not set (environment or .env) and the current kubeconfig cluster is not local." >&2
      echo "       Seeding bex-bootstrap replaces that platform client's live secret: export the secret this cluster was deployed with, or point KUBECONFIG at the local mock cluster." >&2
      return 1
    fi
    secret="$(openssl rand -hex 32)" || return 1
  fi
  export BEX_BOOTSTRAP_CLIENT_SECRET="$secret"
}
