#!/usr/bin/env bash
# Start the legacy loopback-only host OpenSandbox server (Docker runtime) for
# local development on :8077. Production uses the GitOps-owned cluster config.
# Pre-pulls the runtime images so the first sandbox create doesn't block/timeout.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "pre-pulling OpenSandbox runtime images (cached after first time)..."
docker pull -q opensandbox/execd:v1.0.16@sha256:af7b55c861926c1304371c4578007fbaa424538219154a6a49a5d636217d2a3a || true
docker pull -q opensandbox/egress:v1.0.12@sha256:ddf92e8f303c5715c8bbe8f346af80d7f14efaef6d760add92bf50e558b06c2a || true

echo "starting opensandbox-server on :8077 (insecure/local) ..."
export OPENSANDBOX_INSECURE_SERVER=YES
exec uvx --from opensandbox-server==0.2.2 opensandbox-server --config deploy/opensandbox/sandbox.toml
