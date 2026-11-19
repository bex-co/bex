# syntax=docker/dockerfile:1.7

# Tools baked in beside the language toolchains (w10/m513), each pinned by
# version here and by digest, SHA-256 or module sum where it is fetched. The
# final stage's check fails the build unless each reports its pin.
ARG PYTHON312_VERSION=3.12.16
ARG UV_VERSION=0.13.9
ARG GITLEAKS_VERSION=8.30.1
ARG GH_VERSION=2.105.0

FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS go-toolchain

# The driver is TypeScript; this stage compiles it to plain JS so devDependencies
# (typescript) never reach the runtime image below.
# Platform-independent output (tsc to JS, npm ci --ignore-scripts): build it
# natively on the build host instead of under emulation.
FROM --platform=$BUILDPLATFORM node:22.23.4-bookworm-slim@sha256:de170b39439eb0e5a3444ad7d60b015614a381e10fecae2e30ec9e735122f38d AS driver-build

WORKDIR /opt/bex-agent-driver
COPY agent-image/driver/package.json agent-image/driver/package-lock.json ./
RUN npm ci --ignore-scripts
COPY agent-image/driver/tsconfig.json ./
COPY backend/internal/agentsession/agent-profiles.json ./agent-profiles.json
COPY agent-image/driver/src ./src
RUN npm run build

# Warm Go dependency cache (ADR105 § Until the AX42-1 arrives). This stage
# builds the third-party packages listed in agent-image/go-deps.txt (deploy.yml
# writes it with `go list -deps -test` over the four modules) in workspace mode,
# with the toolchain the image ships, so that a session building this
# repository with default flags reuses them. GOMODCACHE and GOCACHE must equal
# the final stage's: build cache keys include each package's directory. A build
# without the list (a local one) leaves the cache empty.
FROM go-toolchain AS go-deps
ENV GOMODCACHE=/opt/bex-go/mod \
    GOCACHE=/opt/bex-go/cache \
    GOFLAGS= \
    GOTOOLCHAIN=local
WORKDIR /workspace/lego
COPY go.work go.work.sum agent-image/go-deps.tx[t] ./
COPY types/go.mod types/go.sum ./types/
COPY operator/go.mod operator/go.sum ./operator/
COPY backend/go.mod backend/go.sum ./backend/
COPY cli/go.mod cli/go.sum ./cli/
# Every listed package builds in a session too, so a failure here is real; the
# retries cover the module proxy. `go vet` in a session computes facts for every
# dependency, so a throwaway package importing each importable one is vetted to
# cache those too. The module zips are dropped: a build reads the extracted
# trees. Entries get a far-future mtime, so Go neither trims them (it trims by
# mtime) nor rewrites their mtime on use, which on an image layer would copy
# every used file into the sandbox's writable layer.
RUN set -e; \
    mkdir -p "$GOMODCACHE" "$GOCACHE"; \
    if [ -s go-deps.txt ]; then \
      n=0; until xargs go build < go-deps.txt; do \
        n=$((n+1)); [ "$n" -lt 3 ] || exit 1; sleep $((n*15)); \
      done; \
      mkdir backend/zzwarmvet; \
      { echo 'package zzwarmvet'; echo 'import ('; \
        grep -Ev '(^|/)internal(/|$)' go-deps.txt | sed 's/.*/\t_ "&"/'; \
        echo ')'; } > backend/zzwarmvet/warm.go; \
      go vet ./backend/zzwarmvet || echo "warning: vet facts are partial"; \
      rm -rf backend/zzwarmvet; \
    fi; \
    if [ -d "$GOMODCACHE/cache/download" ]; then find "$GOMODCACHE/cache/download" -name '*.zip' -delete; fi; \
    find "$GOCACHE" -type f -name '*-[ad]' -exec touch -d @4102444800 {} +; \
    du -sh "$GOMODCACHE" "$GOCACHE"

