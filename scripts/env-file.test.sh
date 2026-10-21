#!/usr/bin/env bash
# Self-test for scripts/lib/env-file.sh (w3/m173 t006, ADR079 finding 1): a
# group- or world-readable .env must be refused with the fix named, a 0600 file
# and a missing file must pass, and no script may source .env unguarded.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib/env-file.sh
. "$here/lib/env-file.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fails=0

check() { # <label> <mode-or-missing> <want-rc>
  local label="$1" mode="$2" want="$3" f="$tmp/env-$RANDOM" got err
  if [ "$mode" != missing ]; then printf 'X=1\n' >"$f"; chmod "$mode" "$f"; fi
  set +e; err="$(bex_env_file_guard "$f" 2>&1)"; got=$?; set -e
  if [ "$got" -ne "$want" ]; then echo "FAIL: $label — rc $got, want $want" >&2; fails=$((fails + 1)); return; fi
  if [ "$want" -ne 0 ] && ! printf '%s' "$err" | grep -qF "chmod 600 $f"; then
    echo "FAIL: $label — refusal did not name the fix" >&2; fails=$((fails + 1)); return
  fi
  echo "ok: $label"
}
check "0600 passes" 600 0
check "0400 passes" 400 0
check "missing file passes" missing 0
check "0644 (world-readable) refused" 644 1
check "0640 (group-readable) refused" 640 1
check "0604 (other-readable only) refused" 604 1
check "0660 (group-writable) refused" 660 1

# bex_load_env refuses before sourcing and exports what it loads.
f="$tmp/load.env"; printf 'BEX_LOAD_PROBE=loaded\n' >"$f"; chmod 644 "$f"
if (bex_load_env "$f" 2>/dev/null); then echo "FAIL: bex_load_env loaded a 0644 file" >&2; fails=$((fails + 1)); else echo "ok: bex_load_env refuses a 0644 file"; fi
chmod 600 "$f"
got="$(bex_load_env "$f" && bash -c 'printf %s "${BEX_LOAD_PROBE:-}"')"
if [ "$got" = loaded ]; then echo "ok: bex_load_env exports loaded assignments"; else echo "FAIL: bex_load_env did not export (got '$got')" >&2; fails=$((fails + 1)); fi

# No script loads .env except through bex_load_env.
direct="$(cd "$here/.." && grep -nE '(^|[;&|[:space:]])(\.|source)[[:space:]]+("?\./\.env"?|"\$repo_root/\.env"|"\$env_file"|"\$ENV_FILE")([[:space:];]|$)' scripts/*.sh scripts/lib/*.sh | grep -v '^scripts/lib/env-file.sh:' || true)"
if [ -z "$direct" ]; then echo "ok: every .env load goes through bex_load_env"; else
  echo "FAIL: .env sourced directly (use bex_load_env from scripts/lib/env-file.sh):" >&2; printf '  %s\n' "$direct" >&2; fails=$((fails + 1)); fi

[ "$fails" -eq 0 ] && echo "PASS: env-file.sh" || { echo "FAIL: $fails case(s)" >&2; exit 1; }
