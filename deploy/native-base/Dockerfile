# syntax=docker/dockerfile:1.7
# Official language versions stay pinned by the publishing matrix. System
# packages are installed once here, never by a tenant build.
ARG BASE_IMAGE=scratch
FROM ${BASE_IMAGE} AS primary

FROM node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea AS node-tools
WORKDIR /opt/bex/native-node-tools
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund \
  && cp /usr/local/bin/node /opt/bex/native-node-tools/node

FROM node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e AS node-backport
COPY node-brotli/patch.sh /tmp/bex-node-brotli/patch.sh
# Companions also run on Bullseye. A Bookworm-built Node would require newer
# libc/libstdc++ than Python 3.7 and Ruby 3.0 supply. Inspect the header before
# executing anything from a modern primary on this older compiler baseline.
RUN --mount=type=bind,from=primary,source=/usr/local,target=/opt/bex-primary,readonly \
  set -eu; \
  mkdir -p /opt/bex-node-backport; \
  if [ -f /opt/bex-primary/include/node/node_version.h ]; then \
    major="$(awk '$1 == "#define" && $2 == "NODE_MAJOR_VERSION" { print $3 }' /opt/bex-primary/include/node/node_version.h)"; \
    case "$major" in \
      14|16|18|20) \
        rm -rf /usr/local; \
        mkdir -p /usr/local; \
        cp -a /opt/bex-primary/. /usr/local/; \
        sh /tmp/bex-node-brotli/patch.sh; \
        install -D -m 755 /usr/local/bin/node /opt/bex-node-backport/usr/local/bin/node; \
        install -D -m 644 /usr/share/bex-node-brotli/sources.txt /opt/bex-node-backport/usr/share/bex-node-brotli/sources.txt \
        ;; \
    esac; \
  fi

FROM primary
USER root
# Only the repaired executable and its source record leave the compiler stage.
# Inherit every runtime environment setting from the original primary.
COPY --link --from=node-backport /opt/bex-node-backport/ /
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