# Native provider CLIs the profile manifest pins by URL and SHA-256 (ADR108
# D2, Muse Code). native-pins keeps only the nativeBinary entries, so an edit to
# another profile leaves native-binaries' input unchanged: the registry build
# cache serves the download, and the linked COPY below keeps the layer, until
# a pin changes.
FROM --platform=$BUILDPLATFORM node:22.23.4-bookworm-slim@sha256:de170b39439eb0e5a3444ad7d60b015614a381e10fecae2e30ec9e735122f38d AS native-pins
WORKDIR /build
COPY backend/internal/agentsession/agent-profiles.json ./
RUN node -e 'const fs = require("fs"); const { profiles } = JSON.parse(fs.readFileSync("agent-profiles.json", "utf8")); fs.writeFileSync("native-pins.json", JSON.stringify({ profiles: profiles.filter((p) => p.nativeBinary).map(({ id, nativeBinary }) => ({ id, nativeBinary })) }))'

FROM --platform=$BUILDPLATFORM node:22.23.4-bookworm-slim@sha256:de170b39439eb0e5a3444ad7d60b015614a381e10fecae2e30ec9e735122f38d AS native-binaries
ARG TARGETARCH
WORKDIR /build
COPY agent-image/fetch-native-binaries.mjs ./
COPY --from=native-pins /build/native-pins.json ./
RUN node fetch-native-binaries.mjs native-pins.json /native "${TARGETARCH}"

# Python 3.12 next to Debian's 3.11 (w10/m513): the official build for the
# same Debian release, so the same glibc, OpenSSL and shared libraries. Only
# its versioned names are copied (python3.12, pip3.12), so `python3` stays
# bookworm's 3.11. The interpreter finds libpython through its $ORIGIN/../lib
# run path, and uv finds it on PATH for `--python 3.12`.
FROM python:${PYTHON312_VERSION}-slim-bookworm@sha256:935c3c5b12357a7e10caa9c1e6ca5692344d97cf229832ef584be2cec9fc03df AS python-312

# gitleaks, built from its module with this image's Go toolchain (w10/m513):
# the release binary is built with Go 1.24, whose standard library carries
# fixed CRITICAL findings deploy.yml's Trivy gate refuses. The module is
# pinned by its go.sum hash, checked before the build, as well as by the
# checksum database go install consults.
FROM --platform=$BUILDPLATFORM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS gitleaks
ARG TARGETARCH
ARG GITLEAKS_VERSION
ENV CGO_ENABLED=0 GOFLAGS=-trimpath GOTOOLCHAIN=local
RUN set -e; \
    module=github.com/zricethezav/gitleaks/v8; \
    sum="$(go mod download -json "$module@v$GITLEAKS_VERSION" | sed -n 's/^\t"Sum": "\(.*\)",$/\1/p')"; \
    [ "$sum" = "h1:PmEvCfVI7ti9dV3s5aMZUY7sS2GxRvG3yzih7E+cS3w=" ] \
      || { echo "gitleaks $GITLEAKS_VERSION module sum is '$sum', not the pinned one" >&2; exit 1; }; \
    GOOS=linux GOARCH="$TARGETARCH" go install -ldflags "-s -w -X $module/version.Version=$GITLEAKS_VERSION" "$module@v$GITLEAKS_VERSION"; \
    bin="$(go env GOPATH)/bin"; [ -f "$bin/gitleaks" ] || bin="$bin/linux_$TARGETARCH"; \
    install -D -m 0755 "$bin/gitleaks" /out/usr/local/bin/gitleaks

