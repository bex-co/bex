#!/usr/bin/env bash
# Self-test for fetch-app-kubeconfig.sh, with curl/ssh/kubectl stubbed on PATH.
# No network, no cluster, no real credentials: every token below is a dummy.
#   * the mid-roll retry (w6/m153);
#   * the repository-root .env fallback for HCLOUD_TOKEN (w21/097).
set -euo pipefail
cd "$(dirname "$0")/.."

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
bin="$work/bin"
mkdir -p "$bin"

# Each stub appends what it was called with to $CURL_LOG / $SSH_LOG and the
# environment it saw to $ENV_LOG (all unset => /dev/null).
# Two running control planes; the Hetzner API is the only curl call.
cat > "$bin/curl" <<'STUB'
#!/usr/bin/env bash
echo "$*" >> "${CURL_LOG:-/dev/null}"
env >> "${ENV_LOG:-/dev/null}"
echo '{"servers":[
 {"name":"bex-control-plane-a","status":"running","created":"2026-10-24T06:00:00Z","public_net":{"ipv4":{"ip":"10.0.0.1"}}},
 {"name":"bex-control-plane-b","status":"running","created":"2026-10-24T06:01:00Z","public_net":{"ipv4":{"ip":"10.0.0.2"}}}]}'
STUB
# ssh fails host-key verification until $SSH_OK_AFTER calls have been made
# (counted in $SSH_COUNT); SSH_MODE=refused fails with a plain connect error.
cat > "$bin/ssh" <<'STUB'
#!/usr/bin/env bash
echo "$*" >> "${SSH_LOG:-/dev/null}"
env >> "${ENV_LOG:-/dev/null}"
n=$(( $(cat "$SSH_COUNT" 2>/dev/null || echo 0) + 1 ))
echo "$n" > "$SSH_COUNT"
if [ "${SSH_MODE:-}" = refused ]; then
  echo "ssh: connect to host port 22: Connection refused" >&2
  exit 255
fi
if [ "$n" -le "${SSH_OK_AFTER:-0}" ]; then
  echo "Host key verification failed." >&2
  exit 255
