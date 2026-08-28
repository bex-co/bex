# LOCAL-DEV ONLY arm64-capable build of the OpenSandbox lifecycle server.
#
# deploy/opensandbox/server.Dockerfile is the reviewed production image and pins
# its python base by DIGEST. That digest names one architecture (amd64), so on an
# Apple-Silicon workstation it produces an amd64 image that a local arm64
# containerd node cannot exec ("exec format error"). Production builds on amd64
# runners and must keep the digest pin; this sibling exists only so `dev-env.sh N
# agent-up` can build a natively-runnable server for the CAPD mock cluster.
#
# Everything else — the pinned server version and the full resolved dependency
# lock — is identical to the production image, and the same
# `grep -qx opensandbox-server==<version>` release gate is enforced here.
#
#   docker build -f scripts/dev-env/agent/opensandbox-server.local.Dockerfile \
#     -t opensandbox-server:0.2.2-local deploy/opensandbox
#
# The base is pinned to the MULTI-ARCH INDEX digest of python:3.12.13-slim-trixie
# (not production's amd64-specific pin), so scripts/image-pin-validate.sh's
# supply-chain gate is satisfied while `docker build` on Apple Silicon still
# resolves the arm64 variant from the index.
FROM python:3.12.13-slim-trixie@sha256:229a2c5bfa27522db7815ea81f9bed70af17ccb9de9fc7ad142b1877b5830d36

ARG OPENSANDBOX_SERVER_VERSION=0.2.2

COPY requirements.lock /tmp/opensandbox-requirements.lock
RUN grep -qx "opensandbox-server==${OPENSANDBOX_SERVER_VERSION}" /tmp/opensandbox-requirements.lock \
    && pip install --no-cache-dir --requirement /tmp/opensandbox-requirements.lock \
    && pip check \
    && rm /tmp/opensandbox-requirements.lock

RUN useradd --create-home --uid 10001 opensandbox
USER 10001

ENTRYPOINT ["opensandbox-server"]
CMD ["--config", "/etc/opensandbox/sandbox-cluster.toml"]