# uv/uvx and gh (w10/m513): release tarballs for the target architecture,
# refused unless their SHA-256 matches the pin.
FROM --platform=$BUILDPLATFORM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS cli-tools
ARG TARGETARCH
ARG UV_VERSION
ARG GH_VERSION
WORKDIR /build
RUN set -e; \
    case "$TARGETARCH" in \
      amd64) uv_arch=x86_64; \
             uv_sha=a7fbaff2b9a271d85c1bdb540941090fd1ce011797b4abeb48e4dcd39c2471d5; \
             gh_sha=79e7ab5db738c48777b696e374f8b428f0647debfb424f44d2f426370407fe2f ;; \
      arm64) uv_arch=aarch64; \
             uv_sha=0797542018e541a9288855df555c8077eed06f7a47d206362030480e8ecc9f3d; \
             gh_sha=b1278c75107423d818f1937fd92cafc9e5e9b606631568f68d7fb816f5885a02 ;; \
      *) echo "no pinned uv or gh for $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    fetch() { curl -fsSL --retry 3 --retry-all-errors -o "$2" "$1" && echo "$3  $2" | sha256sum -c -; }; \
    fetch "https://github.com/astral-sh/uv/releases/download/$UV_VERSION/uv-$uv_arch-unknown-linux-gnu.tar.gz" uv.tar.gz "$uv_sha"; \
    fetch "https://github.com/cli/cli/releases/download/v$GH_VERSION/gh_${GH_VERSION}_linux_$TARGETARCH.tar.gz" gh.tar.gz "$gh_sha"; \
    mkdir -p /out/usr/local/bin; \
    tar -xzf uv.tar.gz -C /out/usr/local/bin --no-same-owner --strip-components=1 "uv-$uv_arch-unknown-linux-gnu/uv" "uv-$uv_arch-unknown-linux-gnu/uvx"; \
    tar -xzf gh.tar.gz -C /out/usr/local/bin --no-same-owner --strip-components=2 "gh_${GH_VERSION}_linux_$TARGETARCH/bin/gh"

# DeepSeek Harness needs Node 24 (ADR114 D2). Its profile's nodeRuntime runs
# the adapter on this binary; /usr/local/bin/node, the agent's shell's Node,
# stays the image's 22. Same Debian release, so the same glibc and libstdc++.
FROM node:24.22.2-bookworm-slim@sha256:01d566e8d94d8c619591d78652babfccd3c6992a3dffa9ed5ab39e11cf6c70e9 AS node-24

FROM node:22.23.4-bookworm-slim@sha256:de170b39439eb0e5a3444ad7d60b015614a381e10fecae2e30ec9e735122f38d

ARG NPM_VERSION=11.19.0
# deploy.yml passes the commit, so every layer from here on rebuilds on each
# deploy and apt packages refresh, while the registry build cache serves the
# Go dependency stage.
ARG AGENT_REBUILD=

RUN echo "agent image rebuild ${AGENT_REBUILD}" \
    && apt-get update \
    && apt-get install --no-install-recommends -y \
        build-essential \
        ca-certificates \
        curl \
        git \
        openssh-client \
        openssh-sftp-server \
        python3 \
        python3-pip \
        python3-venv \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-toolchain /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

ARG PYTHON312_VERSION
ARG UV_VERSION
ARG GITLEAKS_VERSION
ARG GH_VERSION
COPY --link --from=python-312 /usr/local/bin/python3.12 /usr/local/bin/pip3.12 /usr/local/bin/
COPY --link --from=python-312 /usr/local/lib/libpython3.12.so.1.0 /usr/local/lib/libpython3.so /usr/local/lib/
COPY --link --from=python-312 /usr/local/lib/python3.12 /usr/local/lib/python3.12
COPY --link --from=python-312 /usr/local/include/python3.12 /usr/local/include/python3.12
COPY --link --from=cli-tools /out/ /
COPY --link --from=gitleaks /out/ /
# The build fails unless every baked tool reports its pin, python3 is still
# Debian's 3.11, the 3.12 interpreter has its extension modules, pip and
# venv, and uv finds it with downloads disabled (as a registries-tier session
# without the release-asset host would). The check writes no bytecode, so this
# per-deploy layer stays small.
RUN set -e; \
    export PYTHONDONTWRITEBYTECODE=1; \
    ln -s libpython3.12.so.1.0 /usr/local/lib/libpython3.12.so; \
    ldconfig; \
    python3 --version | grep -Eqx 'Python 3\.11\.[0-9]+'; \
    [ "$(python3.12 --version)" = "Python $PYTHON312_VERSION" ]; \
    python3.12 -c 'import bz2, ctypes, curses, decimal, hashlib, lzma, readline, sqlite3, ssl, uuid, xml.parsers.expat, zlib'; \
    python3.12 -m pip --version; \
    python3.12 -m venv /tmp/venv312; /tmp/venv312/bin/python -m pip --version; rm -rf /tmp/venv312; \
    [ "$(uv --version | cut -d' ' -f2)" = "$UV_VERSION" ]; \
    [ "$(uvx --version | cut -d' ' -f2)" = "$UV_VERSION" ]; \
    UV_PYTHON_DOWNLOADS=never UV_CACHE_DIR=/tmp/uv-cache uv venv --python 3.12 /tmp/uv-venv; \
    [ "$(/tmp/uv-venv/bin/python --version)" = "Python $PYTHON312_VERSION" ]; \
    rm -rf /tmp/uv-venv /tmp/uv-cache; \
    [ "$(gitleaks version)" = "$GITLEAKS_VERSION" ]; \
    gh --version | head -n 1 | grep -q "^gh version $GH_VERSION "

