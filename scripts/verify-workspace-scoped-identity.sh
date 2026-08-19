#!/usr/bin/env bash
# Inspect workspace-scoped registry/static identity on the current kubeconfig
# (w2/m75, docs/ADR073). Read-only. Never prints kubeconfig or Secret data.
#
# Usage:
#   KUBECONFIG=infra/local/bex.kubeconfig bash scripts/verify-workspace-scoped-identity.sh
#   NAME=collide WS_A=tea-aaa WS_B=tea-bbb bash scripts/verify-workspace-scoped-identity.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KUBECONFIG="${KUBECONFIG:-$ROOT/infra/local/bex.kubeconfig}"
if [[ ! -f "$KUBECONFIG" ]]; then
  echo "missing kubeconfig at $KUBECONFIG (not printed)" >&2
  exit 1
fi
kc() { kubectl --kubeconfig "$KUBECONFIG" "$@"; }

ZOT_NS="${BEX_REGISTRY_NS:-bex-registry}"
NAME="${NAME:-collide}"
WS_A="${WS_A:-}"
WS_B="${WS_B:-}"

echo "== nodes =="
if ! kc get nodes --request-timeout=8s >/dev/null; then
  echo "cluster not reachable" >&2
  exit 1
fi
kc get nodes --request-timeout=8s

echo "== Apps (name, namespace, workspace, tombstone, staticPrefix) =="
kc get apps.app.bex.co -A -o custom-columns=\
NS:.metadata.namespace,NAME:.metadata.name,WS:.metadata.labels.app\\.bex\\.co/workspace,TOMBSTONE:.metadata.annotations.app\\.bex\\.co/identity-tombstone,PREFIX:.status.staticPrefix,IMAGE:.status.image

echo "== pull Secrets matching $NAME =="
kc get secrets -A --field-selector type=kubernetes.io/dockerconfigjson -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name \
  | awk -v n="$NAME" 'NR==1 || $2 ~ ("reg-pull-.*" n "$") || $2 == "reg-pull-" n'

echo "== zot htpasswd usernames (names only) =="
if ! kc -n "$ZOT_NS" get secret zot-htpasswd --request-timeout=8s >/dev/null 2>&1; then
  echo "zot-htpasswd absent in $ZOT_NS (per-App registry not enabled on this cluster); skipping ACL checks"
else
  kc -n "$ZOT_NS" get secret zot-htpasswd -o jsonpath='{.data.htpasswd}' | base64 --decode | awk -F: '{print $1}'

  if [[ -n "$WS_A" && -n "$WS_B" ]]; then
    echo "== expect disjoint repos $WS_A/$NAME and $WS_B/$NAME in zot-config =="
    cfg="$(kc -n "$ZOT_NS" get secret zot-config -o jsonpath='{.data.config\.json}' | base64 --decode)"
    python3 - "$cfg" "$WS_A/$NAME" "$WS_B/$NAME" <<'PY'
import json, sys
cfg = json.loads(sys.argv[1])
repos = cfg.get("http", {}).get("accessControl", {}).get("repositories", {})
a, b = sys.argv[2], sys.argv[3]
missing = [r for r in (a, b) if r not in repos]
if missing:
    raise SystemExit("missing repos: " + ", ".join(missing))
if a == b:
    raise SystemExit("workspace ids collided")
print("ok:", a, "and", b)
PY
  fi
fi

echo "verify-workspace-scoped-identity: ok"
