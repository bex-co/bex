# Hermetic GitHub-compatible fixture for credential-e2e-test.sh. It runs a
# token-expiring credential broker and git-http-backend under a github.com TLS
# identity on an isolated Docker network; it is never part of a shipped image.
FROM golang:1.25 AS fixture-builder

WORKDIR /workspace
COPY backend/go.mod backend/go.sum ./backend/
COPY types/ ./types/
RUN cd backend && go mod download
COPY backend/ ./backend/
RUN cd backend && CGO_ENABLED=0 go build -trimpath -o /out/agent-credential-fixture ./internal/agentcredential/testfixture

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
COPY --from=fixture-builder /out/agent-credential-fixture /usr/local/bin/agent-credential-fixture
ENTRYPOINT ["/usr/local/bin/agent-credential-fixture"]
