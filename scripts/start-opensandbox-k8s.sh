#!/usr/bin/env bash
# Start the legacy loopback-only OpenSandbox KUBERNETES runtime on :8078 for
# local development, scheduling into vcluster "acme". Production uses the
# GitOps-owned cluster config. Requires: OrbStack k8s up, vcluster acme running, the
# opensandbox-controller installed in the vcluster, and the vcluster kubeconfig at
# deploy/opensandbox/vcluster-acme.kubeconfig (see README / scripts).
set -euo pipefail
cd "$(dirname "$0")/.."

echo "starting opensandbox-server (kubernetes runtime) on :8078 ..."
export OPENSANDBOX_INSECURE_SERVER=YES
exec uvx --from opensandbox-server==0.2.2 opensandbox-server --config deploy/opensandbox/sandbox-k8s.toml
