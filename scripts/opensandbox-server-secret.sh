#!/usr/bin/env bash
set -euo pipefail

# Mirror the internal bex-api control-plane bearer into the lifecycle server's
# namespace without printing it or committing it to Git. The source Secret is
# the canonical credential installed by cp-token-secret.sh; this copy exists so
# opensandbox-system cannot read any Secret from bex-system at runtime.

source_namespace="${BEX_SYSTEM_NAMESPACE:-bex-system}"
target_namespace="${BEX_OPENSANDBOX_NAMESPACE:-opensandbox-system}"
source_secret="${BEX_CP_SECRET:-bex-control-plane}"
target_secret="${BEX_OPENSANDBOX_AUTH_SECRET:-opensandbox-server-auth}"

command -v kubectl >/dev/null || { echo "missing required command: kubectl" >&2; exit 1; }

kubectl get namespace "$target_namespace" >/dev/null
token_file="$(mktemp)"
trap 'rm -f "$token_file"' EXIT
chmod 600 "$token_file"

kubectl -n "$source_namespace" get secret "$source_secret" \
  -o jsonpath='{.data.token}' | base64 --decode >"$token_file"

if ! grep -Eq '^[A-Za-z0-9._~-]{32,256}$' "$token_file"; then
  echo "source control-plane token has an unsupported format" >&2
  exit 1
fi

kubectl -n "$target_namespace" create secret generic "$target_secret" \
  --from-file=cp_token="$token_file" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# The hand-applied pre-m35 Secret also carried one shared lifecycle api_key.
# Multi-tenant mode no longer consumes it; remove the dormant credential during
# convergence so rollback or configuration drift cannot silently restore the
# cluster-wide shared-auth path. The value is tested for presence but never
# printed.
if kubectl -n "$target_namespace" get secret "$target_secret" \
  -o jsonpath='{.data.api_key}' | grep -q .; then
  kubectl -n "$target_namespace" patch secret "$target_secret" --type=json \
    --patch='[{"op":"remove","path":"/data/api_key"}]' >/dev/null
fi

# The server reads the token once, in an init container, so new pods are needed
# only when the Secret changed. Stamp the Secret's version on the pod template:
# an unchanged stamp is a no-op and a changed one rolls the Deployment. The
# server is one Recreate replica, so a restart on every deploy took the sandbox
# API down each time for a credential that had not changed (w8/248). A stamp,
# unlike comparing versions around the apply, still converges when an earlier
# run changed the Secret and died before it restarted the server.
if kubectl -n "$target_namespace" get deployment/opensandbox-server >/dev/null 2>&1; then
  secret_version="$(kubectl -n "$target_namespace" get secret "$target_secret" \
    -o jsonpath='{.metadata.resourceVersion}')"
  [[ "$secret_version" =~ ^[0-9]+$ ]] \
    || { echo "could not read the version of $target_secret" >&2; exit 1; }
  kubectl -n "$target_namespace" patch deployment/opensandbox-server --type=merge \
    --patch "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"bex.co/cp-token-secret-version\":\"${secret_version}\"}}}}}" >/dev/null
fi

echo "installed $target_secret in namespace $target_namespace"
