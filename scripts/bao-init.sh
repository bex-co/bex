#!/usr/bin/env bash
# Out-of-band init/unseal of OpenBao (docs/ADR013-secrets.md#3), idempotent, plus the
# tenants/ KV v2 mount (docs/ADR013-secrets.md#4).
#
# First run: initializes OpenBao (5 Shamir shares / 3 threshold — the OpenBao
# default), then writes BAO_UNSEAL_KEY_1/2/3 + BAO_ROOT_TOKEN into .env
# (gitignored — never printed to stdout/logs, same convention as
# auth-secrets.sh). Every run after that reads those same keys back out of
# .env and unseals — including after a pod restart, which always comes back
# sealed (no auto-unseal, by design: docs/ADR013-secrets.md#3).
#
# HA (server.ha.replicas >= 2, prod): init runs ONCE on the ordinal-first pod
# (the raft leader that forms the cluster); unseal runs on EVERY pod. Shamir
# seal state is per-node — each pod comes back sealed on restart and must be
# unsealed with the same three keys — and the `openbao` Service round-robins
# across sealed+unsealed members, so a Service-targeted unseal is unreliable
# in HA. Endpoint resolution (bao-endpoints.sh, shared with bao-k8s-auth.sh)
# reaches each pod directly. Single-node and the BAO_ADDR (off-cluster) paths
# operate on the one reachable endpoint and are unaffected.
#
# Usage: scripts/bao-init.sh             # init (first run) / unseal / ensure tenants/ mount
#        BAO_ADDR=http://... ...         # use an already-reachable OpenBao endpoint
#        DRY_RUN=1 scripts/bao-init.sh   # print intent, change nothing
# Requires: kubectl (respects $KUBECONFIG) unless BAO_ADDR is set, curl, yq v4.
set -euo pipefail

NS=secrets

# set_env_var NAME VALUE — set/replace a key in .env without ever printing VALUE.
# Pure function (no cd, no .env read at define-time) so it is unit-testable via
# `source scripts/bao-init.sh` (the main guard below keeps sourcing side-effect-free).
set_env_var() {
  local name="$1" val="$2"
  if [ -f .env ] && grep -q "^${name}=" .env; then
    awk -F= -v n="$name" -v v="$val" 'BEGIN{OFS="="} $1==n{print n,v; next} {print}' .env >.env.tmp
    mv .env.tmp .env
  else
    printf '%s=%s\n' "$name" "$val" >>.env
  fi
}

