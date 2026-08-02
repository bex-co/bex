# Credential-enabled base for ADR047's agent template. w3/m37 layers the session
# driver, toolchains, and pluggable ACP agent onto this contract; this milestone
# owns the git helper and snapshot hygiene pieces.
FROM golang:1.25 AS credential-helper

WORKDIR /workspace
COPY backend/go.mod backend/go.sum ./backend/
COPY types/ ./types/
RUN cd backend && go mod download
COPY backend/ ./backend/
RUN cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/git-credential-bex ./cmd/git-credential-bex

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 --shell /bin/sh bex \
    && mkdir -p /workspace /run/bex-agent /tmp/bex-agent-credentials \
    && chown -R bex:bex /workspace /run/bex-agent /tmp/bex-agent-credentials
COPY --from=credential-helper /out/git-credential-bex /usr/local/bin/git-credential-bex
COPY --chmod=0755 agent-image/bex-pre-snapshot /usr/local/bin/bex-pre-snapshot
RUN git config --system credential.helper bex \
    && git config --system credential.useHttpPath true

# This Service has no edge route. The session-create path supplies the remaining
# non-secret BEX_AGENT_* binding values per sandbox.
ENV HOME=/home/bex \
    BEX_AGENT_CREDENTIAL_URL=http://bex-ssh-gateway.bex-system.svc.cluster.local:8082/session-credential
USER bex
WORKDIR /workspace
ENTRYPOINT ["sleep", "infinity"]
