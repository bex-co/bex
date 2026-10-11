# kubelet with one carried patch (w4/m200): the container-log reader writes
# each stdout/stderr record whole instead of joining a stderr line to a long
# stdout record's partial chunks. Rationale, publishing and rollback:
# infra/packer/README.md "Patched kubelet". Unfixed upstream on master
# (2026-10-11); drop the patch once a release carries an equivalent fix.
#
# docker buildx build --platform linux/amd64 --output type=local,dest=out \
#   infra/packer/log-record-framing   # CI: .github/workflows/kubelet-artifact.yml

FROM --platform=$BUILDPLATFORM golang:1.25.11-bookworm@sha256:b96f24a8d7d010ea0acb9c3ba99064740f02b6b984612b28bd3c9c5ab9453e38 AS build
ARG TARGETARCH
# Tag v1.34.9; bump with release.conf's KUBERNETES_VERSION and re-prove the patch.
ARG KUBERNETES_COMMIT=ad7c7374b74c04d07ea041d367ecb1a526bdf758
ARG KUBERNETES_VERSION=v1.34.9
# The kubelet links cgo, as upstream's release build does; cross-compile it.
RUN apt-get update -q && apt-get install -qy --no-install-recommends gcc-x86-64-linux-gnu libc6-dev-amd64-cross rsync \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git init -q kubernetes \
    && cd kubernetes \
    && git remote add origin https://github.com/kubernetes/kubernetes \
    && git fetch -q --depth 1 origin "${KUBERNETES_COMMIT}" \
    && git checkout -q FETCH_HEAD
COPY kubelet-cri-records.patch /tmp/
WORKDIR /src/kubernetes
RUN git apply --stat --apply /tmp/kubelet-cri-records.patch
RUN cd staging/src/k8s.io/cri-client && go test ./pkg/logs/ -count=1
# Reproducible stamp: the exact upstream commit and its date, a "dirty" tree
# (visible in `kubelet --version=raw`), and the unchanged release version, so
# the node bootstrap's `kubelet --version` pin keeps holding. The binary's
# identity is its sha256, pinned in the Packer recipe.
RUN printf '%s\n' \
      "KUBE_GIT_COMMIT='${KUBERNETES_COMMIT}'" \
      "KUBE_GIT_TREE_STATE='dirty'" \
      "KUBE_GIT_VERSION='${KUBERNETES_VERSION}'" \
      "KUBE_GIT_MAJOR='$(echo "${KUBERNETES_VERSION#v}" | cut -d. -f1)'" \
      "KUBE_GIT_MINOR='$(echo "${KUBERNETES_VERSION#v}" | cut -d. -f2)'" > /tmp/version \
    && SOURCE_DATE_EPOCH="$(git log -1 --format=%ct)" KUBE_GIT_VERSION_FILE=/tmp/version \
       make WHAT=cmd/kubelet KUBE_BUILD_PLATFORMS="linux/${TARGETARCH}" \
    && cp "_output/local/bin/linux/${TARGETARCH}/kubelet" /kubelet \
    && cd / && sha256sum kubelet > /kubelet.sha256

FROM scratch
COPY --from=build /kubelet /kubelet.sha256 /
