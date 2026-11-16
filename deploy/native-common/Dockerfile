# syntax=docker/dockerfile:1.7
# One Bookworm system/tool layer. Selected primary runtimes are installed above
# this image by the native resolver; this factory installs no primary runtime.
ARG BASE_IMAGE=buildpack-deps:bookworm@sha256:f53cd4c58cdb96a467b98696cd510a949ef4ccd7d23e07b75107de754773007a

FROM node:24-bookworm@sha256:9d266751ca3d7fdbe651c878ffd5ad0a42fed49a18c283509267c97f025b7496 AS node-tools
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
COPY native-common/legacy-libs.sources /etc/apt/sources.list.d/bex-native-legacy.sources
COPY native-common/legacy-libs.preferences /etc/apt/preferences.d/bex-native-legacy
# Language images previously supplied these dependencies themselves. Keep the
# matching Bookworm libraries/headers in the shared layer, including OTP's
# optional native applications rather than silently dropping them.
RUN chmod 755 /usr/bin/tsc /usr/bin/webpack /usr/bin/bex-native-python-tools \
  && sh /tmp/bex-native-install.sh \
    python3-venv libbluetooth-dev tk-dev uuid-dev libzstd-dev libjemalloc-dev \
    unixodbc-dev libsctp-dev libwxgtk3.2-dev libwxgtk-webview3.2-dev libssl1.1 libffi7 \
  && /usr/bin/python3 /opt/bex/python-tools/fetch-python-tools.py \
  && rm /tmp/bex-native-install.sh /tmp/bex-patch-system-brotli.sh \
    /etc/apt/sources.list.d/bex-native-legacy.sources /etc/apt/preferences.d/bex-native-legacy

COPY native-common/check-common.py /usr/share/bex-native-common/check-common.py
COPY --chmod=755 native-common/build-otp /usr/libexec/bex-native/build-otp
COPY native-common/otp-features.py /usr/libexec/bex-native/otp-features.py
COPY native-common/prepare-node14-tools.py /usr/libexec/bex-native/prepare-node14-tools.py
COPY --chmod=755 native-base/node-brotli/patch.sh /usr/libexec/bex-native/build-node
# Default Poetry can use this private tool interpreter beside Python 3.7/3.8.
# Verify Debian's complete venv/ensurepip prerequisite without installing Poetry.
RUN --network=none /usr/bin/python3 -m venv /tmp/bex-poetry-bootstrap \
  && /tmp/bex-poetry-bootstrap/bin/python -I -m pip --version \
  && rm -rf /tmp/bex-poetry-bootstrap \
  && /usr/bin/python3 /usr/share/bex-native-common/check-common.py