main() {
  # Resolve the script dir to an absolute path BEFORE cd, so the source below
  # works from any cwd (the old relative dirname was stale after the cd, and
  # broke `bash bao-init.sh` from inside scripts/).
  local here; here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  cd "$here/.."
  # shellcheck disable=SC1091 — defines bao_resolve_endpoints / bao_cleanup_forwards
  source "$here/bao-endpoints.sh"

  # Load .env for the unseal keys + root token, but only when they aren't
  # already supplied via the environment — otherwise blank template lines
  # (BAO_UNSEAL_KEY_1=, as `cp .env.template .env` leaves) would clobber real
  # values the caller exported. Mirrors bao-k8s-auth.sh's guarded load.
  if [ -z "${BAO_ROOT_TOKEN:-}" ] && [ -f .env ]; then
    set -a
    # shellcheck disable=SC1091
    source ./.env
    set +a
  fi

  if [ "${DRY_RUN:-}" = "1" ]; then
    echo "would init (if needed), unseal every pod (if needed), ensure tenants/ kv-v2 mount"
    exit 0
  fi

  # Install the cleanup trap BEFORE resolving: bao_resolve_endpoints launches
  # port-forwards and can itself exit non-zero partway through, which would
  # otherwise orphan them.
  trap bao_cleanup_forwards EXIT
  bao_resolve_endpoints "$NS"
  urls=("${bao_urls[@]}")
  leader="$bao_leader"

  # --- 1. init (once, on the leader) -------------------------------------------
  initialized="$(curl -sf "$leader/v1/sys/seal-status" | yq '.initialized' -)"
  if [ "$initialized" != "true" ]; then
    # First init is a one-time, MANUAL operator step: it mints the ONLY copy of
    # the unseal keys + root token into .env. Refuse to do it implicitly — e.g.
    # from deploy.yml, where .env lives on an ephemeral runner and would be
    # discarded, permanently sealing the store on the next restart. Gate it
    # behind an explicit opt-in (docs/ADR013-secrets.md "Prod deploy path").
    if [ "${BAO_ALLOW_INIT:-}" != "1" ]; then
      echo "error: $leader is not initialized and BAO_ALLOW_INIT != 1 — refusing to auto-init." >&2
      echo "  A first init writes the unseal keys + root token to .env only; losing them (e.g. on" >&2
      echo "  an ephemeral CI runner) bricks the store. Run the one-time manual init instead:" >&2
      echo "    BAO_ALLOW_INIT=1 bash scripts/bao-init.sh   (against the prod kubeconfig)" >&2
      echo "  then push the keys with scripts/gh-secrets.sh (docs/ADR013-secrets.md)." >&2
      exit 1
    fi
    echo "==> initializing (5 shares / 3 threshold)"
    init_resp="$(curl -sf -X PUT "$leader/v1/sys/init" -d '{"secret_shares":5,"secret_threshold":3}')"
    set_env_var BAO_UNSEAL_KEY_1 "$(printf '%s' "$init_resp" | yq '.keys_base64[0]' -)"
    set_env_var BAO_UNSEAL_KEY_2 "$(printf '%s' "$init_resp" | yq '.keys_base64[1]' -)"
    set_env_var BAO_UNSEAL_KEY_3 "$(printf '%s' "$init_resp" | yq '.keys_base64[2]' -)"
    set_env_var BAO_ROOT_TOKEN "$(printf '%s' "$init_resp" | yq '.root_token' -)"
    echo "initialized — unseal keys + root token written to .env (never printed)"
    set -a
    source ./.env
    set +a
  else
    echo "already initialized"
  fi

  for name in BAO_UNSEAL_KEY_1 BAO_UNSEAL_KEY_2 BAO_UNSEAL_KEY_3 BAO_ROOT_TOKEN; do
    [ -n "${!name:-}" ] || { echo "error: $name is missing from .env/env (delete the OpenBao PVC to re-init, or restore .env from backup)" >&2; exit 1; }
  done

  # --- 2. unseal (every endpoint) ----------------------------------------------
  # Fetch seal-status once per endpoint and read both .initialized and .sealed
  # off that one body. On a fresh HA cluster the non-leader pods report
  # initialized=false until service_registration peers them onto the leader's
  # raft cluster — so only those wait; the leader (and the single BAO_ADDR
  # endpoint) is already initialized. On a restart every pod is already
  # initialized and the wait is skipped entirely.
  for u in "${urls[@]}"; do
    status="$(curl -sf "$u/v1/sys/seal-status" || true)"
    initialized="$(printf '%s' "$status" | yq '.initialized' -)"
    if [ "$initialized" != "true" ] && [ "$u" != "$leader" ]; then
      # A fresh follower reports initialized=false until it joins the leader's
      # raft cluster via retry_join (prod overlay's raft config). Poll through
      # that window; tolerate a transient drop (|| true) so set -e doesn't abort
      # the very retry this loop exists to do.
      for _ in $(seq 1 30); do
        status="$(curl -sf "$u/v1/sys/seal-status" || true)"
        initialized="$(printf '%s' "$status" | yq '.initialized' -)"
        [ "$initialized" = "true" ] && break
        sleep 2
      done
      [ "$initialized" = "true" ] || { echo "error: $u never became initialized (raft join to the leader failed?)" >&2; exit 1; }
    fi

    sealed="$(printf '%s' "$status" | yq '.sealed' -)"
    if [ "$sealed" = "true" ]; then
      echo "==> unsealing $u"
      # Keys go in via stdin (-d @-), never argv, so they can't leak through
      # ps / /proc/<pid>/cmdline (the rule gh-secrets.sh's header states).
      printf '{"key":"%s"}' "$BAO_UNSEAL_KEY_1" | curl -sf -X PUT "$u/v1/sys/unseal" -d @- >/dev/null
      printf '{"key":"%s"}' "$BAO_UNSEAL_KEY_2" | curl -sf -X PUT "$u/v1/sys/unseal" -d @- >/dev/null
      result="$(printf '{"key":"%s"}' "$BAO_UNSEAL_KEY_3" | curl -sf -X PUT "$u/v1/sys/unseal" -d @-)"
      [ "$(printf '%s' "$result" | yq '.sealed' -)" = "false" ] || { echo "error: $u still sealed after presenting 3 keys" >&2; exit 1; }
      echo "unsealed $u"
    else
      echo "already unsealed: $u"
    fi
  done

  # --- 3. tenants/ kv-v2 mount (once, cluster-wide via the leader) --------------
  # The root token goes in via a curl --config file on stdin, never argv, so it
  # can't leak through ps / /proc/<pid>/cmdline.
  local tokhdr; tokhdr="$(printf 'header = "X-Vault-Token: %s"' "$BAO_ROOT_TOKEN")"
  mounts="$(printf '%s' "$tokhdr" | curl -sf --config - "$leader/v1/sys/mounts")"
  if [ "$(printf '%s' "$mounts" | yq 'has("tenants/")' -)" = "true" ]; then
    echo "tenants/ mount exists"
  else
    printf '%s' "$tokhdr" | curl -sf --config - -X POST "$leader/v1/sys/mounts/tenants" \
      -d '{"type":"kv","options":{"version":"2"}}' >/dev/null
    echo "created tenants/ kv-v2 mount"
  fi
}

# Run only when executed, not when sourced (for unit tests of set_env_var).
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  main "$@"
fi
