#!/usr/bin/env bash
# Unit tests for ci-fetch-verified.sh's api.github.com fallback. Network-free:
# a stub curl on PATH answers 504 for github.com release URLs and serves the
# asset from the API, the way github.com behaved on 2026-11-08.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/ci-fetch-verified.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir "$work/bin"
printf 'asset bytes\n' >"$work/asset"
want="$(sha256sum "$work/asset" | cut -d' ' -f1)"
cat >"$work/bin/curl" <<EOF
#!/usr/bin/env bash
out="" url=""
while [ \$# -gt 0 ]; do
  case "\$1" in
    -o) out="\$2"; shift ;;
    -H | -C | --connect-timeout | --max-time) shift ;;
    -*) ;;
    *) url="\$1" ;;
  esac
  shift
done
echo "\$url" >>"$work/calls"
case "\$url" in
  https://api.github.com/repos/o/r/releases/tags/v1)
    echo '{"assets":[{"name":"other","id":1},{"name":"tool.tgz","id":42}]}' ;;
  https://api.github.com/repos/o/r/releases/assets/42) cp "$work/asset" "\$out" ;;
  *) echo "curl: (22) The requested URL returned error: 504" >&2; exit 22 ;;
esac
EOF
chmod +x "$work/bin/curl"
# The retry pause is real time the stub sources do not need.
printf '#!/bin/sh\n' >"$work/bin/sleep"
chmod +x "$work/bin/sleep"

failures=0
fail() { echo "FAIL: $1" >&2; failures=$((failures + 1)); }
run() { PATH="$work/bin:$PATH" bash "$script" "$@" >"$work/log" 2>&1; }

rm -f "$work/calls"
if run https://github.com/o/r/releases/download/v1/tool.tgz "$want" "$work/out" 4 \
  && cmp -s "$work/asset" "$work/out"; then
  echo "ok: a github.com 504 falls back to the API asset"
else
  fail "a github.com 504 did not fall back to the API asset"; cat "$work/log" >&2
fi

rm -f "$work/calls" "$work/out"
if run https://github.com/o/r/releases/download/v1/tool.tgz "$(printf '0%.0s' {1..64})" "$work/out" 4; then
  fail "the fallback accepted bytes that do not match the digest"
else
  echo "ok: the fallback still enforces the digest"
fi

rm -f "$work/calls" "$work/out"
if run https://example.com/tool.tgz "$want" "$work/out" 2; then
  fail "a non-GitHub URL verified without its source answering"
elif grep -q api.github.com "$work/calls"; then
  fail "a non-GitHub URL reached the API"
else
  echo "ok: a non-GitHub URL has no fallback"
fi

[ "$failures" -eq 0 ] || exit 1
echo "all ci-fetch-verified.sh tests passed"
