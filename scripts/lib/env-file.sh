#!/usr/bin/env bash
# Refuse an operator `.env` that other local users or processes could read
# (w3/m173 t006; docs/ADR079-security-review-round18.md finding 1). `.env`
# holds the OpenBao root token and unseal keys, the Hetzner token and the
# Terraform state credentials; scripts/bao-init.sh writes it 0600, but a copy
# made any other way inherits the umask (0644) and stays world-readable. Every
# script that sources `.env` calls this guard first.
#
# This file is sourced. Load sites run from the repository root:
#   . scripts/lib/env-file.sh && bex_load_env ./.env || exit 1
#
# bex_load_env is the only sanctioned way to load .env (scripts/env-file.test.sh
# fails on a direct `source`); it exports every assignment, as the `set -a`
# blocks it replaced did. The guard alone passes a missing file (callers decide
# whether .env is optional) and fails a file with any group or other permission
# bit, naming the exact fix.
bex_env_file_guard() {
  local file="$1" mode
  [ -e "$file" ] || return 0
  # GNU stat first (Linux runners); BSD stat (macOS) when -c is unsupported.
  mode="$(stat -c '%a' "$file" 2>/dev/null || stat -f '%Lp' "$file" 2>/dev/null)" || {
    echo "ERROR: cannot read the permissions of $file" >&2
    return 1
  }
  if (( 8#$mode & 8#077 )); then
    echo "ERROR: $file is readable by other users (mode $mode); it holds production credentials." >&2
    echo "       Fix: chmod 600 $file   (docs/ADR079-security-review-round18.md finding 1)" >&2
    return 1
  fi
}

bex_load_env() {
  local file="$1" rc
  bex_env_file_guard "$file" || return 1
  set -a
  # shellcheck disable=SC1090
  . "$file"
  rc=$?
  set +a
  return "$rc"
}