RUN node -e "if(!/^\\d+\\.\\d+\\.\\d+$/.test(process.argv[1]))throw Error('npm must be exactly pinned')" "${NPM_VERSION}" \
    && npm install --global --ignore-scripts "npm@${NPM_VERSION}" \
    && npm cache clean --force

WORKDIR /opt/bex-agent-driver
COPY agent-image/driver/package.json agent-image/driver/package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts \
    && npm cache clean --force
COPY --from=driver-build /opt/bex-agent-driver/dist ./dist
COPY backend/internal/agentsession/agent-profiles.json ./agent-profiles.json
COPY --link --from=native-binaries /native/ /
COPY --link --from=node-24 /usr/local/bin/node /opt/node24/bin/node
RUN chmod 0755 dist/main.js \
    && ln -s /opt/bex-agent-driver/dist/main.js /usr/local/bin/bex-agent-driver \
    && node dist/install-profiles.js

# /attachments is where the agent finds the session's uploaded files
# (ADR095 D4). bex-api extracts them through the gateway as one new directory,
# /var/lib/bex-agent/attachments, which must not exist beforehand; the link
# dangles until then. It sits outside /workspace so the files are never
# committed. /bex/skills is the workspace's skills (ADR099 D5), extracted the
# same way into /var/lib/bex-agent/skills.
RUN useradd --create-home --home-dir /home/bex --uid 10001 --shell /bin/bash bex \
    && mkdir -p /workspace /var/lib/bex-agent /run/bex-agent /var/log/bex-agent /var/run/bex-agent /bex /var/tmp/go-tmp \
    && ln -s /var/lib/bex-agent/attachments /attachments \
    && ln -s /var/lib/bex-agent/skills /bex/skills \
    && chown -R bex:bex /workspace /var/lib/bex-agent /run/bex-agent /var/log/bex-agent /var/run/bex-agent /home/bex /var/tmp/go-tmp

# A linked layer does not depend on the layers above it, so the registry build
# cache reuses it, and a node keeps it, until the dependency list changes; the
# per-deploy layers never pull it.
COPY --link --from=go-deps --chown=10001:10001 /opt/bex-go /opt/bex-go

# Neither listener has an edge route. OpenSandbox supplies the remaining
# non-secret BEX_AGENT_* session binding values per sandbox.
# GOTMPDIR and TMPDIR keep temporary files on the rootfs, which the
# ephemeral-storage limit bounds: under gVisor /tmp is a tmpfs that counts
# against the sandbox's memory limit. A full build and test of a large
# repository there ran an 8 GiB sandbox out of memory (w6/124). GOTMPDIR holds
# the go command's work directory (compiled packages, link outputs, test
# binaries). TMPDIR covers everything that asks the OS, including Node's
# os.tmpdir() and so Claude Code's own temp files and background-task output
# (w6/127).
ENV HOME=/home/bex \
    GOMODCACHE=/opt/bex-go/mod \
    GOCACHE=/opt/bex-go/cache \
    GOTMPDIR=/var/tmp/go-tmp \
    TMPDIR=/var/tmp \
    BEX_AGENT_CWD=/workspace \
    BEX_AGENT_LISTEN_HOST=0.0.0.0 \
    BEX_AGENT_LISTEN_PORT=8787

USER 10001:10001
WORKDIR /workspace
EXPOSE 8787
ENTRYPOINT ["bex-agent-driver"]
