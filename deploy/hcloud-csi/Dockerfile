# Exact hcloud CSI v2.22.0 plus the reviewed logical ext4 sizing patch. The
# original image supplies the same cryptsetup/e2fsprogs library closure.
FROM --platform=$BUILDPLATFORM golang:1.26.10@sha256:672c944cfd07599b5f1bb179869270cdc94c6254e41db4d431259693cb286b82 AS builder
WORKDIR /src
ADD --checksum=sha256:5c3697eef3ca6a97b437930fca51ea9d440a43e33842f96d9ce20300bf4eeae6 https://github.com/hetznercloud/csi-driver/archive/2d814c4f36351a6f1652f80657cb3ddd18d51a0b.tar.gz /tmp/source.tar.gz
RUN tar -xzf /tmp/source.tar.gz --strip-components=1 -C /src && rm /tmp/source.tar.gz
COPY bounded-ext4.patch /tmp/bounded-ext4.patch
RUN git apply --check /tmp/bounded-ext4.patch && git apply /tmp/bounded-ext4.patch
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go test ./...
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go test -c -o /out/bounded-ext4.test ./internal/volumes
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -o /out/hcloud-csi-driver ./cmd

FROM docker.io/hetznercloud/hcloud-csi-driver:v2.22.0@sha256:459090f3c173d6708dcb3d85801509b371c9b8fffd76d704f482de3099729664 AS runtime
COPY --from=builder /out/hcloud-csi-driver /bin/hcloud-csi-driver
LABEL org.opencontainers.image.source="https://github.com/bex-co/bex" \
      org.opencontainers.image.version="2.22.0-bex-bounded-ext4-v1"

# This target is local/CI-only, never the published platform image.
FROM runtime AS kernel-test
RUN apk add --no-cache util-linux
COPY --from=builder /out/bounded-ext4.test /bounded-ext4.test
ENTRYPOINT ["/bounded-ext4.test", "-test.run=^TestBoundedExt4Kernel$", "-test.v"]

FROM runtime AS final
