# Regression check for the bex CNB run image (w1/m169): start the official
# Node 26 linux binary, which links libatomic.so.1, on RUN_IMAGE. Built in CI per
# platform and never pushed. On paketobuildpacks/run-jammy-base this fails with
# "error while loading shared libraries: libatomic.so.1" (exit 127).
ARG RUN_IMAGE
ARG TARGETARCH
FROM ${RUN_IMAGE} AS check-amd64
ADD --checksum=sha256:cb5c9ce9c80d7b8821e3a258543c71b939138cf17c74d5cc44bbe85d6dbc5ad8 \
  https://nodejs.org/dist/v26.10.0/node-v26.10.0-linux-x64.tar.gz /tmp/node.tar.gz

FROM ${RUN_IMAGE} AS check-arm64
ADD --checksum=sha256:423a41bff8e2a2fa15e702fefe2919ef95823b2378744daccb8439302534b44f \
  https://nodejs.org/dist/v26.10.0/node-v26.10.0-linux-arm64.tar.gz /tmp/node.tar.gz

# TARGETARCH selects the stage, so each platform runs its own Node binary.
FROM check-${TARGETARCH}
USER root
RUN mkdir /tmp/node \
  && tar -xzf /tmp/node.tar.gz -C /tmp/node --strip-components=1 \
  && /tmp/node/bin/node -e 'console.log("node", process.version, process.arch, "starts")'
