# Build the two bex binaries from the multi-module workspace (build context is
# lego/). One image, two entrypoints — the manager (operator module, mechanism)
# and bex-api (backend module, business logic), both sharing the types module
# (the CRD contract). The api Deployment overrides command to /api.
FROM golang:1.25 AS builder
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
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o /workspace/manager ./cmd/manager
RUN cd operator && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o /workspace/activator ./cmd/activator
# Output name must not be "api" — that collides with the cmd/api source dir, so
# go build would write the binary INTO that directory.
RUN cd backend  && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o /workspace/bexapi ./cmd/api

# Use distroless as minimal base image to package the binaries
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/bexapi /api
COPY --from=builder /workspace/activator /activator
USER 65532:65532

# Default entrypoint is the operator; the api Deployment sets command: ["/api"].
ENTRYPOINT ["/manager"]
