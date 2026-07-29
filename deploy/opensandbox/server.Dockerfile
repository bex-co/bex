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
# NOTE (m32 t007): this image's sandbox path is validated on a real
# containerd-CRI + gVisor node, NOT the OrbStack mock — see ADR042 D6.
FROM python:3.12.13-slim-trixie@sha256:cab2dbf575e971934a81e4622f5aba17aa7929719bd7e31033a3a83b97fd0464

# Pin the server version; bump deliberately with a re-validation on k3s.
ARG OPENSANDBOX_SERVER_VERSION=0.2.2

# kubectl is not needed (in-cluster SA + the Python k8s client the server already
# depends on). Pin the entire resolved dependency graph, not only the top-level
# package: otherwise a rebuild can silently ingest a new FastAPI/Kubernetes/etc.
# release. The build arg is retained as an explicit release gate and must match
# the reviewed lock.
COPY requirements.lock /tmp/opensandbox-requirements.lock
RUN grep -qx "opensandbox-server==${OPENSANDBOX_SERVER_VERSION}" /tmp/opensandbox-requirements.lock \
    && pip install --no-cache-dir --requirement /tmp/opensandbox-requirements.lock \
    && pip check \
    && rm /tmp/opensandbox-requirements.lock

# Run as non-root; the server binds a high port and needs no privileges.
RUN useradd --create-home --uid 10001 opensandbox
USER 10001

# The config path is provided at runtime via SANDBOX_CONFIG_PATH (mounted
# ConfigMap); the cluster TOML sets [runtime] type=kubernetes and the in-cluster
# ServiceAccount handles auth. Multi-tenant mode comes from the rendered config
# and deliberately has no shared server api_key. The listen port is set in the
# TOML ([server].port).
ENTRYPOINT ["opensandbox-server"]
CMD ["--config", "/etc/opensandbox/sandbox-cluster.toml"]
