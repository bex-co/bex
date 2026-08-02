#!/usr/bin/env bash
set -euo pipefail

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

mkdir -p "$scratch/home/bex/.config/git" "$scratch/home/bex/.cache/git/credential" "$scratch/run/bex-agent" "$scratch/tmp/bex-agent-credentials"
marker='ghs_TEST_TOKEN_DO_NOT_PERSIST'
printf '%s\n' "$marker" >"$scratch/home/bex/.git-credentials"
printf '%s\n' "$marker" >"$scratch/home/bex/.config/git/credentials"
printf '%s\n' "$marker" >"$scratch/home/bex/.cache/git/credential/socket"
printf 'OPEN-SANDBOX-API-KEY=%s\n' "$marker" >"$scratch/run/bex-agent/opensandbox-api-key"
printf '%s\n' "$marker" >"$scratch/tmp/bex-agent-credentials/token"

if ! grep -R -q "$marker" "$scratch"; then
  echo "fixture marker was not seeded" >&2
  exit 1
fi

BEX_SCRUB_ROOT="$scratch" BEX_SCRUB_HOME=/home/bex bash "$(dirname "$0")/bex-pre-snapshot"

if grep -R -q "$marker" "$scratch"; then
  echo "credential marker survived scrub" >&2
  exit 1
fi

echo "credential scrub test passed"
