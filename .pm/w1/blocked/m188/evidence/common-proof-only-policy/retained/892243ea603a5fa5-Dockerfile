# syntax=docker/dockerfile:1.7
# One Bookworm system/tool layer. Selected primary runtimes are installed above
# this image by the native resolver; this factory installs no primary runtime.
ARG BASE_IMAGE=buildpack-deps:bookworm@sha256:db73e309a26ddb362c11c0ee35b3d957c3a8fd2ef6a9d6c2cc76438fae2c0988

FROM node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea AS node-tools
WORKDIR /opt/bex/native-node-tools
COPY native-base/package.json native-base/package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund \
  && cp /usr/local/bin/node /opt/bex/native-node-tools/node

FROM ${BASE_IMAGE}
USER root
COPY --link --from=node-tools /opt/bex/native-node-tools /opt/bex/native-node-tools
COPY native-base/node-brotli/check.js /usr/share/bex-security/node-check.js
COPY native-base/tsc native-base/webpack /usr/bin/
COPY native-base/python-tools.json native-base/fetch-python-tools.py native-base/install-python-tools.py /opt/bex/python-tools/
COPY native-base/bex-native-python-tools /usr/bin/
COPY native-base/install.sh /tmp/bex-native-install.sh
COPY native-base/patch-system-brotli.sh /tmp/bex-patch-system-brotli.sh
# Language images previously supplied these dependencies themselves. Keep the
# matching Bookworm libraries/headers in the shared layer, including OTP's
# optional native applications rather than silently dropping them.
RUN chmod 755 /usr/bin/tsc /usr/bin/webpack /usr/bin/bex-native-python-tools \
  && sh /tmp/bex-native-install.sh \
    libbluetooth-dev tk-dev uuid-dev libzstd-dev libjemalloc-dev \
    unixodbc-dev libsctp-dev libwxgtk3.2-dev libwxgtk-webview3.2-dev \
  && /usr/bin/python3 /opt/bex/python-tools/fetch-python-tools.py \
  && rm /tmp/bex-native-install.sh /tmp/bex-patch-system-brotli.sh

COPY native-common/check-common.py /usr/share/bex-native-common/check-common.py
RUN --network=none /usr/bin/python3 /usr/share/bex-native-common/check-common.py
