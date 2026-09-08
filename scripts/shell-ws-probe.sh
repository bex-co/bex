#!/usr/bin/env bash
set -euo pipefail

# w2/m90/t003 — the cheap, always-on Web Shell WS-edge liveness synthetic.
#
# A total loss of wss://ssh.bex.co/shell would otherwise go unnoticed the same
# way the SSH KEXINIT regression did (w6/m132): manifests shipped, Secret
# optional, no scheduled check. This companion needs no ticket, fixture, or
# eligible service because the failure it catches is pre-authentication — an
# unauthenticated upgrade must get the deterministic 401 "missing ticket"
# refusal (alive-but-refusing). Timeout, connection refused, TLS error, wrong
# route, or a disabled 503 are dead/unactivated and fail loudly.
#
# It drives the tested ProbeUnauthenticatedRefusal the webshell suite covers,
# so the guard and its unit tests can never drift. Exit 0 = refusal shape;
# non-zero = dead edge (ssh-edge-liveness.yml then fails the run).
#
# Usage: scripts/shell-ws-probe.sh [wss://host/shell]   (default wss://ssh.bex.co/shell)

raw_url="${1:-wss://ssh.bex.co/shell}"
repo_root="$(cd "$(dirname "$0")/.." && pwd)"

cd "$repo_root/lego/backend"
# GOWORK=off matches the module's CI (lego/go.work pins a newer toolchain for the
# cli module; the backend resolves standalone via its types/ replace).
GOWORK=off BEX_TEST_SHELL_WS_URL="$raw_url" \
  exec go test ./internal/sshgateway/webshell \
  -run '^TestPublicShellWSLiveness$' -count=1
