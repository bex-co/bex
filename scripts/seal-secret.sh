#!/usr/bin/env bash
# Seal a platform/infra credential into a committable SealedSecret manifest
# (w1/m7 t003, docs/ADR016-sealed-secrets.md). Encrypts with the running sealed-secrets
# controller's public key, so the output is safe to commit to git — only this
# cluster's controller can decrypt it, and only into the namespace/name it was
# sealed for.
#
# Usage:
#   scripts/seal-secret.sh <namespace> <name> KEY=VALUE [KEY=VALUE...]
#   scripts/seal-secret.sh -f secret.yaml            # seal an existing Secret manifest
#
# Writes the SealedSecret YAML to stdout — redirect it into deploy/gitops (e.g.
#   scripts/seal-secret.sh default hetzner hcloud="$HCLOUD_TOKEN" \
#     > deploy/gitops/base/sealed/hetzner.sealedsecret.yaml
# ). The plaintext value is NEVER written to disk or logged.
#
# Requires: kubeseal, kubectl (pointed at the cluster whose controller will
# decrypt these). The controller is name `sealed-secrets` in kube-system
# (deploy/gitops/base/sealed-secrets.yaml).
set -euo pipefail

CONTROLLER_NS=kube-system
CONTROLLER_NAME=sealed-secrets

command -v kubeseal >/dev/null || { echo "error: kubeseal not found — https://github.com/bitnami-labs/sealed-secrets/releases" >&2; exit 1; }
command -v kubectl >/dev/null || { echo "error: kubectl not found" >&2; exit 1; }

seal() { kubeseal --controller-namespace "$CONTROLLER_NS" --controller-name "$CONTROLLER_NAME" --format yaml; }

if [ "${1:-}" = "-f" ]; then
  [ -n "${2:-}" ] || { echo "error: -f needs a Secret manifest path" >&2; exit 1; }
  seal <"$2"
  exit 0
fi

NS="${1:-}"
NAME="${2:-}"
shift 2 || { echo "usage: scripts/seal-secret.sh <namespace> <name> KEY=VALUE... | -f secret.yaml" >&2; exit 1; }
[ -n "$NS" ] && [ -n "$NAME" ] && [ "$#" -gt 0 ] || { echo "usage: scripts/seal-secret.sh <namespace> <name> KEY=VALUE... | -f secret.yaml" >&2; exit 1; }

args=(create secret generic "$NAME" -n "$NS" --dry-run=client -o yaml)
for kv in "$@"; do
  case "$kv" in
    *=*) args+=(--from-literal="$kv") ;;
    *)   echo "error: expected KEY=VALUE, got '$kv'" >&2; exit 1 ;;
  esac
done

kubectl "${args[@]}" | seal
