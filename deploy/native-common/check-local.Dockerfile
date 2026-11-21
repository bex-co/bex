# syntax=docker/dockerfile:1.7
# D3 local-development smoke only. No selected primary, extension or full tool
# transformation claim; production uses the complete AMD64 publisher checks.
# The magic-wormhole round trip is the one exception (w6/m549): Render
# pre-installs it on every native runtime, so it runs on both architectures.
ARG COMMON_IMAGE
FROM ${COMMON_IMAGE} AS common

# Test-only relay pair: Bookworm's signed server packages, unpacked, never installed.
FROM common AS wormhole-servers
RUN apt-get update \
  && mkdir /tmp/bex-wormhole-debs && cd /tmp/bex-wormhole-debs \
  && apt-get download python3-magic-wormhole-mailbox-server magic-wormhole-transit-relay \
  && for package in ./*.deb; do dpkg-deb -x "$package" /opt/bex-wormhole-servers; done

FROM common
RUN --network=none test "$(uname -m)" = aarch64 \
  && grep -qx 'VERSION_CODENAME=bookworm' /etc/os-release \
  && /usr/bin/python3 -c 'import ssl, ctypes; assert ssl.OPENSSL_VERSION; ctypes.CDLL("libffi.so.8")' \
  && /opt/bex/native-node-tools/node --version \
  && jq --version && psql --version && pg_restore --version \
  && gcc --version && tsc --version && webpack --version && wormhole --version
COPY native-base/check-wormhole.py /tmp/check-wormhole.py
RUN --network=none --mount=type=bind,from=wormhole-servers,source=/opt/bex-wormhole-servers,target=/opt/bex-wormhole-servers,readonly \
  /usr/bin/python3 /tmp/check-wormhole.py /opt/bex-wormhole-servers
