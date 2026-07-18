#!/usr/bin/env bash
set -euo pipefail

# OpenSSH shim used only by scripts/ssh-verify.sh's official Render CLI check.
# The unmodified CLI still resolves and invokes an executable named `ssh`; the
# verifier puts a symlink to this file first on PATH so it can pin the public
# host key and record only the non-secret destination selected by the CLI.
: "${BEX_SSH_VERIFY_REAL_SSH:?missing real OpenSSH path}"
: "${BEX_SSH_VERIFY_CLI_KNOWN_HOSTS:?missing verifier known_hosts path}"
: "${BEX_SSH_VERIFY_PRIVATE_KEY_FILE:?missing verifier private-key path}"
: "${BEX_SSH_VERIFY_CLI_TARGET_LOG:?missing verifier target-log path}"

target="${1:?missing SSH destination}"
printf '%s\n' "$target" >"$BEX_SSH_VERIFY_CLI_TARGET_LOG"

exec "$BEX_SSH_VERIFY_REAL_SSH" \
  -tt \
  -o BatchMode=yes \
  -o ConnectTimeout=10 \
  -o StrictHostKeyChecking=yes \
  -o "UserKnownHostsFile=$BEX_SSH_VERIFY_CLI_KNOWN_HOSTS" \
  -o IdentitiesOnly=yes \
  -i "$BEX_SSH_VERIFY_PRIVATE_KEY_FILE" \
  "$@"
