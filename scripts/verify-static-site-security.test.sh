#!/usr/bin/env bash
# Self-test for the PSL gate in scripts/verify-static-site-security.sh, against
# a file:// list (no network). A suffix listed near the start of the PRIVATE
# section, with far more than a pipe buffer of list after it, passes `present`
# and `report`: awk used to stop reading at the match, so curl failed writing
# the rest (exit 23) and pipefail aborted the gate before it printed anything.
# A suffix listed only in the ICANN section, or not at all, fails `present`;
# `absent` agrees; an unreachable list fails closed. node (the browser check),
# go and kubectl are stubs that record their calls.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fails=0

mkdir "$tmp/bin"
for tool in node go kubectl; do
  printf '#!/bin/sh\necho "%s $*" >>"%s/calls"\n' "$tool" "$tmp" >"$tmp/bin/$tool"
  chmod +x "$tmp/bin/$tool"
done

# Shaped like the live list: onbex.co's prepared entry sorts near the start of
# PRIVATE (Besties < onbex.co < BinaryLane) with ~90 KB after it; this leaves
# ~500 KB, past any pipe buffer.
psl="$tmp/public_suffix_list.dat"
{
  echo "// ===BEGIN ICANN DOMAINS==="
  echo "co.uk"
  echo "// ===END ICANN DOMAINS==="
  echo "// ===BEGIN PRIVATE DOMAINS==="
  echo "onbex.co"
  awk 'BEGIN { for (i = 0; i < 20000; i++) printf "padding-%05d.example\n", i }'
  echo "late.example"
  echo "// ===END PRIVATE DOMAINS==="
} >"$psl"

check() { # <label> <want-rc> <want-line> <browser-check: ran|skipped> [VAR=value ...]
  local label="$1" want="$2" line="$3" browser="$4" got out ran=skipped
  shift 4
  : >"$tmp/calls"
  set +e
  out="$(env -u BEX_HOSTING_SUFFIX -u PSL_EXPECTED PATH="$tmp/bin:$PATH" PSL_URL="file://$psl" "$@" \
    bash "$here/verify-static-site-security.sh" repo 2>&1)"
  got=$?
  set -e
  grep -q '^node ' "$tmp/calls" && ran=ran
  if [ "$got" -ne "$want" ] || ! grep -qF -- "$line" <<<"$out" || [ "$ran" != "$browser" ]; then
    echo "FAIL: $label: rc $got (want $want), browser check $ran (want $browser), output:" >&2
    printf '  %s\n' "$out" >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label"
}

check "a suffix listed early in PRIVATE passes present" 0 "PASS  PSL: private section contains onbex.co" ran \
  BEX_HOSTING_SUFFIX=onbex.co
check "present runs the rest of the repo gate" 0 "PASS  repository static-site security checks" ran \
  BEX_HOSTING_SUFFIX=onbex.co PSL_EXPECTED=present
check "a suffix listed last in PRIVATE passes present" 0 "PASS  PSL: private section contains late.example" ran \
  BEX_HOSTING_SUFFIX=late.example
check "report names an early PRIVATE suffix as contained" 0 "INFO  PSL: private section contains onbex.co (diagnostic mode)" ran \
  BEX_HOSTING_SUFFIX=onbex.co PSL_EXPECTED=report
check "absent fails for an early PRIVATE suffix" 1 "FAIL  diagnostic expected onbex.co to be absent" skipped \
  BEX_HOSTING_SUFFIX=onbex.co PSL_EXPECTED=absent
check "an ICANN-only suffix fails present" 1 "FAIL  PSL: private section does not contain co.uk" skipped \
  BEX_HOSTING_SUFFIX=co.uk
check "an unlisted suffix fails present" 1 "FAIL  PSL: private section does not contain unlisted.example" skipped \
  BEX_HOSTING_SUFFIX=unlisted.example
check "an unlisted suffix passes absent" 0 "PASS  diagnostic: private section excludes unlisted.example" ran \
  BEX_HOSTING_SUFFIX=unlisted.example PSL_EXPECTED=absent
check "no candidate suffix skips the PSL and browser checks" 0 "PASS  shared tenant hosting disabled" skipped
# curl exit 37: a file:// list it cannot read. The later PSL_URL wins in env.
check "an unreachable list fails closed" 37 "curl: (37)" skipped \
  BEX_HOSTING_SUFFIX=onbex.co PSL_URL="file://$tmp/missing.dat"

[ "$fails" -eq 0 ] && echo "PASS: verify-static-site-security.sh PSL gate" || { echo "FAIL: $fails case(s)" >&2; exit 1; }
