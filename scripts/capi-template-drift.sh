#!/usr/bin/env bash
# Reports drift between the CAPI machine templates in this repository and the
# ones the management cluster actually uses. Read-only: `kubectl diff` runs a
# server-side dry run, so defaulted fields do not show as drift and nothing is
# written.
#
# A template edited in Git but never applied changes nothing: CAPI reads the
# live template only when it creates a machine. That is how the sandbox pool
# kept downloading gVisor from a dead `latest` URL long after the pinned,
# digest-checked install was committed (w9/074).
#
# Usage:
#   BEX_KUBE_CONTEXT=hetzner-prod scripts/capi-template-drift.sh [manifest ...]
# Exit: 0 no drift, 1 drift found, 2 a template could not be compared.
set -euo pipefail
cd "$(dirname "$0")/.."

KUBE=(kubectl)
if [ -n "${BEX_KUBE_CONTEXT:-}" ]; then
  KUBE=(kubectl --context "$BEX_KUBE_CONTEXT")
fi
if [ "$#" -gt 0 ]; then
  manifests=("$@")
else
  manifests=(infra/clusterapi/overlays/hetzner-caph/cluster.yaml infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml)
fi
command -v python3 >/dev/null || { echo "error: python3 is required" >&2; exit 2; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

python3 - "$tmp" "${manifests[@]}" <<'PY'
import os, sys, yaml
out, paths = sys.argv[1], sys.argv[2:]
kinds = {"KubeadmConfigTemplate", "HCloudMachineTemplate"}
n = 0
for path in paths:
    for doc in yaml.safe_load_all(open(path)):
        if doc and doc.get("kind") in kinds:
            n += 1
            name = f'{n:03d}-{doc["kind"]}-{doc["metadata"]["name"]}.yaml'
            with open(os.path.join(out, name), "w") as f:
                yaml.safe_dump(doc, f)
if n == 0:
    sys.exit("error: no machine templates found in " + " ".join(paths))
PY

drift=0
errors=0
for file in "$tmp"/*.yaml; do
  label="$(basename "$file" .yaml | cut -d- -f2-)"
  set +e
  "${KUBE[@]}" diff -f "$file" >"$file.diff" 2>"$file.err"
  rc=$?
  set -e
  case "$rc" in
    0) echo "ok     $label" ;;
    1)
      echo "DRIFT  $label"
      sed 's/^/       /' "$file.diff"
      drift=1
      ;;
    *)
      echo "ERROR  $label: $(head -c 300 "$file.err")" >&2
      errors=1
      ;;
  esac
done

if [ "$errors" -ne 0 ]; then
  exit 2
fi
if [ "$drift" -ne 0 ]; then
  echo "Live machine templates differ from Git. Apply the repo template (kubectl apply -f <file>, templates only), then replace machines surge-first; see infra/clusterapi/overlays/hetzner-caph/README.md." >&2
  exit 1
fi
echo "no drift"
