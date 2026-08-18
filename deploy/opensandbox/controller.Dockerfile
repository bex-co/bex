# OpenSandbox controller image with bex's carried controller patches.
#
# The upstream controller creates its snapshot commit/unpause Jobs — which
# hostPath-mount the node containerd socket — in the SandboxSnapshot's own
# tenant namespace, where PodSecurity "baseline" rejects them, so pause hangs.
# patches/controller-snapshot-job-namespace.patch adds a
# --snapshot-job-namespace flag confining those privileged Jobs to one platform
# namespace. patches/controller-terminal-pod-status.patch makes a failed child
# Pod terminalize its BatchSandbox (w5/m72). This build clones upstream at a
# pinned commit, applies both patches, and mirrors upstream's build flags. Drop
# each patch once its change lands in the pinned upstream release.
#
# Build (CI: deploy.yml step build_opensandbox_controller):
#   docker build -f deploy/opensandbox/controller.Dockerfile \
#     -t <registry>/opensandbox-controller:<ver> deploy/opensandbox

FROM golang:1.25.12-alpine@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587 AS builder
ARG TARGETOS
ARG TARGETARCH

# Pin the exact upstream commit the carried patch was authored against; bump the
# two together (the patch header records the same SHA).
ARG OPENSANDBOX_COMMIT=e95681e791b33b3893033940cbeaa5ab192bf21b

RUN apk add --no-cache git
WORKDIR /src
RUN git init -q opensandbox \
    && cd opensandbox \
    && git remote add origin https://github.com/alibaba/OpenSandbox \
    && git fetch -q --depth 1 origin "${OPENSANDBOX_COMMIT}" \
    && git checkout -q FETCH_HEAD

COPY patches/controller-snapshot-job-namespace.patch /tmp/snapshot-job-namespace.patch
COPY patches/controller-terminal-pod-status.patch /tmp/terminal-pod-status.patch
RUN cd opensandbox \
    && git apply --stat --apply /tmp/snapshot-job-namespace.patch \
    && git apply --stat --apply /tmp/terminal-pod-status.patch

WORKDIR /src/opensandbox/kubernetes
RUN go mod download

# Mirror upstream kubernetes/Dockerfile: static, trimmed, no VCS stamping.
RUN CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="${TARGETARCH}" \
    go build -trimpath -buildvcs=false \
    -ldflags "-buildid= -B none -X main.commitID=${OPENSANDBOX_COMMIT}-bex-snapshot-job-ns-terminal-pod" \
    -o server ./cmd/controller

# Upstream ships the runtime on golang:alpine (nsenter via util-linux
# availability rationale); keep the same base so behavior matches.
FROM golang:1.25.12-alpine@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587
ARG USERID=65532
WORKDIR /workspace
COPY --from=builder /src/opensandbox/kubernetes/server .
USER $USERID
ENTRYPOINT ["/workspace/server"]