fi
echo "apiVersion: v1"
STUB
printf '#!/usr/bin/env bash\nenv >> "${ENV_LOG:-/dev/null}"\nexit 0\n' > "$bin/kubectl"
chmod +x "$bin"/*

known="$work/known_hosts"
echo "10.0.0.9 ssh-ed25519 AAAA" > "$known"
run() {
  : > "$work/count"
  PATH="$bin:$PATH" HCLOUD_TOKEN=t BEX_SSH_KEY_PATH="$work/key" \
    BEX_SSH_KNOWN_HOSTS_FILE="$known" SSH_COUNT="$work/count" \
    BEX_KUBECONFIG_FETCH_RETRY_SECONDS=0 BEX_KUBECONFIG_FETCH_ATTEMPTS=3 \
    "$@" bash scripts/fetch-app-kubeconfig.sh "$work/out.kubeconfig" > "$work/stdout" 2> "$work/stderr"
}
fail() { echo "FAIL: $*"; cat "$work/stderr"; exit 1; }

# A roll that settles: both nodes mismatch on attempt 1 (2 calls), then the
# first node of attempt 2 is pinned again.
rm -f "$work/out.kubeconfig"
run env SSH_OK_AFTER=2 || fail "a roll that settles should succeed"
grep -q "a roll may be in progress" "$work/stderr" || fail "no mid-roll retry notice"
[ -s "$work/out.kubeconfig" ] || fail "no kubeconfig written"
[ "$(cat "$work/count")" = 3 ] || fail "want 3 ssh calls, got $(cat "$work/count")"

# A stale pin: every attempt mismatches; bounded, then one named error.
if run env SSH_OK_AFTER=99; then fail "a stale pin must fail"; fi
grep -q "still do not match the pinned BEX_SSH_KNOWN_HOSTS after 3 attempt" "$work/stderr" || fail "no stale-pin error"
[ "$(cat "$work/count")" = 6 ] || fail "want 3 attempts x 2 nodes = 6 ssh calls, got $(cat "$work/count")"

# A plain connection failure is not a roll: no retry, no stale-pin claim.
if run env SSH_MODE=refused; then fail "refused connections must fail"; fi
grep -q "a roll may be in progress" "$work/stderr" && fail "retried a non-host-key failure"
grep -q "still do not match" "$work/stderr" && fail "blamed the pin for a refused connection"
[ "$(cat "$work/count")" = 2 ] || fail "want one pass of 2 ssh calls, got $(cat "$work/count")"

echo "ok: retries only mid-roll host-key failures, bounded, and names a stale pin"

# --- the repository-root .env fallback (w21/097) ----------------------------
# The script resolves .env from its own location, so run a copy of it inside a
# fake repository (never the developer's real .env) from an unrelated cwd that
# has its own decoy .env.
token=dummy-hcloud-token-0123456789
other=dummy-other-hcloud-token-9876543210
fake="$work/repo"
elsewhere="$work/elsewhere"
mkdir -p "$fake/scripts/lib" "$elsewhere"
cp scripts/fetch-app-kubeconfig.sh "$fake/scripts/"
cp scripts/lib/env-file.sh scripts/lib/ssh-hostkey.sh "$fake/scripts/lib/"
printf 'HCLOUD_TOKEN=%s\n' "$other" > "$elsewhere/.env"
chmod 600 "$elsewhere/.env"
envkey="$work/env-key"
envknown="$work/env-known_hosts"
echo "10.0.0.8 ssh-ed25519 AAAA" > "$envknown"

# write_env <mode> <line>...: the fake repository's .env.
write_env() {
  local mode="$1"; shift
  printf '%s\n' "$@" > "$fake/.env"
  chmod "$mode" "$fake/.env"
}
# run_env [NAME=value ...]: the caller's environment never has a token or SSH
# setting of its own unless the case passes it.
run_env() {
  : > "$work/count"; : > "$work/curl.log"; : > "$work/ssh.log"; : > "$work/env.log"
  rm -f "$work/out.kubeconfig"
  ( cd "$elsewhere" && env -u HCLOUD_TOKEN -u BEX_SSH_KEY_PATH -u BEX_SSH_PRIVATE_KEY_FILE \
      -u BEX_SSH_KNOWN_HOSTS_FILE -u BEX_SSH_REQUIRE_KNOWN_HOSTS \
      PATH="$bin:$PATH" SSH_COUNT="$work/count" CURL_LOG="$work/curl.log" \
      SSH_LOG="$work/ssh.log" ENV_LOG="$work/env.log" "$@" \
      bash "$fake/scripts/fetch-app-kubeconfig.sh" "$work/out.kubeconfig" ) > "$work/stdout" 2> "$work/stderr"
}
# No run may print a token, whether it succeeded or not (checked without
# echoing the output, so a failure here cannot leak it either).
no_leak() {
  if grep -qF -e "$token" -e "$other" "$work/stdout" "$work/stderr"; then
    echo "FAIL: $1 — the token reached stdout/stderr"; exit 1
  fi
}
used_token() { grep -qF "Bearer $1" "$work/curl.log"; }

# Unset + .env: the token is loaded and sent to the Hetzner API, nothing else
# in the file reaches the children, and the token itself is not exported.
write_env 600 "HCLOUD_TOKEN=$token" "BEX_TEST_UNRELATED=leaked-value" \
  "KUBECONFIG=/nonexistent/leaked.kubeconfig" "HTTPS_PROXY=http://127.0.0.1:9"
run_env BEX_SSH_KNOWN_HOSTS_FILE="$known" || fail "an unset token with a .env should load it"
no_leak "token unset, .env"
used_token "$token" || fail ".env's token was not sent to the Hetzner API"
used_token "$other" && fail "the cwd's .env was read instead of the repository root's"
[ -s "$work/out.kubeconfig" ] || fail "no kubeconfig written after loading .env"
[ -s "$work/env.log" ] || fail "the stubs recorded no environment"
grep -q -e '^BEX_TEST_UNRELATED=' -e 'leaked.kubeconfig' -e '^HTTPS_PROXY=' "$work/env.log" \
  && fail "an unrelated .env variable reached curl/ssh/kubectl"
grep -q '^HCLOUD_TOKEN=' "$work/env.log" && fail "the token loaded from .env was exported to the children"
echo "ok: an unset token is loaded from the repository-root .env, from any cwd, and nothing else leaks"

# An exported token wins and .env is not even consulted: a different token in
# the file is ignored, and so is a file the guard would refuse.
write_env 600 "HCLOUD_TOKEN=$other"
run_env HCLOUD_TOKEN="$token" BEX_SSH_KNOWN_HOSTS_FILE="$known" || fail "an exported token should be used"
no_leak "token exported, different .env"
used_token "$token" || fail "the exported token was not sent"
used_token "$other" && fail ".env overrode an exported token"
write_env 644 "HCLOUD_TOKEN=$other"
run_env HCLOUD_TOKEN="$token" BEX_SSH_KNOWN_HOSTS_FILE="$known" || fail "an exported token must not depend on .env's mode"
no_leak "token exported, 0644 .env"
used_token "$token" || fail "the exported token was not sent with a 0644 .env"
rm -f "$fake/.env"
run_env HCLOUD_TOKEN="$token" BEX_SSH_KNOWN_HOSTS_FILE="$known" || fail "CI's call (token exported, no .env) must be unchanged"
no_leak "token exported, no .env"
used_token "$token" || fail "CI's call did not send the token"
echo "ok: an exported token is used as is; .env is never read (CI's call is unchanged)"

# Unset and nothing to load: the same error as before, and no API call.
rm -f "$fake/.env"
if run_env BEX_SSH_KNOWN_HOSTS_FILE="$known"; then fail "no token and no .env must fail"; fi
no_leak "no token, no .env"
grep -q "HCLOUD_TOKEN is required" "$work/stderr" || fail "no 'HCLOUD_TOKEN is required' error without a .env"
[ ! -s "$work/curl.log" ] || fail "called the Hetzner API without a token"
write_env 600 "BEX_TEST_UNRELATED=x" "HCLOUD_TOKEN="
if run_env BEX_SSH_KNOWN_HOSTS_FILE="$known"; then fail "an empty HCLOUD_TOKEN in .env must fail"; fi
grep -q "HCLOUD_TOKEN is required" "$work/stderr" || fail "no 'HCLOUD_TOKEN is required' error for an empty token"
[ ! -s "$work/curl.log" ] || fail "called the Hetzner API with an empty token"
echo "ok: neither an exported token nor a usable .env gives 'HCLOUD_TOKEN is required'"

# A .env other users could read is refused by the guard before anything is
# loaded or called, with the fix named and no value printed.
for mode in 640 644 604; do
  write_env "$mode" "HCLOUD_TOKEN=$token"
  if run_env BEX_SSH_KNOWN_HOSTS_FILE="$known"; then fail "a $mode .env must be refused"; fi
  no_leak "$mode .env"
  grep -qF "chmod 600 $fake/.env" "$work/stderr" || fail "the $mode refusal did not name the fix"
  [ ! -s "$work/curl.log" ] || fail "called the Hetzner API with a $mode .env"
done
echo "ok: a group- or other-readable .env is refused by the guard"

# The script's other inputs come from .env too, each only when not already set.
write_env 600 "HCLOUD_TOKEN=$token" "BEX_SSH_PRIVATE_KEY_FILE=$envkey" "BEX_SSH_KNOWN_HOSTS_FILE=$envknown"
run_env || fail ".env's key and known-hosts file should be honored"
grep -qF -e "-i $envkey" "$work/ssh.log" || fail ".env's BEX_SSH_PRIVATE_KEY_FILE was not used"
grep -qF "UserKnownHostsFile=$envknown" "$work/ssh.log" || fail ".env's BEX_SSH_KNOWN_HOSTS_FILE was not used"
run_env BEX_SSH_KEY_PATH="$work/key" BEX_SSH_KNOWN_HOSTS_FILE="$known" || fail "exported SSH inputs should win"
grep -qF -e "-i $work/key" "$work/ssh.log" || fail "BEX_SSH_KEY_PATH did not win over .env"
grep -qF "UserKnownHostsFile=$known" "$work/ssh.log" || fail "an exported BEX_SSH_KNOWN_HOSTS_FILE did not win over .env"
run_env BEX_SSH_PRIVATE_KEY_FILE="$work/exported-key" BEX_SSH_KNOWN_HOSTS_FILE="$known" || fail "an exported key file should win"
grep -qF -e "-i $work/exported-key" "$work/ssh.log" || fail "an exported BEX_SSH_PRIVATE_KEY_FILE did not win over .env"
no_leak "ssh inputs from .env"
echo "ok: .env supplies the SSH key and known-hosts file only when they are not already set"

echo "PASS: fetch-app-kubeconfig retries only mid-roll host-key failures and loads HCLOUD_TOKEN from the repository-root .env without touching an exported one"
