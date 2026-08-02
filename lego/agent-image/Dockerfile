# syntax=docker/dockerfile:1.7

FROM golang:1.25.7-bookworm AS credential-helper

WORKDIR /workspace
COPY backend/go.mod backend/go.sum ./backend/
COPY types/ ./types/
RUN cd backend && go mod download
COPY backend/ ./backend/
RUN cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/git-credential-bex ./cmd/git-credential-bex

FROM node:22-bookworm-slim

ARG CLAUDE_CODE_ACP_VERSION=0.16.2
ARG NPM_VERSION=11.19.0

RUN apt-get update \
    && apt-get install --no-install-recommends -y \
        build-essential \
        ca-certificates \
        curl \
        git \
        openssh-client \
        python3 \
        python3-pip \
        python3-venv \
    && rm -rf /var/lib/apt/lists/*

COPY --from=credential-helper /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

RUN npm install --global --ignore-scripts "npm@${NPM_VERSION}" \
    && npm install --global --ignore-scripts "@zed-industries/claude-code-acp@${CLAUDE_CODE_ACP_VERSION}" \
    && npm cache clean --force

WORKDIR /opt/bex-agent-driver
COPY agent-image/driver/package.json agent-image/driver/package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts \
    && npm cache clean --force
COPY agent-image/driver/src ./src
RUN chmod 0755 src/main.mjs \
    && ln -s /opt/bex-agent-driver/src/main.mjs /usr/local/bin/bex-agent-driver

COPY --from=credential-helper /out/git-credential-bex /usr/local/bin/git-credential-bex
COPY --chmod=0755 agent-image/bex-pre-snapshot /usr/local/bin/bex-pre-snapshot
RUN git config --system credential.helper bex \
    && git config --system credential.useHttpPath true \
    && useradd --create-home --home-dir /home/bex --uid 10001 --shell /bin/bash bex \
    && mkdir -p /workspace /run/bex-agent /tmp/bex-agent-credentials /var/log/bex-agent /var/run/bex-agent \
    && chown -R bex:bex /workspace /run/bex-agent /tmp/bex-agent-credentials /var/log/bex-agent /var/run/bex-agent /home/bex

# Neither listener has an edge route. OpenSandbox supplies the remaining
# non-secret BEX_AGENT_* session binding values per sandbox.
ENV HOME=/home/bex \
    BEX_AGENT_CREDENTIAL_URL=http://bex-ssh-gateway.bex-system.svc.cluster.local:8082/session-credential \
    BEX_AGENT_COMMAND=claude-code-acp \
    BEX_AGENT_ARGS='[]' \
    BEX_AGENT_CWD=/workspace \
    BEX_AGENT_LISTEN_HOST=0.0.0.0 \
    BEX_AGENT_LISTEN_PORT=8787

USER 10001:10001
WORKDIR /workspace
EXPOSE 8787
ENTRYPOINT ["bex-agent-driver"]
