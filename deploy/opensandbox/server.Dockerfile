# OpenSandbox lifecycle server image (pillar 5, ADR042 D1 / w3/m32 t001).
#
# No official opensandbox-server image exists upstream, so this packages the
# PyPI `opensandbox-server` (the FastAPI lifecycle API the host start scripts run
# via `uvx --from opensandbox-server opensandbox-server`) into a cluster-runnable
# image. The in-cluster Deployment (t002) mounts the cluster TOML
# (sandbox-cluster.toml) and the BatchSandbox template ConfigMap, and runs with
# an in-cluster ServiceAccount (no kubeconfig_path).
#
# Build (pushed to the in-cluster Zot by CI, not the host):
#   docker build -f deploy/opensandbox/server.Dockerfile \
#     -t <registry>/opensandbox-server:<ver> deploy/opensandbox
#
# NOTE (m32 t007): this image's snapshot round-trip is validated on a real
# containerd-CRI + Kata node (k3s), NOT the OrbStack mock — see ADR042 D6.
FROM python:3.12-slim

# Pin the server version; bump deliberately with a re-validation on k3s.
ARG OPENSANDBOX_SERVER_VERSION=

# uv gives fast, reproducible installs; kubectl is not needed (in-cluster SA +
# the Python k8s client the server already depends on).
RUN pip install --no-cache-dir "uv" \
  && uv pip install --system --no-cache \
       "opensandbox-server${OPENSANDBOX_SERVER_VERSION:+==${OPENSANDBOX_SERVER_VERSION}}"

# Run as non-root; the server binds a high port and needs no privileges.
RUN useradd --create-home --uid 10001 opensandbox
USER 10001

# The config path is provided at runtime via SANDBOX_CONFIG_PATH (mounted
# ConfigMap); the cluster TOML sets [runtime] type=kubernetes and the in-cluster
# ServiceAccount handles auth. Multi-tenant mode + api_key come from the same
# config (t006). The listen port is set in the TOML ([server].port).
ENTRYPOINT ["opensandbox-server"]
CMD ["--config", "/etc/opensandbox/sandbox-cluster.toml"]
