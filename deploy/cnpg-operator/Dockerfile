# CloudNativePG's operator, built from the release UPSTREAM names plus bex's
# reviewed patches (w21/m606, docs/ADR092 § The CloudNativePG build).
#
# patches/0001-instance-keep-extensions-the-cluster-preloads.patch keeps an
# owner's `CREATE EXTENSION pgaudit` through an instance manager restart: the
# release drops every managed extension no `pgaudit.*`-style parameter uses,
# even one whose library the cluster preloads itself. Each patch comes out once
# an upstream release carries an equivalent fix.
#
# One binary is both the operator and the instance manager CloudNativePG copies
# into every Postgres pod, and the image carries it for both architectures
# (/operator/manager_amd64 and /operator/manager_arm64), because the operator
# hands each instance the one for its node. The runtime stage is the release
# image itself, so everything else in it (the distroless base, the licenses,
# the /manager link) is the release's own; only the two binaries are rebuilt,
# with the release's version, commit and date ldflags. The commit gains a
# `-bex.<revision>` suffix, which `manager version` and each Cluster's
# status.cloudNativePGCommitHash show.
#
# Build (CI: .github/workflows/cnpg-operator-image.yml):
#   docker buildx build --platform linux/amd64,linux/arm64 deploy/cnpg-operator

# Cross-compiles on the build host's platform; nothing in this stage depends on
# the target, so a two-platform build runs it once.
FROM --platform=$BUILDPLATFORM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS builder
ENV GOTOOLCHAIN=local CGO_ENABLED=0
COPY UPSTREAM /bex/UPSTREAM
COPY patches/ /bex/patches/
WORKDIR /src/cloudnative-pg
# The release tag must still name the reviewed commit.
RUN . /bex/UPSTREAM \
    && git init -q . \
    && git remote add origin https://github.com/cloudnative-pg/cloudnative-pg \
    && git fetch -q --depth 1 origin "refs/tags/v${CNPG_VERSION}:refs/tags/v${CNPG_VERSION}" \
    && tagged="$(git rev-parse "v${CNPG_VERSION}^{commit}")" \
    && { [ "$tagged" = "$CNPG_COMMIT" ] || { echo "v${CNPG_VERSION} is $tagged, UPSTREAM says $CNPG_COMMIT" >&2; exit 1; }; } \
    && git -c advice.detachedHead=false checkout -q "$CNPG_COMMIT"
# A patch that no longer applies fails the build.
RUN for patch in /bex/patches/*.patch; do \
      git apply --check "$patch" && git apply "$patch" || { echo "$patch does not apply" >&2; exit 1; }; \
    done
# go mod download retries (3 attempts, backoff): one proxy.golang.org timeout
# must not fail the build (deploy/opensandbox/controller.Dockerfile).
RUN n=0; until go mod download; do n=$((n+1)); [ "$n" -ge 3 ] && exit 1; sleep $((n*15)); done
# The patched packages' tests, upstream's and the patch's own.
RUN --mount=type=cache,target=/root/.cache/go-build \
    go vet ./internal/management/controller/ \
    && go test ./internal/management/controller/... ./pkg/postgres/...
RUN --mount=type=cache,target=/root/.cache/go-build . /bex/UPSTREAM \
    && commit="$(printf %.9s "$CNPG_COMMIT")-bex.${BEX_REVISION}" \
    && date="$(git log -1 --pretty=format:%ad --date=short "$CNPG_COMMIT")" \
    && versions=github.com/cloudnative-pg/cloudnative-pg/pkg/versions \
    && for arch in amd64 arm64; do \
      GOOS=linux GOARCH="$arch" go build -trimpath -o "/out/manager_$arch" \
        -ldflags "-s -w -X $versions.buildVersion=${CNPG_VERSION} -X $versions.buildCommit=${commit} -X $versions.buildDate=${date}" \
        ./cmd/manager || exit 1; \
    done

# The release image of UPSTREAM's CNPG_VERSION, by digest.
FROM ghcr.io/cloudnative-pg/cloudnative-pg:1.30.1@sha256:923c267ec29636db3bee20f993d0ec4973fa22998e1adad37da79e4d32b5bc07
COPY --from=builder --chown=65532:65532 --chmod=0755 /out/manager_amd64 /out/manager_arm64 /operator/
LABEL org.opencontainers.image.source="https://github.com/bex-co/bex" \
      org.opencontainers.image.base.name="ghcr.io/cloudnative-pg/cloudnative-pg:1.30.1@sha256:923c267ec29636db3bee20f993d0ec4973fa22998e1adad37da79e4d32b5bc07"
