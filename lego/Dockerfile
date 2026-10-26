# Build the bex binaries from the multi-module workspace (build context is
# lego/). One image, several entrypoints — the manager (operator module,
# mechanism), the wake activator, the static-site origin server, the Postgres
# SNI proxy (operator module too), bex-api, and the isolated SSH gateway
# (backend module, business logic),
# all sharing the types module (the CRD contract). Each non-default Deployment
# overrides command (/api, /activator, /staticserver, /pg-sni-proxy,
# /ssh-gateway), and the Key Value backup CronJob's encrypt stage runs
# /backup-encrypt out of this same image (w7/m85 — it used to download the age
# release into an alpine container standing next to the plaintext RDB).
FROM golang:1.26.10@sha256:672c944cfd07599b5f1bb179869270cdc94c6254e41db4d431259693cb286b82 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Module graph first for layer caching. types/ is a local `replace` target of the
# other two modules, so its (tiny) source must be present before `go mod download`.
COPY operator/go.mod operator/go.sum ./operator/
COPY backend/go.mod  backend/go.sum  ./backend/
COPY types/ ./types/
# go mod download retries (3 attempts, backoff): a single proxy.golang.org TLS
# handshake timeout failed a whole deploy build on 2026-10-04.
RUN cd operator && n=0; until go mod download; do n=$((n+1)); [ "$n" -ge 3 ] && exit 1; sleep $((n*15)); done
RUN cd backend  && n=0; until go mod download; do n=$((n+1)); [ "$n" -ge 3 ] && exit 1; sleep $((n*15)); done

# Source (relies on .dockerignore to filter to *.go + go.mod/go.sum)
COPY operator/ ./operator/
COPY backend/  ./backend/

# Build. GOARCH left unset so the binary matches the build host / BUILDPLATFORM.
# Each module builds in its own dir; the `replace …/lego/types => ../types` in
# each go.mod resolves the shared contract without a workspace file.
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/manager ./cmd/manager
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/activator ./cmd/activator
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/staticserver ./cmd/staticserver
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/pg-sni-proxy ./cmd/pg-sni-proxy
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/kv-sni-proxy ./cmd/kv-sni-proxy
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/egress-meter ./cmd/egress-meter
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/edge-cache ./cmd/edge-cache
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/backup-encrypt ./cmd/backup-encrypt
# The no-redirect health probe a service's init container installs into its pod (w8/m133).
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /workspace/health-probe ./cmd/health-probe
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/native-resolve ./cmd/native-resolve
# native-init ships inside every native Node tenant image (w8/m88): stripped
# and path-free so each release adds one small, reproducible layer.
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /workspace/native-init ./cmd/native-init
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/disk-snapshot ./cmd/disk-snapshot
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/drain-guard ./cmd/drain-guard
# Output name must not be "api" — that collides with the cmd/api source dir, so
# go build would write the binary INTO that directory.
RUN cd backend  && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/bexapi ./cmd/api
RUN cd backend  && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/ssh-gateway ./cmd/ssh-gateway

# Use distroless as minimal base image to package the binaries
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:d62067879de890a1def9da0e7766b7d20810a5e5555cef8506b752437d7a3e88
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/bexapi /api
COPY --from=builder /workspace/activator /activator
COPY --from=builder /workspace/staticserver /staticserver
COPY --from=builder /workspace/pg-sni-proxy /pg-sni-proxy
COPY --from=builder /workspace/kv-sni-proxy /kv-sni-proxy
COPY --from=builder /workspace/egress-meter /egress-meter
COPY --from=builder /workspace/edge-cache /edge-cache
COPY --from=builder /workspace/backup-encrypt /backup-encrypt
COPY --from=builder /workspace/health-probe /health-probe
COPY --from=builder /workspace/native-resolve /native-resolve
COPY --from=builder /workspace/native-init /native-init
COPY --from=builder /workspace/disk-snapshot /disk-snapshot
COPY --from=builder /workspace/drain-guard /drain-guard
COPY --from=builder /workspace/ssh-gateway /ssh-gateway
USER 65532:65532

# Default entrypoint is the operator; the api Deployment sets command: ["/api"].
ENTRYPOINT ["/manager"]
