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
FROM golang:1.26@sha256:9d2f36f06329b2a141b9db99ffa32765cf695ee57b813ca29e245e8670bcbfff AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Module graph first for layer caching. types/ is a local `replace` target of the
# other two modules, so its (tiny) source must be present before `go mod download`.
COPY operator/go.mod operator/go.sum ./operator/
COPY backend/go.mod  backend/go.sum  ./backend/
COPY types/ ./types/
RUN cd operator && go mod download
RUN cd backend  && go mod download

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
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/backup-encrypt ./cmd/backup-encrypt
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/disk-snapshot ./cmd/disk-snapshot
# Output name must not be "api" — that collides with the cmd/api source dir, so
# go build would write the binary INTO that directory.
RUN cd backend  && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/bexapi ./cmd/api
RUN cd backend  && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /workspace/ssh-gateway ./cmd/ssh-gateway

# Use distroless as minimal base image to package the binaries
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/bexapi /api
COPY --from=builder /workspace/activator /activator
COPY --from=builder /workspace/staticserver /staticserver
COPY --from=builder /workspace/pg-sni-proxy /pg-sni-proxy
COPY --from=builder /workspace/kv-sni-proxy /kv-sni-proxy
COPY --from=builder /workspace/egress-meter /egress-meter
COPY --from=builder /workspace/backup-encrypt /backup-encrypt
COPY --from=builder /workspace/disk-snapshot /disk-snapshot
COPY --from=builder /workspace/ssh-gateway /ssh-gateway
USER 65532:65532

# Default entrypoint is the operator; the api Deployment sets command: ["/api"].
ENTRYPOINT ["/manager"]
