#!/usr/bin/env bash
# Exercise the real reconciliation and import helpers with file-backed node
# images. No Docker daemon or cluster required; command failures stay distinct
# from missing images, and imports must pass the real post-import verification.
set -euo pipefail
cd "$(dirname "$0")/.."

eval "$(awk '
  /^AGENT_NODE_IMAGES=\(/ {keep=1}
  keep {print}
  /^agent_reconcile_node_images\(\)/ {in_fn=1}
  in_fn && /^}$/ {exit}
' scripts/dev-env.sh)"

fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
# shellcheck disable=SC2034 # consumed by the helpers loaded through eval above
AGENT_NODE_IMAGES=(bex-probe:dev)

kubectl() { printf 'fresh\nmatching\n'; }
docker() {
  case "$1" in
    image | save) printf 'sha256:desired\n' ;;
    exec)
      shift
      [ "$1" != -i ] || shift
      local node="$1"
      shift
      case "$1" in
        crictl)
          printf '%s\n' "$node" >>"$fixture/inspections"
          if [ -f "$fixture/inspect-error" ]; then
            echo 'container runtime unavailable' >&2
            return 1
          fi
          if [ ! -f "$fixture/$node" ]; then
            echo 'No such image' >&2
            return 1
          fi
          cat "$fixture/$node"
          ;;
        ctr)
          case "$5" in
            rm) rm -f "$fixture/$node" ;;
            import)
              printf '%s\n' "$node" >>"$fixture/imports"
              cat >"$fixture/$node"
              [ ! -f "$fixture/import-error" ] || return 1
              if [ -f "$fixture/mismatch" ]; then
                printf 'sha256:wrong\n' >"$fixture/$node"
              fi
              ;;
            *) echo "unexpected ctr command: $*" >&2; return 1 ;;
          esac
          ;;
        *) echo "unexpected exec command: $*" >&2; return 1 ;;
      esac
      ;;
    *) echo "unexpected docker command: $*" >&2; return 1 ;;
  esac
}

reset_fixture() {
  rm -f "$fixture"/*
  printf 'sha256:desired\n' >"$fixture/matching"
  : >"$fixture/imports"
  : >"$fixture/inspections"
}

# Model the caller: rollout is reachable only after reconciliation succeeds.
reconcile_then_rollout() {
  agent_reconcile_node_images || return 1
  touch "$fixture/rolled-out"
}

reset_fixture
if ! reconcile_then_rollout; then
  echo 'FAIL: missing image must import and allow rollout' >&2
  exit 1
fi
[ "$(cat "$fixture/fresh")" = sha256:desired ]
[ "$(cat "$fixture/imports")" = fresh ] # matching node must not import
[ "$(cat "$fixture/inspections")" = "$(printf 'fresh\nfresh\nmatching')" ]
[ -f "$fixture/rolled-out" ]
echo 'ok: missing image imports, verifies, and rolls out; matching image skips'

for failure in inspect-error import-error mismatch; do
  reset_fixture
  touch "$fixture/$failure"
  if reconcile_then_rollout >"$fixture/output" 2>&1; then
    echo "FAIL: $failure must stop reconciliation" >&2
    exit 1
  fi
  [ ! -f "$fixture/rolled-out" ]
  if [ "$failure" = inspect-error ]; then
    [ ! -s "$fixture/imports" ]
    grep -q 'container runtime unavailable' "$fixture/output"
  else
    [ "$(cat "$fixture/imports")" = fresh ]
    grep -q 'error:.*bex-probe:dev.*fresh' "$fixture/output"
  fi
  echo "ok: $failure stops before rollout"
done
