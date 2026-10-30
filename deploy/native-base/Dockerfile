# syntax=docker/dockerfile:1.7
# Official language versions stay pinned by the publishing matrix. System
# packages are installed once here, never by a tenant build.
ARG BASE_IMAGE=scratch
FROM node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea AS node-tools
WORKDIR /opt/bex/native-node-tools
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund \
  && cp /usr/local/bin/node /opt/bex/native-node-tools/node

FROM ${BASE_IMAGE}
USER root
COPY node-brotli/patch.sh /tmp/bex-node-brotli/patch.sh
# Patch the interpreter itself before adding utilities: Node's bundled Brotli
# is independent of Debian's libbrotli1. The script is a no-op off EOL Node.
RUN sh /tmp/bex-node-brotli/patch.sh && rm -rf /tmp/bex-node-brotli
COPY node-brotli/check.js /usr/share/bex-security/node-check.js
COPY --link --from=node-tools /opt/bex/native-node-tools /opt/bex/native-node-tools
COPY tsc webpack /usr/bin/
COPY python-tools.json fetch-python-tools.py install-python-tools.py /opt/bex/python-tools/
COPY bex-native-python-tools /usr/bin/
COPY install.sh /tmp/bex-native-install.sh
COPY patch-system-brotli.sh /tmp/bex-patch-system-brotli.sh
RUN chmod 755 /usr/bin/tsc /usr/bin/webpack /usr/bin/bex-native-python-tools \
  && sh /tmp/bex-native-install.sh \
  && if [ -x /usr/local/bin/python3 ]; then \
    /usr/bin/python3 /opt/bex/python-tools/fetch-python-tools.py; \
  fi \
  && rm /tmp/bex-native-install.sh /tmp/bex-patch-system-brotli.sh
RUN --network=none /usr/bin/bex-native-python-tools
