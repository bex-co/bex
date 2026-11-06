#!/usr/bin/env bash
# runner-link-probe.test.sh — runner-link-probe.sh against a stubbed curl.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }

# The stub answers by target URL: CURL_<host-word> = "<exit> <http> <bytes/s>".
cat >"$tmp/curl" <<'STUB'
#!/usr/bin/env bash
url="${!#}"
case "$url" in
  *github*) spec="${CURL_GITHUB:-0 200 20000000}" ;;
  *cloudflare*) spec="${CURL_CLOUDFLARE:-0 200 20000000}" ;;
  *mirror*) spec="0 200 0" ;;
esac
read -r rc code speed <<<"$spec"
printf '%s %s 0.05' "$code" "$speed"
exit "$rc"
STUB
chmod +x "$tmp/curl"
probe() { PATH="$tmp:$PATH" bash "$root/scripts/runner-link-probe.sh" >"$tmp/out" 2>&1; }
expect() { # <want-exit> <needle> <label>
  local got=0
  probe || got=$?
  [ "$got" = "$1" ] || fail "$3: exit $got, want $1: $(cat "$tmp/out")"
  grep -qF "$2" "$tmp/out" || fail "$3: output lacks '$2': $(cat "$tmp/out")"
}

expect 0 "github 20.00 MB/s" "a fast link"
CURL_CLOUDFLARE="0 200 500000" expect 1 "cloudflare 0.50 MB/s connect 0.05s slow" "one slow target"
CURL_GITHUB="28 200 65000" expect 1 "github 0.07 MB/s" "a transfer cut by --max-time is still a measurement"
CURL_GITHUB="7 000 0" expect 1 "github unreachable (curl exit 7" "an unreachable target"
CURL_GITHUB="7 000 0" CURL_CLOUDFLARE="6 000 0" expect 2 "no target answered" "nothing answered"
CURL_GITHUB="0 404 9000000" expect 1 "github unreachable (curl exit 0, HTTP 404)" "an HTTP error is not a measurement"
RUNNER_LINK_MIN_MBPS=25 expect 1 "slow" "the threshold is configurable"
RUNNER_LINK_MIN_MBPS=20 expect 0 "ok (>= 20 MB/s)" "exactly the threshold is ok"
DOCKER_REGISTRY_MIRROR=http://registry-mirror-ci:5000 expect 0 "mirror http://registry-mirror-ci:5000 HTTP 200" "the mirror is reported"
RUNNER_LINK_MIN_MBPS=3x expect 2 "must be a number" "a malformed threshold is unusable, not ok"
echo "PASS: runner-link-probe"
