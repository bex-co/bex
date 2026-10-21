#!/usr/bin/env bash
# Self-test for scripts/gh-secrets.sh (w3/m173): driven by
# scripts/production-secrets.txt, it must push every production secret into
# each environment the table names and nowhere else, read file-backed values
# from NAME_FILE, abort on a missing required secret, and refuse a
# world-readable .env. Runs in DRY_RUN inside a scratch copy of the layout, so
# gh is never called.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fails=0

mkdir -p "$tmp/scripts/lib"
cp "$here/gh-secrets.sh" "$tmp/scripts/"
cp "$here/lib/env-file.sh" "$tmp/scripts/lib/"
printf '%s\n' '# fixture' 'TOKEN_A production-deploy,production-observe' 'KEY_B production-release' \
  'BEX_ONBEX_TLS_CERT production-deploy' 'BEX_ONBEX_TLS_KEY production-deploy' \
  'BEX_SSH_PUBLIC_KEY production-infra' 'BEX_SSH_PRIVATE_KEY production-deploy,production-cluster' \
  'OPTIONAL_C production-observe' >"$tmp/scripts/production-secrets.txt"
for f in cert key pub priv kb; do printf 'material-%s\n' "$f" >"$tmp/$f"; done
write_env() {
  printf '%s\n' 'TOKEN_A=alpha' "KEY_B_FILE=$tmp/kb" "BEX_ONBEX_TLS_CERT_FILE=$tmp/cert" \
    "BEX_ONBEX_TLS_KEY_FILE=$tmp/key" "BEX_SSH_PUBLIC_KEY_FILE=$tmp/pub" "$1" >"$tmp/.env"
  chmod 600 "$tmp/.env"
}
run() { DRY_RUN=1 bash "$tmp/scripts/gh-secrets.sh" 2>&1; }
expect() { # <label> <output> <needle>
  if printf '%s' "$2" | grep -qF -- "$3"; then echo "ok: $1"; else echo "FAIL: $1 — missing '$3' in: $2" >&2; fails=$((fails + 1)); fi
}

write_env "BEX_SSH_PRIVATE_KEY_FILE=$tmp/priv"
out="$(run)"
expect "scalar pushed to first environment" "$out" "set   TOKEN_A (environment: production-deploy)"
expect "scalar pushed to second environment" "$out" "set   TOKEN_A (environment: production-observe)"
expect "file-backed value read from NAME_FILE" "$out" "set   KEY_B  (from $tmp/kb) (environment: production-release)"
expect "multi-environment file secret pushed to each" "$out" "set   BEX_SSH_PRIVATE_KEY  (from $tmp/priv) (environment: production-cluster)"
expect "optional empty secret skipped" "$out" "skip  OPTIONAL_C (empty in .env)"
if printf '%s\n' "$out" | grep -E '^set   ' | grep -qv '(environment: '; then
  echo "FAIL: a secret was pushed at repository scope" >&2; fails=$((fails + 1))
else echo "ok: nothing pushed at repository scope"; fi
if [ "$(printf '%s\n' "$out" | grep -c '^set   TOKEN_A')" -eq 2 ]; then echo "ok: TOKEN_A pushed exactly twice"; else echo "FAIL: TOKEN_A push count" >&2; fails=$((fails + 1)); fi
expect "dry run says nothing was pushed" "$out" "DRY_RUN — nothing was pushed"

write_env "UNRELATED=1"
set +e; out="$(run)"; rc=$?; set -e
[ "$rc" -ne 0 ] && expect "missing required secret aborts" "$out" "BEX_SSH_PRIVATE_KEY is required" \
  || { echo "FAIL: missing required secret exited 0" >&2; fails=$((fails + 1)); }

write_env "BEX_SSH_PRIVATE_KEY_FILE=$tmp/priv"; chmod 644 "$tmp/.env"
set +e; out="$(run)"; rc=$?; set -e
[ "$rc" -ne 0 ] && expect "world-readable .env refused" "$out" "chmod 600" \
  || { echo "FAIL: world-readable .env accepted" >&2; fails=$((fails + 1)); }

[ "$fails" -eq 0 ] && echo "PASS: gh-secrets.sh" || { echo "FAIL: $fails case(s)" >&2; exit 1; }
