#!/usr/bin/env bash
# Unit tests for the OpenBao prod-wiring scripts (docs/ADR013-secrets.md, w1/m10):
#   1. bao-init.sh's set_env_var — the in-file .env editor that writes the
#      Shamir unseal keys + root token. A regression here (dropping a value,
#      echoing it, clobbering an unrelated key, or failing to replace) is
#      silent and catastrophic, so it gets a real test. The main guard in
#      bao-init.sh keeps sourcing side-effect-free, so this sources the script
#      and calls the function directly against a throwaway .env.
#   2. The BAO_* keys are mirrored in both env templates and pushed by
#      gh-secrets.sh (the t001/t002 wiring this milestone added).
#
# Pure bash — no cluster, no kubectl, no curl.
#   bash scripts/bao-init.test.sh   # exits 0 on pass
set -euo pipefail
cd "$(dirname "$0")/.."

fails=0
assert() { # desc  condition
  if eval "$2"; then echo "    ok: $1"; else echo "FAIL: $1" >&2; fails=$((fails + 1)); fi
}

echo "==> set_env_var (bao-init.sh)"
# shellcheck disable=SC1091 — main guard means sourcing defines set_env_var only
source scripts/bao-init.sh

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

# 1a. insert into a non-existent .env (creates it); value never reaches stdout
out="$(set_env_var BAO_ROOT_TOKEN s3cret-value 2>/dev/null)"; rc=$?
assert "insert creates .env and exits 0" "[ $rc -eq 0 ]"
assert "insert wrote the key=value" "[ -f .env ] && grep -q '^BAO_ROOT_TOKEN=s3cret-value$' .env"
assert "insert printed nothing to stdout" "[ -z \"\$out\" ]"

# 1b. append a second key without disturbing the first
set_env_var BAO_UNSEAL_KEY_1 k1 >/dev/null
assert "append preserved the prior key" "grep -q '^BAO_ROOT_TOKEN=s3cret-value$' .env"
assert "append added the new key" "grep -q '^BAO_UNSEAL_KEY_1=k1$' .env"

# 1c. update an existing key in place: replace, not duplicate
out="$(set_env_var BAO_ROOT_TOKEN rotated-value)"
assert "update replaced the value" "grep -q '^BAO_ROOT_TOKEN=rotated-value$' .env"
assert "update left exactly one line for the key" "[ \"\$(grep -c '^BAO_ROOT_TOKEN=' .env)\" -eq 1 ]"
assert "update left the unrelated key intact" "grep -q '^BAO_UNSEAL_KEY_1=k1$' .env"
assert "update printed nothing to stdout" "[ -z \"\$out\" ]"

# 1d. a value containing '=' round-trips (the -F= awk split must not mangle it)
set_env_var TRICKY 'a=b=c' >/dev/null
assert "value with '=' round-trips verbatim" "grep -q '^TRICKY=a=b=c$' .env"

# 1e. a key whose name is a prefix of another is not matched
set_env_var BAO_UNSEAL_KEY_1 v1 >/dev/null
set_env_var BAO_UNSEAL_KEY_10 v10 >/dev/null
assert "prefix-distinct key added separately" "grep -q '^BAO_UNSEAL_KEY_10=v10$' .env"
set_env_var BAO_UNSEAL_KEY_1 updated-v1 >/dev/null
assert "updating the short key leaves the longer one alone" "grep -q '^BAO_UNSEAL_KEY_10=v10$' .env"
assert "updating the short key changed only it" "grep -q '^BAO_UNSEAL_KEY_1=updated-v1$' .env"

# 1f. every write pins .env to owner-only 0600 (codex round-18 #1), including
# the awk/mv replace path that would otherwise recreate it at the umask default
assert ".env is mode 600 after writes" "[ \"\$(stat -c '%a' .env 2>/dev/null || stat -f '%Lp' .env)\" = 600 ]"
chmod 644 .env
set_env_var BAO_ROOT_TOKEN perms-check >/dev/null
assert "replace path re-pins 644 back to 600" "[ \"\$(stat -c '%a' .env 2>/dev/null || stat -f '%Lp' .env)\" = 600 ]"

cd - >/dev/null

echo "==> BAO keys declared in .env.example + pushed by gh-secrets.sh"
for k in BAO_UNSEAL_KEY_1 BAO_UNSEAL_KEY_2 BAO_UNSEAL_KEY_3 BAO_ROOT_TOKEN; do
  assert ".env.example declares $k" "grep -q '^$k=' .env.example"
  assert "gh-secrets.sh pushes $k" "grep -q '\\b$k\\b' scripts/gh-secrets.sh"
done

if [ "$fails" -eq 0 ]; then
  echo "PASS: bao-init set_env_var + env-template/gh-secrets wiring"
  exit 0
fi
echo "FAIL: $fails assertion(s) failed" >&2
exit 1
