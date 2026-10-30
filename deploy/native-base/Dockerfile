# Official language versions stay pinned by the publishing matrix. System
# packages are installed once here, never by a tenant build.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}
USER root
COPY install.sh /tmp/bex-native-install.sh
RUN sh /tmp/bex-native-install.sh && rm /tmp/bex-native-install.sh
