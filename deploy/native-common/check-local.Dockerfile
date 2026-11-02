# syntax=docker/dockerfile:1.7
# D3 local-development smoke only. No selected primary, extension or full tool
# transformation claim; production uses the complete AMD64 publisher checks.
ARG COMMON_IMAGE
FROM ${COMMON_IMAGE}
RUN --network=none test "$(uname -m)" = aarch64 \
  && grep -qx 'VERSION_CODENAME=bookworm' /etc/os-release \
  && /usr/bin/python3 -c 'import ssl, ctypes; assert ssl.OPENSSL_VERSION; ctypes.CDLL("libffi.so.8")' \
  && /opt/bex/native-node-tools/node --version \
  && jq --version && psql --version && pg_restore --version \
  && gcc --version && tsc --version && webpack --version
