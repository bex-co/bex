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
COPY --link --from=node-tools /opt/bex/native-node-tools /opt/bex/native-node-tools
COPY tsc webpack /usr/bin/
COPY install.sh /tmp/bex-native-install.sh
RUN chmod 755 /usr/bin/tsc /usr/bin/webpack \
  && sh /tmp/bex-native-install.sh && rm /tmp/bex-native-install.sh
