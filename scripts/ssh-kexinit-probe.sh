#!/usr/bin/env bash
set -euo pipefail

# w6/m132/t004 — the cheap, always-on SSH liveness synthetic.
#
# A total loss of the SSH handshake went unnoticed for weeks because the thorough
# acceptance script (scripts/ssh-verify.sh) was never wired to a schedule. This
# is the minimal companion that CAN run continuously: it needs no key, no
# fixture, and no eligible service, because the failure it catches
# (banner written, then no SSH_MSG_KEXINIT — w6/m132) is pre-authentication and
# hits every connection alike.
#
# It drives the same, tested ProbeKEXINIT the gateway suite covers, so the guard
# and its unit tests can never drift. Exit 0 = the edge reached KEXINIT; non-zero
# = it did not (the .github/workflows/ssh-edge-liveness.yml scheduled run then
# fails loudly and opens a tracking issue).
#
# Usage: scripts/ssh-kexinit-probe.sh [host:port]   (default ssh.bex.co:22)

address="${1:-ssh.bex.co:22}"
repo_root="$(cd "$(dirname "$0")/.." && pwd)"

cd "$repo_root/lego/backend"
# GOWORK=off matches the module's CI (lego/go.work pins a newer toolchain for the
# cli module; the backend resolves standalone via its types/ replace).
GOWORK=off BEX_TEST_SSH_KEXINIT_ADDR="$address" \
  exec go test ./internal/sshgateway/nativessh \
  -run '^TestPublicGatewayKEXINITLiveness$' -count=1
