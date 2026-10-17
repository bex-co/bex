#!/usr/bin/env bash
# Self-test for scripts/edge-probe.sh (w1/131): a transient failure that a
# retry clears opens nothing, a sustained one fails, and each probe error text
# maps to the class the issue names — without contacting any edge.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=edge-probe.sh
. "$here/edge-probe.sh"

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

expect_class() { # expect_class <kind> <probe output> <want class prefix>
  printf '%s\n' "$2" >"$tmp/out"
  local got
  got="$(classify_edge_failure "$1" "$tmp/out")"
  case "$got" in
    "$3"*) ;;
    *) echo "FAIL [$1: $2] class=$got want prefix=$3" >&2; fails=$((fails + 1)) ;;
  esac
}

# The 2026-10-16 21:40Z run's real outputs.
expect_class ssh "kexinit_probe_test.go:102: live SSH edge ssh.bex.co:22 did not reach KEXINIT: read server identification: read tcp 172.18.0.5:60224->49.12.20.236:22: i/o timeout" "no SSH banner"
expect_class tls "curl: (28) Operation timed out after 20002 milliseconds with 0 bytes received" "HTTP request timed out"
expect_class ssh "live SSH edge ssh.bex.co:22 did not reach KEXINIT: EOF" "banner but no KEXINIT"
expect_class ssh "dial tcp 49.12.20.236:22: connect: connection refused" "connection refused"
expect_class ssh "dial tcp 49.12.20.236:22: i/o timeout" "connect timed out"
expect_class tls "error: notfound.onbex.co did not complete a publicly trusted TLS handshake" "TLS handshake failed"
expect_class tls "error: the fallback certificate expires inside 30 days" "certificate expires"
expect_class tls "error: notfound.onbex.co is trusted but the fallback leaf lacks the exact *.onbex.co SAN" "certificate lacks"
expect_class tls "error: https://notfound.onbex.co/ returned HTTP 200, expected the intentional 404" "unexpected HTTP status"

for class in "certificate expires within 30 days" "TLS handshake failed or the certificate is not publicly trusted"; do
  certificate_failure "$class" || { echo "FAIL: $class should call for a certificate rotation" >&2; fails=$((fails + 1)); }
done
if certificate_failure "HTTP request timed out after the TLS checks passed"; then
  echo "FAIL: an HTTP timeout must not prescribe a certificate rotation" >&2; fails=$((fails + 1))
fi

# A probe that fails once then passes: no outage. One that never passes: down.
counter="$tmp/n"
cat >"$tmp/flaky.sh" <<'PROBE'
#!/usr/bin/env bash
n=$(cat "$COUNTER" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" >"$COUNTER"
[ "$n" -ge 2 ] && exit 0
echo "read server identification: i/o timeout"; exit 1
PROBE
cat >"$tmp/dead.sh" <<'PROBE'
#!/usr/bin/env bash
echo "read server identification: i/o timeout"; exit 1
PROBE
chmod +x "$tmp/flaky.sh" "$tmp/dead.sh"
if ! COUNTER="$counter" EDGE_PROBE_CMD="$tmp/flaky.sh" EDGE_PROBE_PAUSE_SECONDS=0 bash "$here/edge-probe.sh" ssh x "$tmp/flaky.out"; then
  echo "FAIL: a failure the retry cleared reported the edge down" >&2; fails=$((fails + 1))
fi
[ "$(cat "$tmp/flaky.out.class")" = "ok" ] || { echo "FAIL: flaky class $(cat "$tmp/flaky.out.class")" >&2; fails=$((fails + 1)); }
if EDGE_PROBE_CMD="$tmp/dead.sh" EDGE_PROBE_PAUSE_SECONDS=0 bash "$here/edge-probe.sh" ssh x "$tmp/dead.out"; then
  echo "FAIL: a sustained failure reported the edge healthy" >&2; fails=$((fails + 1))
fi
case "$(cat "$tmp/dead.out.class")" in "no SSH banner"*) ;; *) echo "FAIL: dead class $(cat "$tmp/dead.out.class")" >&2; fails=$((fails + 1)) ;; esac
grep -q "failed 3 of 3 attempts" "$tmp/dead.out" || { echo "FAIL: attempts not recorded" >&2; fails=$((fails + 1)); }

if [ "$fails" -ne 0 ]; then
  echo "edge-probe.test: $fails failure(s)" >&2
  exit 1
fi
echo "edge-probe.test: ok"
