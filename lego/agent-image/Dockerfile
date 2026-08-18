# syntax=docker/dockerfile:1.7

FROM golang:1.25.7-bookworm AS go-toolchain

# The driver is TypeScript; this stage compiles it to plain JS so devDependencies
# (typescript) never reach the runtime image below.
FROM node:22-bookworm-slim AS driver-build

WORKDIR /opt/bex-agent-driver
COPY agent-image/driver/package.json agent-image/driver/package-lock.json ./
RUN npm ci --ignore-scripts
COPY agent-image/driver/tsconfig.json ./
COPY agent-image/driver/src ./src
RUN npm run build

FROM node:22-bookworm-slim

ARG CLAUDE_CODE_ACP_VERSION=0.16.2
ARG CODEX_ACP_VERSION=1.4.0
ARG GEMINI_CLI_VERSION=0.55.1
ARG NPM_VERSION=11.19.0

RUN apt-get update \
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

RUN npm install --global --ignore-scripts "npm@${NPM_VERSION}" \
    && npm install --global --ignore-scripts "@zed-industries/claude-code-acp@${CLAUDE_CODE_ACP_VERSION}" \
        "@agentclientprotocol/codex-acp@${CODEX_ACP_VERSION}" \
        "@google/gemini-cli@${GEMINI_CLI_VERSION}" \
    && command -v claude-code-acp \
    && command -v codex-acp \
    && command -v gemini \
    && npm cache clean --force

WORKDIR /opt/bex-agent-driver
COPY agent-image/driver/package.json agent-image/driver/package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts \
    && npm cache clean --force
COPY --from=driver-build /opt/bex-agent-driver/dist ./dist
RUN chmod 0755 dist/main.js \
    && ln -s /opt/bex-agent-driver/dist/main.js /usr/local/bin/bex-agent-driver

COPY --chmod=0755 agent-image/bex-pre-snapshot /usr/local/bin/bex-pre-snapshot
RUN useradd --create-home --home-dir /home/bex --uid 10001 --shell /bin/bash bex \
    && mkdir -p /workspace /run/bex-agent /tmp/bex-agent-credentials /var/log/bex-agent /var/run/bex-agent \
    && chown -R bex:bex /workspace /run/bex-agent /tmp/bex-agent-credentials /var/log/bex-agent /var/run/bex-agent /home/bex

# Neither listener has an edge route. OpenSandbox supplies the remaining
# non-secret BEX_AGENT_* session binding values per sandbox.
ENV HOME=/home/bex \
    BEX_AGENT_COMMAND=/usr/local/bin/claude-code-acp \
    BEX_AGENT_ARGS='[]' \
    BEX_AGENT_CWD=/workspace \
    BEX_AGENT_LISTEN_HOST=0.0.0.0 \
    BEX_AGENT_LISTEN_PORT=8787

USER 10001:10001
WORKDIR /workspace
EXPOSE 8787
ENTRYPOINT ["bex-agent-driver"]
