# syntax=docker/dockerfile:1.7

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

RUN node -e "if(!/^\\d+\\.\\d+\\.\\d+$/.test(process.argv[1]))throw Error('npm must be exactly pinned')" "${NPM_VERSION}" \
    && npm install --global --ignore-scripts "npm@${NPM_VERSION}" \
    && npm cache clean --force

WORKDIR /opt/bex-agent-driver
COPY agent-image/driver/package.json agent-image/driver/package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts \
    && npm cache clean --force
COPY --from=driver-build /opt/bex-agent-driver/dist ./dist
COPY backend/internal/agentsession/agent-profiles.json ./agent-profiles.json
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
# GOTMPDIR keeps the go command's work directory (compiled packages, link
# outputs, test binaries) on the rootfs: under gVisor /tmp is a tmpfs that
# counts against the sandbox's memory limit, and a full build and test of a
# large repository there ran an 8 GiB sandbox out of memory (w6/124).
ENV HOME=/home/bex \
    GOMODCACHE=/opt/bex-go/mod \
    GOCACHE=/opt/bex-go/cache \
    GOTMPDIR=/var/tmp/go-tmp \
    BEX_AGENT_CWD=/workspace \
    BEX_AGENT_LISTEN_HOST=0.0.0.0 \
    BEX_AGENT_LISTEN_PORT=8787

USER 10001:10001
WORKDIR /workspace
EXPOSE 8787
ENTRYPOINT ["bex-agent-driver"]
