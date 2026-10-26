#!/usr/bin/env bash
# Self-test for fetch-app-kubeconfig.sh's mid-roll retry (w6/m153), with
# curl/ssh/kubectl stubbed on PATH. No network, no cluster.
set -euo pipefail
cd "$(dirname "$0")/.."

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
bin="$work/bin"
mkdir -p "$bin"

# Two running control planes; the Hetzner API is the only curl call.
cat > "$bin/curl" <<'STUB'
#!/usr/bin/env bash
echo '{"servers":[
 {"name":"bex-control-plane-a","status":"running","created":"2026-10-24T06:00:00Z","public_net":{"ipv4":{"ip":"10.0.0.1"}}},
 {"name":"bex-control-plane-b","status":"running","created":"2026-10-24T06:01:00Z","public_net":{"ipv4":{"ip":"10.0.0.2"}}}]}'
STUB
# ssh fails host-key verification until $SSH_OK_AFTER calls have been made
# (counted in $SSH_COUNT); SSH_MODE=refused fails with a plain connect error.
cat > "$bin/ssh" <<'STUB'
#!/usr/bin/env bash
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
printf '#!/usr/bin/env bash\nexit 0\n' > "$bin/kubectl"
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

echo "PASS: fetch-app-kubeconfig retries only mid-roll host-key failures, bounded, and names a stale pin"
