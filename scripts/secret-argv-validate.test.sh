#!/usr/bin/env bash
# Self-test for scripts/secret-argv-validate.sh (w3/m472), against throwaway
# fixture repositories. Every planted line is assembled from pieces at run
# time, so this file never matches the gate itself.
set -euo pipefail
cd "$(dirname "$0")/.."
validator="$PWD/scripts/secret-argv-validate.sh"

work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT
fails=0
ok() { echo "    ok: $1"; }
bad() {
  echo "FAIL: $1" >&2
  fails=$((fails + 1))
}

# The names, built so no line here expands one.
hc='HCLOUD''_TOKEN'
bao='BAO_ROOT''_TOKEN'
s3='TF_STATE_SECRET''_KEY'

repo="$work/repo"
mkdir -p "$repo/scripts/lib"
git -C "$repo" init -q
git -C "$repo" config user.email test@example.invalid
git -C "$repo" config user.name test
# A list without HCLOUD_TOKEN: the gate covers it whether or not it is listed.
printf '# fixture\n%s production-restore\n%s production-infra # comment\n' "$bao" "$s3" > "$repo/scripts/production-secrets.txt"
# Clean forms: the helper, a header built elsewhere, the token on stdin, a
# commented-out example, an unlisted name, a longer name sharing the prefix,
# and a listed name that is a separate argument, not the flag's.
{
  printf 'source scripts/lib/curl-auth.sh\n'
  printf 'bex_curl_bearer %s -sf "https://api.hetzner.cloud/v1/servers"\n' "$hc"
  printf 'curl -sf -H "$auth" "https://example.invalid"\n'
  printf 'curl --config - <<< "header = \\"X-Vault-Token: $%s\\""\n' "$bao"
  printf '# curl -H "Authorization: Bearer $%s" (the old form)\n' "$hc"
  printf 'curl -H "Authorization: Bearer $DEV_ONLY_TOKEN" http://localhost\n'
  printf 'curl -H "Authorization: Bearer ${%s_OTHER}" http://localhost\n' "$hc"
  printf 'env -u %s curl -u user https://x/$%s\n' "$hc" "$hc"
} > "$repo/scripts/clean.sh"
# The helper is the one place allowed to build the header, whatever it names.
printf 'curl -H "Authorization: Bearer $%s" "$@"\n' "$hc" > "$repo/scripts/lib/curl-auth.sh"
git -C "$repo" add -A
git -C "$repo" commit -qm clean

run() { SECRET_ARGV_ROOT="$repo" bash "$validator" "$@" > "$work/out" 2>&1; }

echo "secret-argv-validate.sh"
if run; then ok "the clean forms pass"; else bad "the clean forms failed: $(cat "$work/out")"; fi
if run HEAD; then ok "the clean forms pass at HEAD"; else bad "the clean forms failed at HEAD"; fi

# expect_hit <description> <line>: planting the line fails the gate, which
# names the file and line; removing it passes again.
expect_hit() {
  local description="$1" line="$2"
  printf '#!/usr/bin/env bash\nset -e\n%s\n' "$line" > "$repo/scripts/planted.sh"
  git -C "$repo" add scripts/planted.sh
  if run; then
    bad "$description: not refused"
  elif grep -q "scripts/planted.sh:3:" "$work/out"; then
    ok "$description"
  else
    bad "$description: refused without naming scripts/planted.sh:3"
  fi
  git -C "$repo" rm -qf scripts/planted.sh
}

# The pre-w3/m472 fetch-app-kubeconfig.sh:77 form, continued on the next line.
expect_hit "the old fetch-app-kubeconfig.sh form" \
  "$(printf 'curl -sf -H "Authorization: Bearer $%s" \\\n  "https://api.hetzner.cloud/v1/servers"' "$hc")"
expect_hit "a braced expansion in an argument array" \
  "$(printf 'auth=(-H "Authorization: Bearer ${%s}")' "$hc")"
expect_hit "a listed name in --header=" \
  "$(printf 'curl --header="X-Vault-Token: $%s" http://127.0.0.1:8200' "$bao")"
# The flag is passed separately so the planted line is not a credential shape
# to the secret scanner (gitleaks curl-auth-user).
expect_hit "a listed name in -u user:password" \
  "$(printf 'curl %s "admin:$%s" https://example.invalid' -u "$s3")"
expect_hit "--oauth2-bearer" \
  "$(printf 'curl --oauth2-bearer "$%s" https://example.invalid' "$hc")"
expect_hit "-H at the end of a short-option cluster" \
  "$(printf 'curl -sfH "Authorization: Bearer $%s" https://example.invalid' "$hc")"
expect_hit "an unquoted expansion" \
  "$(printf 'curl -H Authorization:Bearer$%s https://example.invalid' "$hc")"

# With a tree-ish, only that commit decides: an uncommitted violation does not
# fail HEAD, a committed one does even after the working tree is fixed.
printf 'curl -H "Authorization: Bearer $%s" x\n' "$hc" > "$repo/scripts/late.sh"
git -C "$repo" add scripts/late.sh
if run HEAD; then ok "an uncommitted violation does not decide HEAD"; else bad "an uncommitted violation failed HEAD"; fi
if run; then bad "the working-tree scan missed an added violation"; else ok "the working-tree scan sees an added violation"; fi
git -C "$repo" commit -qm late
printf 'bex_curl_bearer %s x\n' "$hc" > "$repo/scripts/late.sh"
if run HEAD; then bad "a committed violation passed at HEAD"; else ok "a committed violation fails at HEAD"; fi
grep -q "^  scripts/late.sh:1:" "$work/out" && ok "the HEAD report drops the tree-ish prefix" || bad "the HEAD report kept the tree-ish prefix: $(cat "$work/out")"
if run; then ok "the fixed working tree passes"; else bad "the fixed working tree failed"; fi

# A name list the gate cannot turn into a pattern is an error, not a pass.
printf 'BAD-NAME production-deploy\n' > "$repo/scripts/production-secrets.txt"
rc=0
run || rc=$?
[ "$rc" = 2 ] && ok "a malformed name list is an error (exit 2)" || bad "a malformed name list gave exit $rc"

if [ "$fails" -ne 0 ]; then
  echo "$fails check(s) failed" >&2
  exit 1
fi
echo "PASS: secret-argv-validate refuses a production credential in a curl header or user argument"
